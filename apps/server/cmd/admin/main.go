package main

import (
	"flag"
	"fmt"
	"os"

	"github.com/MinChen05/JYX-BI/internal/template"
)

// admin 运维小工具。
//
//	admin validate-tpl -dir ../../templates   校验全部模板（含动态列/公式/映射展开）
func main() {
	if len(os.Args) < 2 {
		fmt.Fprintln(os.Stderr, "usage: admin <validate-tpl> [flags]")
		os.Exit(2)
	}
	switch os.Args[1] {
	case "validate-tpl":
		fs := flag.NewFlagSet("validate-tpl", flag.ExitOnError)
		dir := fs.String("dir", "../../templates", "模板目录")
		fs.Parse(os.Args[2:])
		if err := validate(*dir); err != nil {
			fmt.Fprintln(os.Stderr, "FAIL:", err)
			os.Exit(1)
		}
		fmt.Println("OK: 全部模板通过校验")
	default:
		fmt.Fprintf(os.Stderr, "unknown command %q\n", os.Args[1])
		os.Exit(2)
	}
}

func validate(dir string) error {
	defs, err := template.LoadDir(dir)
	if err != nil {
		return err
	}
	eng, err := template.NewEngine(dir)
	if err != nil {
		return err
	}
	for _, code := range eng.Codes() {
		// 用样例参数触发动态列展开 + 公式 + 写回映射的完整编译
		params := map[string]string{}
		if def, ok := eng.Get(code); ok {
			for _, p := range def.Spec.Params {
				switch p.Type {
				case "month":
					params[p.Key] = "2026-01"
				case "date":
					params[p.Key] = "2026-01-01"
				case "text":
					params[p.Key] = "x"
				}
			}
		}
		if _, err := eng.Compile(code, params); err != nil {
			return fmt.Errorf("report %s: %w", code, err)
		}
		fmt.Printf("  ✓ %s (v%d, %d 列样例展开)\n", code, defs[code].Metadata.Version, len(defs[code].Spec.Columns))
	}
	return nil
}
