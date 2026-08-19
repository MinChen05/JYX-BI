package template

import (
	"fmt"
	"strings"

	"github.com/expr-lang/expr"
	"github.com/expr-lang/expr/vm"
)

// Issue 校验错误明细。
type Issue struct {
	RowKey  string `json:"row_key"`
	RowIdx  int    `json:"row_idx"`
	Col     string `json:"col,omitempty"`
	Value   any    `json:"value,omitempty"`
	Message string `json:"message"`
}

// compiledRule 预编译的单元格规则。
type compiledRule struct {
	Cols   []string
	Prog   *vm.Program
	Strict bool // required：空值也报错
	Raw    string
}

// compiledValidator 模板级校验器（加载期编译一次）。
type compiledValidator struct {
	CellRules []compiledRule
	Unique    [][]string // 每组为参与唯一性约束的列
}

// CompileValidation 加载期编译校验规则，模板写错直接报错。
func CompileValidation(spec Spec, cols []ColumnSpec) (*compiledValidator, error) {
	v := &compiledValidator{}
	for _, vd := range spec.Validation {
		if strings.HasPrefix(vd.Rule, "unique(") {
			inner := strings.TrimSuffix(strings.TrimPrefix(vd.Rule, "unique("), ")")
			var keys []string
			for _, k := range strings.Split(inner, ",") {
				keys = append(keys, strings.TrimSpace(k))
			}
			v.Unique = append(v.Unique, keys)
			continue
		}
		if len(vd.Cols) == 0 {
			return nil, fmt.Errorf("rule %q requires cols", vd.Rule)
		}
		colsMatched := MatchCols(vd.Cols, cols)
		if len(colsMatched) == 0 {
			return nil, fmt.Errorf("rule %q matches no columns", vd.Rule)
		}
		strict := vd.Rule == "required"
		var prog *vm.Program
		if !strict {
			// v = 单元格值，其余变量 = 同行其他列值
			p, err := expr.Compile(vd.Rule, expr.AllowUndefinedVariables())
			if err != nil {
				return nil, fmt.Errorf("rule %q: %w", vd.Rule, err)
			}
			prog = p
		}
		v.CellRules = append(v.CellRules, compiledRule{Cols: colsMatched, Prog: prog, Strict: strict, Raw: vd.Rule})
	}
	return v, nil
}

// Run 对整个网格跑全部规则，返回错误清单（空 = 通过）。
func (v *compiledValidator) Run(rows []map[string]any) []Issue {
	var issues []Issue
	for ri, row := range rows {
		for _, cr := range v.CellRules {
			for _, col := range cr.Cols {
				val := row[col]
				if val == nil || AsString(val) == "" {
					if cr.Strict {
						issues = append(issues, Issue{RowIdx: ri, Col: col, Message: "必填"})
					}
					continue
				}
				if cr.Prog == nil {
					continue
				}
				env := make(map[string]any, len(row)+1)
				for k, x := range row {
					env[k] = x
				}
				env["v"] = val
				out, err := expr.Run(cr.Prog, env)
				if err != nil {
					issues = append(issues, Issue{RowIdx: ri, Col: col, Value: val, Message: fmt.Sprintf("规则执行失败: %v", err)})
					continue
				}
				if b, ok := out.(bool); !ok || !b {
					issues = append(issues, Issue{RowIdx: ri, Col: col, Value: val, Message: fmt.Sprintf("不满足规则: %s", cr.Raw)})
				}
			}
		}
	}
	// 表级唯一约束
	for _, keys := range v.Unique {
		seen := map[string]int{}
		for ri, row := range rows {
			var b strings.Builder
			empty := false
			for _, k := range keys {
				s := AsString(row[k])
				if s == "" {
					empty = true
					break
				}
				b.WriteString(s)
				b.WriteByte('|')
			}
			if empty {
				continue
			}
			if first, dup := seen[b.String()]; dup {
				issues = append(issues, Issue{RowIdx: ri, Message: fmt.Sprintf("重复记录（与第 %d 行）: %s", first+1, strings.Join(keys, ", "))})
				continue
			}
			seen[b.String()] = ri
		}
	}
	return issues
}

// AsString 统一转字符串（nil/空 → ""）。
func AsString(v any) string {
	if v == nil {
		return ""
	}
	return strings.TrimSpace(fmt.Sprintf("%v", v))
}
