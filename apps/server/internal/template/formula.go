package template

import (
	"fmt"
	"regexp"
	"strings"

	"github.com/expr-lang/expr"
	"github.com/expr-lang/expr/vm"
)

// 公式引擎：行内表达式（v1 无跨行聚合）。
// 模板里 "sum(m01..m12)" 的 a..b 范围在编译期展开为实际列 key 列表，
// 运行期 env 为该行所有列值（缺失/空为 nil），sum/avg 自动跳过 nil。

var rangeRe = regexp.MustCompile(`(\w+)\.\.(\w+)`)

func sumFn(vals ...any) any {
	var s float64
	for _, v := range vals {
		if f, ok := toFloat(v); ok {
			s += f
		}
	}
	return s
}

func avgFn(vals ...any) any {
	var s float64
	var n int
	for _, v := range vals {
		if f, ok := toFloat(v); ok {
			s += f
			n++
		}
	}
	if n == 0 {
		return nil
	}
	return s / float64(n)
}

func toFloat(v any) (float64, bool) {
	switch x := v.(type) {
	case float64:
		return x, true
	case float32:
		return float64(x), true
	case int:
		return float64(x), true
	case int64:
		return float64(x), true
	case string:
		x = strings.TrimSpace(x)
		if x == "" {
			return 0, false
		}
		var f float64
		if _, err := fmt.Sscanf(x, "%f", &f); err == nil {
			return f, true
		}
	}
	return 0, false
}

// CompileFormula 编译公式为可执行 program。
// cols 用于展开 a..b 范围；在模板加载期调用，模板写错会直接 fail fast。
// 行变量（列 key）运行期才确定，故允许未定义变量。
func CompileFormula(formula string, cols []ColumnSpec) (*vm.Program, error) {
	expanded := rangeRe.ReplaceAllStringFunc(formula, func(m string) string {
		keys := ExpandRange(m, cols)
		if len(keys) == 0 {
			return m
		}
		return strings.Join(keys, ",")
	})
	env := map[string]any{
		"sum": sumFn,
		"avg": avgFn,
	}
	return expr.Compile(expanded, expr.Env(env), expr.AllowUndefinedVariables())
}

// EvalFormula 对单行求值。row 为该行的列值映射（列 key → 值，可为 nil）。
func EvalFormula(p *vm.Program, row map[string]any) (any, error) {
	env := make(map[string]any, len(row)+2)
	env["sum"] = sumFn
	env["avg"] = avgFn
	for k, v := range row {
		env[k] = v
	}
	out, err := expr.Run(p, env)
	if err != nil {
		return nil, err
	}
	if out == nil {
		return nil, nil
	}
	return out, nil
}
