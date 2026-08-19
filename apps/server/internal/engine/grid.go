package engine

import (
	"fmt"
	"strings"
	"time"

	"github.com/MinChen05/JYX-BI/internal/template"
)

// 前端/导入共用的网格契约。

type GridSpec struct {
	Report       string                `json:"report"`
	Name         string                `json:"name"`
	Version      int                   `json:"version"`
	Params       map[string]string     `json:"params"`
	Instance     *InstanceInfo         `json:"instance"`
	Columns      []template.ColumnSpec `json:"columns"`
	Rows         []RowSpec             `json:"rows"`
	RowOps       RowOps                `json:"row_ops"`
	NumberFormat string                `json:"number_format"`
	Editable     bool                  `json:"editable"` // false = 纯展示报表
}

type InstanceInfo struct {
	ID        int64     `json:"id"`
	Status    int8      `json:"status"`
	UpdatedAt time.Time `json:"updated_at"`
}

type RowSpec struct {
	RowKey string         `json:"row_key"`
	Cells  map[string]any `json:"cells"`
}

type RowOps struct {
	Add    bool `json:"add"`
	Delete bool `json:"delete"`
}

// MergedRow 带 rowKey 的完整行（含公式列），供校验使用。
type MergedRow struct {
	RowKey string
	Cells  map[string]any
}

// RowKeyOf 由匹配列生成规范 rowKey；匹配列不全时退化为 r{idx:04d}（ok=false）。
func RowKeyOf(def *template.ReportDef, row map[string]any, idx int) (string, bool) {
	keys := def.MatchKeys()
	if len(keys) > 0 {
		var b strings.Builder
		all := true
		for i, k := range keys {
			if i > 0 {
				b.WriteByte('|')
			}
			s := template.AsString(row[k])
			if s == "" {
				all = false
				break
			}
			b.WriteString(s)
		}
		if all {
			return b.String(), true
		}
	}
	return fmt.Sprintf("r%04d", idx), false
}

// BuildRows base 行集 + 草稿值 → 完整行（含公式列、序号列）。
func BuildRows(c *template.Compiled, base []map[string]any, draft map[string]map[string]any) []MergedRow {
	rows := make([]MergedRow, 0, len(base))
	for i, b := range base {
		rowKey, _ := RowKeyOf(c.Def, b, i)
		rows = append(rows, mergeOne(c, b, rowKey, draft, i))
	}
	// 草稿里新增的行（base 之外的 rowKey）
	baseKeys := map[string]bool{}
	for _, r := range rows {
		baseKeys[r.RowKey] = true
	}
	for rk, cells := range draft {
		if baseKeys[rk] {
			continue
		}
		full := map[string]any{}
		for k, v := range cells {
			full[k] = v
		}
		rows = append(rows, finalize(c, full, rk))
	}
	return rows
}

func mergeOne(c *template.Compiled, base map[string]any, rowKey string, draft map[string]map[string]any, idx int) MergedRow {
	full := map[string]any{}
	for _, col := range c.Cols {
		if col.Type == "auto" {
			continue
		}
		if v, ok := base[col.Key]; ok {
			full[col.Key] = v
		}
	}
	if dv, ok := draft[rowKey]; ok {
		for _, col := range c.Cols {
			if col.Readonly || col.Type == "auto" {
				continue
			}
			if v, ok := dv[col.Key]; ok {
				full[col.Key] = v
			}
		}
	}
	return finalize(c, full, rowKey)
}

func finalize(c *template.Compiled, full map[string]any, rowKey string) MergedRow {
	for key, p := range c.Formulas {
		if v, err := template.EvalFormula(p, full); err == nil {
			full[key] = v
		}
	}
	if full["seq"] == nil {
		// 序号由渲染层按位置生成，这里不填
	}
	return MergedRow{RowKey: rowKey, Cells: full}
}

// ToMaps MergedRow 行 → 纯 map 切片（校验引擎用）。
func ToMaps(rows []MergedRow) []map[string]any {
	out := make([]map[string]any, len(rows))
	for i, r := range rows {
		out[i] = r.Cells
	}
	return out
}

// ToGridRows MergedRow → 前端 RowSpec（seq 列按位置填充）。
func ToGridRows(c *template.Compiled, rows []MergedRow) []RowSpec {
	out := make([]RowSpec, 0, len(rows))
	for i, r := range rows {
		cells := make(map[string]any, len(r.Cells))
		for k, v := range r.Cells {
			cells[k] = v
		}
		for _, col := range c.Cols {
			if col.Type == "auto" {
				cells[col.Key] = i + 1
			}
		}
		out = append(out, RowSpec{RowKey: r.RowKey, Cells: cells})
	}
	return out
}

// NormalizePayload 客户端提交的行 → 可存储的草稿（仅保留可编辑列，规范 rowKey）。
func NormalizePayload(c *template.Compiled, payload []RowPayload) (map[string]map[string]any, error) {
	out := map[string]map[string]any{}
	for i, p := range payload {
		if len(p.Cells) == 0 {
			continue
		}
		row := map[string]any{}
		for _, col := range c.Cols {
			if col.Readonly || col.Type == "auto" {
				continue
			}
			if v, ok := p.Cells[col.Key]; ok {
				row[col.Key] = v
			}
		}
		// 公式列一律丢弃（服务端重算），readonly 列已跳过
		rk := p.RowKey
		if canon, ok := RowKeyOf(c.Def, p.Cells, 0); ok {
			rk = canon
		}
		if rk == "" {
			rk = fmt.Sprintf("n%04d", i)
		}
		if _, exists := out[rk]; exists {
			return nil, fmt.Errorf("提交数据中 row_key 重复: %s", rk)
		}
		out[rk] = row
	}
	return out, nil
}

// RowPayload 客户端提交的行数据。
type RowPayload struct {
	RowKey string         `json:"row_key"`
	Cells  map[string]any `json:"cells"`
}

// DraftRequest 存草稿/提交的请求体。
type DraftRequest struct {
	ExpectedUpdatedAt *time.Time  `json:"expected_updated_at"`
	Rows              []RowPayload `json:"rows"`
}
