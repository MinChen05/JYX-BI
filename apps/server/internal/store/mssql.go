package store

import (
	"fmt"
	"regexp"
	"strings"

	"gorm.io/gorm"
)

var mssqlIdentRe = regexp.MustCompile(`^[\w]+$`)

func checkIdent(name string) error {
	if !mssqlIdentRe.MatchString(name) {
		return fmt.Errorf("非法标识符 %q", name)
	}
	return nil
}

func cleanVal(v any) any {
	if s, ok := v.(string); ok {
		s = strings.TrimSpace(s)
		if s == "" {
			return nil
		}
		return s
	}
	return v
}

// UpsertRowMssql 单行 MERGE（按 keys 匹配，命中则更新非 key 列，未命中则整行插入）。
// keys/cols 为列名（表内），row 为 列名→值 映射。
func UpsertRowMssql(db *gorm.DB, table string, keys []string, cols []string, row map[string]any) error {
	if db == nil {
		return fmt.Errorf("mssql 未配置")
	}
	if err := checkIdent(table); err != nil {
		return err
	}
	for _, c := range append(append([]string{}, cols...), keys...) {
		if err := checkIdent(c); err != nil {
			return err
		}
	}
	sd, err := db.DB()
	if err != nil {
		return err
	}
	keySet := map[string]bool{}
	for _, k := range keys {
		keySet[k] = true
	}

	var sb strings.Builder
	var args []any
	cond := make([]string, 0, len(keys))
	for _, k := range keys {
		cond = append(cond, fmt.Sprintf("t.[%s] = @k_%s", k, k))
		args = append(args, cleanVal(row[k]))
	}
	sb.WriteString("MERGE [")
	sb.WriteString(table)
	sb.WriteString(`] AS t WITH (HOLDLOCK) USING (SELECT 1) AS src ON 1=1 AND `)
	sb.WriteString(strings.Join(cond, " AND "))

	var updCols []string
	for _, c := range cols {
		if !keySet[c] {
			updCols = append(updCols, c)
		}
	}
	if len(updCols) > 0 {
		setParts := make([]string, 0, len(updCols))
		for _, c := range updCols {
			setParts = append(setParts, fmt.Sprintf("t.[%s] = @u_%s", c, c))
			args = append(args, cleanVal(row[c]))
		}
		sb.WriteString(" WHEN MATCHED THEN UPDATE SET ")
		sb.WriteString(strings.Join(setParts, ", "))
	}
	sb.WriteString(" WHEN NOT MATCHED THEN INSERT (")
	colList := make([]string, 0, len(cols))
	valList := make([]string, 0, len(cols))
	for _, c := range cols {
		colList = append(colList, "["+c+"]")
		valList = append(valList, "@i_"+c)
		args = append(args, cleanVal(row[c]))
	}
	sb.WriteString(strings.Join(colList, ", "))
	sb.WriteString(") VALUES (")
	sb.WriteString(strings.Join(valList, ", "))
	sb.WriteString(");")

	_, err = sd.Exec(sb.String(), args...)
	return err
}
