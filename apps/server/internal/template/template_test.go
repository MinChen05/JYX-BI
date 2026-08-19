package template

import (
	"fmt"
	"testing"
)

func TestDaysInMonth(t *testing.T) {
	cases := map[string]int{"2026-01": 31, "2026-02": 28, "2024-02": 29, "2026-12": 31}
	for m, want := range cases {
		got, err := daysInMonth(m)
		if err != nil || got != want {
			t.Errorf("daysInMonth(%s) = %d, %v; want %d", m, got, err, want)
		}
	}
}

func TestExpandColumnsDynamic(t *testing.T) {
	def, err := Parse([]byte(`
apiVersion: rpt/v1
kind: Report
metadata: {code: t, name: t, version: 1}
spec:
  params: [{key: biz_date, type: month}]
  rows: {source: static}
  columns:
    - {key: name, type: text, readonly: true}
    - {key: day, type: money, dynamic: {expr: "days(param.biz_date)", key: "d{day:02}", label: "{biz_date:MM}-{day:02}"}}
`))
	if err != nil {
		t.Fatal(err)
	}
	cols, err := ExpandColumns(def, map[string]string{"biz_date": "2026-02"})
	if err != nil {
		t.Fatal(err)
	}
	if len(cols) != 29 { // 1 静态 + 2 月 28 天
		t.Fatalf("cols = %d, want 29", len(cols))
	}
	if cols[1].Key != "d01" || cols[1].Label != "02-01" {
		t.Errorf("first dyn col = %+v", cols[1])
	}
	if cols[28].Key != "d28" || cols[28].Label != "02-28" {
		t.Errorf("last dyn col = %+v", cols[28])
	}
}

func TestFormulaSumRange(t *testing.T) {
	cols := make([]ColumnSpec, 12)
	for i := 0; i < 12; i++ {
		cols[i] = ColumnSpec{Key: padKey("m", i+1), Type: "money"}
	}
	p, err := CompileFormula("sum(m01..m12)", cols)
	if err != nil {
		t.Fatal(err)
	}
	row := map[string]any{}
	for i := 0; i < 12; i++ {
		row[padKey("m", i+1)] = float64(i + 1)
	}
	v, err := EvalFormula(p, row)
	if err != nil {
		t.Fatal(err)
	}
	if v != float64(78) { // 1+..+12
		t.Errorf("sum = %v, want 78", v)
	}
}

func TestValidatorUniqueAndRange(t *testing.T) {
	cols := []ColumnSpec{{Key: "year"}, {Key: "ftype"}, {Key: "m01", Type: "money"}}
	v, err := CompileValidation(Spec{Validation: []ValidationDef{
		{Cols: []string{"m01"}, Rule: "v >= 0"},
		{Rule: "unique(year, ftype)"},
	}}, cols)
	if err != nil {
		t.Fatal(err)
	}
	rows := []map[string]any{
		{"year": 2026, "ftype": "a", "m01": 10},
		{"year": 2026, "ftype": "a", "m01": 5},  // 重复
		{"year": 2027, "ftype": "b", "m01": -1}, // 负数
	}
	issues := v.Run(rows)
	if len(issues) != 2 {
		t.Fatalf("issues = %d, want 2: %+v", len(issues), issues)
	}
}

func padKey(prefix string, n int) string {
	return fmt.Sprintf("%s%02d", prefix, n)
}
