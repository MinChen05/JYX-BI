package template

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"time"
)

// ColumnSpec 展开后的列定义（前端渲染与导入导出共用的契约）。
type ColumnSpec struct {
	Key      string `json:"key"`
	Label    string `json:"label"`
	Type     string `json:"type"`
	Readonly bool   `json:"readonly"`
	Formula  string `json:"formula,omitempty"`
	Width    int    `json:"width,omitempty"`
}

var dayExprRe = regexp.MustCompile(`^days\(param\.(\w+)\)$`)

// ExpandColumns 将模板列定义展开为最终列列表。
// 动态列 v1 仅支持 days(param.<monthKey>)：按月份天数生成 d01..dNN。
func ExpandColumns(def *ReportDef, params map[string]string) ([]ColumnSpec, error) {
	out := make([]ColumnSpec, 0, len(def.Spec.Columns))
	for _, c := range def.Spec.Columns {
		if c.Dynamic == nil {
			out = append(out, ColumnSpec{
				Key: c.Key, Label: c.Label, Type: c.Type,
				Readonly: c.Readonly, Formula: c.Formula, Width: c.Width,
			})
			continue
		}
		m := dayExprRe.FindStringSubmatch(c.Dynamic.Expr)
		if m == nil {
			return nil, fmt.Errorf("column %s: unsupported dynamic expr %q", c.Key, c.Dynamic.Expr)
		}
		month, ok := params[m[1]]
		if !ok || month == "" {
			return nil, fmt.Errorf("column %s: param %s missing", c.Key, m[1])
		}
		days, err := daysInMonth(month)
		if err != nil {
			return nil, fmt.Errorf("column %s: %w", c.Key, err)
		}
		for day := 1; day <= days; day++ {
			out = append(out, ColumnSpec{
				Key:      renderTpl(c.Dynamic.Key, day, params),
				Label:    renderTpl(c.Dynamic.Label, day, params),
				Type:     c.Type,
				Readonly: c.Readonly,
				Width:    c.Width,
			})
		}
	}
	return out, nil
}

func daysInMonth(month string) (int, error) {
	t, err := time.Parse("2006-01", month)
	if err != nil {
		return 0, fmt.Errorf("param must be YYYY-MM, got %q", month)
	}
	// 下月 1 号减 1 天 = 当月最后一天
	return t.AddDate(0, 1, 0).AddDate(0, 0, -1).Day(), nil
}

// renderTpl 渲染 {day} / {day:02} / {paramKey} / {paramKey:MM}。
func renderTpl(tpl string, day int, params map[string]string) string {
	repl := func(g string) string {
		// ReplaceAllStringFunc 传入完整匹配（含花括号），先剥掉
		g = g[1 : len(g)-1]
		name, format := g, ""
		if i := strings.IndexByte(g, ':'); i >= 0 {
			name, format = g[:i], g[i+1:]
		}
		if name == "day" {
			if format == "02" {
				return fmt.Sprintf("%02d", day)
			}
			return strconv.Itoa(day)
		}
		if v, ok := params[name]; ok {
			if format == "MM" && strings.HasPrefix(v, "20") && len(v) >= 7 {
				return v[5:7]
			}
			return v
		}
		return "{" + g + "}"
	}
	return regexp.MustCompile(`\{(\w+(?::\w+)?)\}`).ReplaceAllStringFunc(tpl, repl)
}

// ExpandRange 把 "m01..m12" 展开为存在的列 key 列表。
// cols 为全部列 key（含动态展开后的）。
func ExpandRange(pattern string, cols []ColumnSpec) []string {
	if !strings.Contains(pattern, "..") {
		return []string{pattern}
	}
	parts := strings.SplitN(pattern, "..", 2)
	start, end := parts[0], parts[1]
	re := regexp.MustCompile(`^(.*?)(\d+)$`)
	ms, me := re.FindStringSubmatch(start), re.FindStringSubmatch(end)
	if ms == nil || me == nil {
		return []string{pattern}
	}
	s, _ := strconv.Atoi(ms[2])
	e, _ := strconv.Atoi(me[2])
	width := len(ms[2])
	byKey := map[string]bool{}
	for _, c := range cols {
		byKey[c.Key] = true
	}
	var out []string
	for i := s; i <= e; i++ {
		k := fmt.Sprintf("%s%0*d", ms[1], width, i)
		if byKey[k] {
			out = append(out, k)
		}
	}
	return out
}

// MatchCols 解析校验/导入的列选择：支持精确 key、"a..b" 范围、"prefix.*" 通配。
func MatchCols(patterns []string, cols []ColumnSpec) []string {
	var out []string
	seen := map[string]bool{}
	for _, p := range patterns {
		if strings.Contains(p, "..") {
			for _, k := range ExpandRange(p, cols) {
				if !seen[k] {
					seen[k] = true
					out = append(out, k)
				}
			}
			continue
		}
		if strings.HasSuffix(p, ".*") {
			prefix := strings.TrimSuffix(p, ".*")
			for _, c := range cols {
				if strings.HasPrefix(c.Key, prefix) && !seen[c.Key] {
					seen[c.Key] = true
					out = append(out, c.Key)
				}
			}
			continue
		}
		if !seen[p] {
			seen[p] = true
			out = append(out, p)
		}
	}
	return out
}
