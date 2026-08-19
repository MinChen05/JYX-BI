package store

import (
	"fmt"
	"regexp"
	"strings"
	"time"

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
// 注意：
//  1. go-mssqldb 只按 @p1,@p2,... 顺序绑定参数，自定义参数名（@k_xxx）不会被绑定；
//  2. 该库（SQL Server 2022）的 MERGE 不接受 WITH (HOLDLOCK) 提示（任意位置均语法错误），
//     单写者低频场景下省略该提示。
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
	pi := 0
	nextP := func() string {
		pi++
		return fmt.Sprintf("@p%d", pi)
	}

	cond := make([]string, 0, len(keys))
	for _, k := range keys {
		cond = append(cond, fmt.Sprintf("t.[%s] = %s", k, nextP()))
		args = append(args, cleanVal(row[k]))
	}
	sb.WriteString("MERGE [")
	sb.WriteString(table)
	sb.WriteString(`] AS t USING (SELECT 1 AS x) AS src ON 1=1 AND `)
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
			setParts = append(setParts, fmt.Sprintf("t.[%s] = %s", c, nextP()))
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
		valList = append(valList, nextP())
		args = append(args, cleanVal(row[c]))
	}
	sb.WriteString(strings.Join(colList, ", "))
	sb.WriteString(") VALUES (")
	sb.WriteString(strings.Join(valList, ", "))
	sb.WriteString(");")

	_, err = sd.Exec(sb.String(), args...)
	return err
}

// SelectKeyRows 查询 scope 范围内已存在的键值组合（计算删除差集用）。
// scopeSQL 形如 "t1 = @p1 AND t2 = @p2"（可为空串 = 全表），scopeArgs 与占位符按序对应。
func SelectKeyRows(db *gorm.DB, table string, keys []string, scopeSQL string, scopeArgs []any) ([][]any, error) {
	if err := checkIdent(table); err != nil {
		return nil, err
	}
	for _, k := range keys {
		if err := checkIdent(k); err != nil {
			return nil, err
		}
	}
	sd, err := db.DB()
	if err != nil {
		return nil, err
	}
	cols := make([]string, 0, len(keys))
	for _, k := range keys {
		cols = append(cols, "["+k+"]")
	}
	q := "SELECT " + strings.Join(cols, ", ") + " FROM [" + table + "]"
	if scopeSQL != "" {
		q += " WHERE " + scopeSQL
	}
	rows, err := sd.Query(q, scopeArgs...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out [][]any
	for rows.Next() {
		vals := make([]any, len(keys))
		ptrs := make([]any, len(keys))
		for i := range vals {
			ptrs[i] = &vals[i]
		}
		if err := rows.Scan(ptrs...); err != nil {
			return nil, err
		}
		out = append(out, vals)
	}
	return out, nil
}

// DeleteByKeys 删除 scope 范围内键值组合命中 tuples 的行，返回删除行数。
// tuples 每行 = 一个键值组合（与 keys 顺序一致）。按 500 行/语句分批，避免参数超限。
func DeleteByKeys(db *gorm.DB, table string, keys []string, scopeSQL string, scopeArgs []any, tuples [][]any) (int64, error) {
	if err := checkIdent(table); err != nil {
		return 0, err
	}
	for _, k := range keys {
		if err := checkIdent(k); err != nil {
			return 0, err
		}
	}
	if len(tuples) == 0 {
		return 0, nil
	}
	sd, err := db.DB()
	if err != nil {
		return 0, err
	}
	var total int64
	for start := 0; start < len(tuples); start += 500 {
		end := start + 500
		if end > len(tuples) {
			end = len(tuples)
		}
		var sb strings.Builder
		var args []any
		pi := 0
		nextP := func() string {
			pi++
			return fmt.Sprintf("@p%d", pi)
		}
		if scopeSQL != "" {
			// scope 参数在前
			sb.WriteString(" WHERE ")
			sb.WriteString(scopeSQL)
			args = append(args, scopeArgs...)
			pi = len(scopeArgs)
			sb.WriteString(" AND ")
		} else {
			sb.WriteString(" WHERE ")
		}
		conds := make([]string, 0, end-start)
		for _, t := range tuples[start:end] {
			kc := make([]string, 0, len(keys))
			for i, k := range keys {
				kc = append(kc, fmt.Sprintf("[%s] = %s", k, nextP()))
				args = append(args, t[i])
			}
			conds = append(conds, "("+strings.Join(kc, " AND ")+")")
		}
		sb.WriteString(strings.Join(conds, " OR "))
		res, err := sd.Exec("DELETE FROM ["+table+"]"+sb.String(), args...)
		if err != nil {
			return total, err
		}
		if n, err := res.RowsAffected(); err == nil {
			total += n
		}
	}
	return total, nil
}

// KeyTupleEq 比较两个键值组合是否相等（nil/空串 视为相同，
// decimal 的 []byte 与字符串按内容比较）。
func KeyTupleEq(a, b []any) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if fmt.Sprintf("%v", normKeyVal(a[i])) != fmt.Sprintf("%v", normKeyVal(b[i])) {
			return false
		}
	}
	return true
}

func normKeyVal(v any) any {
	switch x := v.(type) {
	case nil:
		return ""
	case []byte:
		return strings.TrimSpace(string(x))
	case string:
		return strings.TrimSpace(x)
	case time.Time:
		return x.Format("2006-01-02")
	default:
		return v
	}
}
