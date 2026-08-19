package engine

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"

	"github.com/MinChen05/JYX-BI/internal/template"
	"gorm.io/gorm"
)

var (
	monthRe = regexp.MustCompile(`^\d{4}-(0[1-9]|1[0-2])$`)
	dateRe  = regexp.MustCompile(`^\d{4}-\d{2}-\d{2}$`)
	textRe  = regexp.MustCompile(`^[\w\-]{1,64}$`)
	tokenRe = regexp.MustCompile(`\{(\w+)\}`)
)

// ValidateParams 校验参数格式（同时也是 SQL token 替换前的注入防线：
// 只允许通过白名单格式校验的参数进入查询）。
func ValidateParams(def *template.ReportDef, params map[string]string) error {
	for _, p := range def.Spec.Params {
		v, ok := params[p.Key]
		if !ok || v == "" {
			if p.Required {
				return fmt.Errorf("参数 %s 必填", p.Key)
			}
			continue
		}
		switch p.Type {
		case "month":
			if !monthRe.MatchString(v) {
				return fmt.Errorf("参数 %s 必须为 YYYY-MM", p.Key)
			}
		case "date":
			if !dateRe.MatchString(v) {
				return fmt.Errorf("参数 %s 必须为 YYYY-MM-DD", p.Key)
			}
		case "text":
			if !textRe.MatchString(v) {
				return fmt.Errorf("参数 %s 只能包含字母数字下划线短横线", p.Key)
			}
		}
	}
	return nil
}

// FetchBaseRows 获取行集（主数据预填行）。
func FetchBaseRows(doris *gorm.DB, def *template.ReportDef, params map[string]string) ([]map[string]any, error) {
	if def.Spec.Rows.Source == "static" {
		out := make([]map[string]any, 0, len(def.Spec.Rows.StaticRows))
		for _, r := range def.Spec.Rows.StaticRows {
			c := make(map[string]any, len(r))
			for k, v := range r {
				c[k] = v
			}
			out = append(out, c)
		}
		return out, nil
	}
	if doris == nil {
		return nil, fmt.Errorf("数据源未配置，无法执行行集查询")
	}
	q := tokenRe.ReplaceAllStringFunc(def.Spec.Rows.Query, func(m string) string {
		key := tokenRe.FindStringSubmatch(m)[1]
		if v, ok := params[key]; ok {
			return v // 已通过 ValidateParams 白名单校验
		}
		return m
	})
	var rows []map[string]any
	if err := doris.Raw(q).Scan(&rows).Error; err != nil {
		return nil, fmt.Errorf("行集查询失败: %w", err)
	}
	for _, r := range rows {
		for k, v := range r {
			r[k] = normalizeCell(v)
		}
	}
	return rows, nil
}

// normalizeCell 驱动返回值统一转成 JSON 友好类型：
// []byte（decimal 等）→ 数值/字符串；string → 去首尾空白（库里存在单空格脏数据），空 → nil。
func normalizeCell(v any) any {
	switch x := v.(type) {
	case []byte:
		s := strings.TrimSpace(string(x))
		if s == "" {
			return nil
		}
		if f, err := strconv.ParseFloat(s, 64); err == nil {
			return f
		}
		return s
	case string:
		s := strings.TrimSpace(x)
		if s == "" {
			return nil
		}
		return s
	default:
		return v
	}
}

// NormalizeCell 驱动返回值统一转 JSON 友好类型（normalizeCell 的导出包装，供 SQL 预览等复用）。
func NormalizeCell(v any) any { return normalizeCell(v) }

// ReplaceTokens 通用 {param} 替换（供其他模块复用）。
func ReplaceTokens(s string, params map[string]string) string {
	return tokenRe.ReplaceAllStringFunc(s, func(m string) string {
		key := tokenRe.FindStringSubmatch(m)[1]
		if v, ok := params[key]; ok {
			return v
		}
		return m
	})
}

// NormalizeForDoris 值归一化：月份 "2026-01" → 月初日期 "2026-01-01"（写入 DATE 列）。
func NormalizeForDoris(v any) any {
	if s, ok := v.(string); ok {
		s = strings.TrimSpace(s)
		if monthRe.MatchString(s) {
			return s + "-01"
		}
		if s == "" {
			return nil
		}
	}
	return v
}
