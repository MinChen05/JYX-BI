package main

import (
	"database/sql"
	"fmt"
	"os"
	"strings"

	_ "github.com/go-sql-driver/mysql"
	_ "github.com/microsoft/go-mssqldb"
)

func main() {
	dsn := os.Args[1]
	q := os.Args[2]
	var db *sql.DB
	var err error
	if strings.HasPrefix(dsn, "sqlserver") {
		db, err = sql.Open("sqlserver", dsn)
	} else {
		db, err = sql.Open("mysql", dsn)
	}
	if err != nil {
		fmt.Println("open:", err)
		os.Exit(1)
	}
	defer db.Close()
	if err := db.Ping(); err != nil {
		fmt.Println("ping:", err)
		os.Exit(1)
	}
	rows, err := db.Query(q)
	if err != nil {
		fmt.Println("query:", err)
		os.Exit(1)
	}
	defer rows.Close()
	cols, _ := rows.Columns()
	for rows.Next() {
		vals := make([]any, len(cols))
		ptrs := make([]any, len(cols))
		for i := range vals {
			ptrs[i] = &vals[i]
		}
		if err := rows.Scan(ptrs...); err != nil {
			fmt.Println("scan:", err)
			return
		}
		var out []string
		for i, v := range vals {
			if b, ok := v.([]byte); ok {
				v = string(b)
			}
			if v == nil {
				v = "NULL"
			}
			out = append(out, fmt.Sprintf("%s=%v", cols[i], v))
		}
		fmt.Println(strings.Join(out, " | "))
	}
}
