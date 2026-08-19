package store

import (
	"fmt"
	"regexp"
	"strings"

	"gorm.io/gorm"
)

var identRe = regexp.MustCompile(`^[\w]+$`)

// UpsertRows Doris 批量 upsert：INSERT ... ON DUPLICATE KEY UPDATE（全列）。
// table/cols 来自内部模板配置（非用户输入），列名仍做标识符白名单校验。
func UpsertRows(db *gorm.DB, table string, cols []string, rows [][]any) error {
	if db == nil {
		return fmt.Errorf("doris 未配置，无法写入 %s", table)
	}
	if len(rows) == 0 {
		return nil
	}
	if !identRe.MatchString(table) {
		return fmt.Errorf("非法表名 %q", table)
	}
	for _, c := range cols {
		if !identRe.MatchString(c) {
			return fmt.Errorf("非法列名 %q", c)
		}
	}
	sd, err := db.DB()
	if err != nil {
		return err
	}
	var sb strings.Builder
	sb.WriteString("INSERT INTO `")
	sb.WriteString(table)
	sb.WriteString("` (")
	for i, c := range cols {
		if i > 0 {
			sb.WriteString(",")
		}
		sb.WriteString("`")
		sb.WriteString(c)
		sb.WriteString("`")
	}
	sb.WriteString(") VALUES ")
	args := make([]any, 0, len(rows)*len(cols))
	for ri, row := range rows {
		if len(row) != len(cols) {
			return fmt.Errorf("第 %d 行列数 %d 与列定义 %d 不符", ri+1, len(row), len(cols))
		}
		if ri > 0 {
			sb.WriteString(",")
		}
		sb.WriteString("(")
		for ci := range cols {
			if ci > 0 {
				sb.WriteString(",")
			}
			sb.WriteString("?")
			v := row[ci]
			if s, ok := v.(string); ok {
				s = strings.TrimSpace(s)
				if s == "" {
					v = nil
				} else {
					v = s
				}
			}
			args = append(args, v)
		}
		sb.WriteString(")")
	}
	sb.WriteString(" ON DUPLICATE KEY UPDATE ")
	for i, c := range cols {
		if i > 0 {
			sb.WriteString(",")
		}
		sb.WriteString("`")
		sb.WriteString(c)
		sb.WriteString("`=VALUES(`")
		sb.WriteString(c)
		sb.WriteString("`)")
	}
	_, err = sd.Exec(sb.String(), args...)
	return err
}
