package template

import (
	"crypto/sha256"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"gopkg.in/yaml.v3"
)

// LoadDir 加载目录下所有 .yaml 模板并做结构校验。
func LoadDir(dir string) (map[string]*ReportDef, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, fmt.Errorf("read template dir %s: %w", dir, err)
	}
	out := make(map[string]*ReportDef)
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".yaml") {
			continue
		}
		raw, err := os.ReadFile(filepath.Join(dir, e.Name()))
		if err != nil {
			return nil, err
		}
		def, err := Parse(raw)
		if err != nil {
			return nil, fmt.Errorf("template %s: %w", e.Name(), err)
		}
		if _, exists := out[def.Metadata.Code]; exists {
			return nil, fmt.Errorf("duplicate report code %s", def.Metadata.Code)
		}
		out[def.Metadata.Code] = def
	}
	return out, nil
}

// Parse 解析并校验单个模板。
func Parse(raw []byte) (*ReportDef, error) {
	var def ReportDef
	if err := yaml.Unmarshal(raw, &def); err != nil {
		return nil, err
	}
	if def.APIVersion != "rpt/v1" {
		return nil, fmt.Errorf("apiVersion must be rpt/v1, got %q", def.APIVersion)
	}
	if def.Metadata.Code == "" {
		return nil, fmt.Errorf("metadata.code required")
	}
	if len(def.Spec.Columns) == 0 {
		return nil, fmt.Errorf("at least one column required")
	}
	keys := map[string]bool{}
	for _, c := range def.Spec.Columns {
		if c.Key == "" {
			return nil, fmt.Errorf("column with empty key")
		}
		if c.Dynamic != nil {
			if !strings.Contains(c.Dynamic.Key, "{day") {
				return nil, fmt.Errorf("dynamic column key must contain {day}")
			}
			if !strings.Contains(c.Dynamic.Expr, "days(param.") {
				return nil, fmt.Errorf("dynamic expr v1 only supports days(param.<key>), got %q", c.Dynamic.Expr)
			}
		} else {
			if keys[c.Key] {
				return nil, fmt.Errorf("duplicate column key %s", c.Key)
			}
			keys[c.Key] = true
		}
		if c.Formula != "" && c.Readonly == false {
			return nil, fmt.Errorf("formula column %s must be readonly", c.Key)
		}
	}
	if def.Spec.Rows.Source == "sql" && def.Spec.Rows.Query == "" {
		return nil, fmt.Errorf("rows.source=sql requires rows.query")
	}
	if def.Spec.Rows.Source != "sql" && def.Spec.Rows.Source != "static" {
		return nil, fmt.Errorf("rows.source must be sql|static")
	}
	for _, p := range def.Spec.Params {
		if p.Type != "month" && p.Type != "date" && p.Type != "text" {
			return nil, fmt.Errorf("param %s: type must be month|date|text", p.Key)
		}
	}
	def.Raw = raw
	return &def, nil
}

// Checksum 模板内容指纹（用于版本落库）。
func (d *ReportDef) Checksum(raw []byte) string {
	sum := sha256.Sum256(raw)
	return fmt.Sprintf("%x", sum)
}

// ParamsHash (模板+参数) 的稳定哈希：key 排序后拼接。
func ParamsHash(params map[string]string) string {
	keys := make([]string, 0, len(params))
	for k := range params {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	var b strings.Builder
	for _, k := range keys {
		b.WriteString(k)
		b.WriteByte('=')
		b.WriteString(params[k])
		b.WriteByte(';')
	}
	sum := sha256.Sum256([]byte(b.String()))
	return fmt.Sprintf("%x", sum)
}

// ColumnKeySet 静态列 key 集合（不含动态展开）。
func (d *ReportDef) ColumnKeySet() map[string]bool {
	out := map[string]bool{}
	for _, c := range d.Spec.Columns {
		if c.Dynamic == nil {
			out[c.Key] = true
		}
	}
	return out
}

// MatchKeys 导入匹配列：显式配置优先，否则取 readonly 前 N 列。
func (d *ReportDef) MatchKeys() []string {
	if len(d.Spec.Import.MatchKeys) > 0 {
		return d.Spec.Import.MatchKeys
	}
	var out []string
	for _, c := range d.Spec.Columns {
		if c.Readonly {
			out = append(out, c.Key)
		}
		if len(out) >= 2 {
			break
		}
	}
	return out
}
