package template

import (
	"fmt"
	"regexp"
	"sort"
	"strings"
	"sync"

	"github.com/expr-lang/expr/vm"
)

// DorisTarget 提交写回的列映射（编译期展开，含动态列与参数列）。
type DorisTarget struct {
	DstCol string
	SrcCol string // 网格列 key；空则取 Param
	Param  string // 参数名；Now=true 时填时间
	Now    bool
}

// Compiled 模板 + 参数组合的编译产物（列展开/公式/校验/写回映射）。
type Compiled struct {
	Def       *ReportDef
	Cols      []ColumnSpec
	Formulas  map[string]*vm.Program
	Validator *compiledValidator
	Doris     []DorisTarget
}

// Engine 模板注册表 + 编译缓存。
type Engine struct {
	defs  map[string]*ReportDef
	mu    sync.RWMutex
	cache sync.Map
}

func NewEngine(dir string) (*Engine, error) {
	defs, err := LoadDir(dir)
	if err != nil {
		return nil, err
	}
	return &Engine{defs: defs}, nil
}

// Reload 重新加载模板目录（设计器保存 / 手工改 YAML 后热生效）。
// 加载失败时保留旧模板。
func (e *Engine) Reload(dir string) error {
	defs, err := LoadDir(dir)
	if err != nil {
		return err
	}
	e.mu.Lock()
	e.defs = defs
	e.mu.Unlock()
	e.cache = sync.Map{}
	return nil
}

func (e *Engine) Codes() []string {
	e.mu.RLock()
	defer e.mu.RUnlock()
	codes := make([]string, 0, len(e.defs))
	for c := range e.defs {
		codes = append(codes, c)
	}
	sort.Strings(codes)
	return codes
}

func (e *Engine) Get(code string) (*ReportDef, bool) {
	e.mu.RLock()
	defer e.mu.RUnlock()
	d, ok := e.defs[code]
	return d, ok
}

func (e *Engine) Compile(code string, params map[string]string) (*Compiled, error) {
	key := code + "|" + ParamsHash(params)
	if c, ok := e.cache.Load(key); ok {
		return c.(*Compiled), nil
	}
	e.mu.RLock()
	def, ok := e.defs[code]
	e.mu.RUnlock()
	if !ok {
		return nil, fmt.Errorf("report %s not found", code)
	}
	cols, err := ExpandColumns(def, params)
	if err != nil {
		return nil, err
	}
	validator, err := CompileValidation(def.Spec, cols)
	if err != nil {
		return nil, fmt.Errorf("report %s: %w", code, err)
	}
	formulas := map[string]*vm.Program{}
	for _, dc := range def.Spec.Columns {
		if dc.Formula == "" || dc.Dynamic != nil {
			continue
		}
		p, err := CompileFormula(dc.Formula, cols)
		if err != nil {
			return nil, fmt.Errorf("report %s column %s formula: %w", code, dc.Key, err)
		}
		formulas[dc.Key] = p
	}
	c := &Compiled{Def: def, Cols: cols, Formulas: formulas, Validator: validator}
	if def.Spec.Submit.Doris.Table != "" {
		targets, err := expandDorisMapping(def, cols)
		if err != nil {
			return nil, fmt.Errorf("report %s doris mapping: %w", code, err)
		}
		c.Doris = targets
	}
	if def.Spec.Submit.Mssql.Table != "" {
		if err := validateMssqlDelete(def.Spec.Submit.Mssql, def.Spec.Submit.Mode); err != nil {
			return nil, fmt.Errorf("report %s mssql delete: %w", code, err)
		}
	}
	e.cache.Store(key, c)
	return c, nil
}

var tplIdentRe = regexp.MustCompile(`^[A-Za-z0-9_]+$`)

// validateMssqlDelete 校验写回删除语义配置的静态部分（值合法、列名合法、month 依赖 unpivot）。
func validateMssqlDelete(mw MssqlWrite, mode string) error {
	switch mw.Delete {
	case "", "all":
	case "month":
		if mode != "unpivot" || mw.Pivot == nil {
			return fmt.Errorf("delete:month 仅支持 unpivot 模式")
		}
		if !tplIdentRe.MatchString(mw.Pivot.DateCol) {
			return fmt.Errorf("非法日期列名 %q", mw.Pivot.DateCol)
		}
	default:
		return fmt.Errorf("非法值 %q（可选 all|month）", mw.Delete)
	}
	for col := range mw.DeleteWhere {
		if !tplIdentRe.MatchString(col) {
			return fmt.Errorf("非法列名 %q", col)
		}
	}
	return nil
}

var numSuffixRe = regexp.MustCompile(`^(.*?)(\d+)$`)

func dayMapped(srcPattern, colKey string) bool {
	if !strings.Contains(srcPattern, "{day}") {
		return false
	}
	m := numSuffixRe.FindStringSubmatch(colKey)
	if m == nil {
		return false
	}
	return strings.Replace(srcPattern, "{day}", m[2], 1) == colKey
}

func expandDorisMapping(def *ReportDef, cols []ColumnSpec) ([]DorisTarget, error) {
	mapping := def.Spec.Submit.Doris.Mapping
	seenDst := map[string]bool{}
	var out []DorisTarget
	addTarget := func(t DorisTarget) error {
		if seenDst[t.DstCol] {
			return fmt.Errorf("duplicate target column %s", t.DstCol)
		}
		seenDst[t.DstCol] = true
		out = append(out, t)
		return nil
	}
	keys := make([]string, 0, len(mapping))
	for k := range mapping {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, src := range keys {
		dst := mapping[src]
		switch {
		case strings.HasPrefix(src, "param."):
			if err := addTarget(DorisTarget{DstCol: dst, Param: strings.TrimPrefix(src, "param.")}); err != nil {
				return nil, err
			}
		case strings.Contains(src, "{day}"):
			matched := false
			for _, c := range cols {
				if dayMapped(src, c.Key) {
					matched = true
					dstCol := strings.Replace(dst, "{day}", numSuffixRe.FindStringSubmatch(c.Key)[2], 1)
					if err := addTarget(DorisTarget{DstCol: dstCol, SrcCol: c.Key}); err != nil {
						return nil, err
					}
				}
			}
			if !matched {
				return nil, fmt.Errorf("dynamic mapping %q matches no columns", src)
			}
		default:
			found := false
			for _, c := range cols {
				if c.Key == src {
					found = true
					break
				}
			}
			if !found {
				return nil, fmt.Errorf("mapping source %q is not a column", src)
			}
			if err := addTarget(DorisTarget{DstCol: dst, SrcCol: src}); err != nil {
				return nil, err
			}
		}
	}
	if err := addTarget(DorisTarget{DstCol: "updated_at", Now: true}); err != nil {
		return nil, err
	}
	return out, nil
}
