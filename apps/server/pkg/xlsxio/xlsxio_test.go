package xlsxio

import (
	"bytes"
	"testing"

	"github.com/MinChen05/kingdee-rpt/internal/template"
	"github.com/xuri/excelize/v2"
)

// TestRoundTrip 导出→解析 往返保真（M3 核心不变量）。
func TestRoundTrip(t *testing.T) {
	cols := []template.ColumnSpec{
		{Key: "material", Label: "材料名称", Type: "text"},
		{Key: "d01", Label: "01-01", Type: "money"},
		{Key: "d02", Label: "01-02", Type: "money"},
	}
	rows := []map[string]any{
		{"material": "40Cr", "d01": 3760.0, "d02": 3770.0},
		{"material": "SPCC", "d01": nil, "d02": 3825.0},
	}
	var buf bytes.Buffer
	meta := Meta{Tpl: "t", Ver: 1, Params: map[string]string{"biz_date": "2026-01"}}
	if err := Write(&buf, meta, cols, rows, "#,##0", true); err != nil {
		t.Fatal(err)
	}
	gotMeta, headers, dataRows, err := Parse(buf.Bytes())
	if err != nil {
		t.Fatal(err)
	}
	if gotMeta.Tpl != "t" || gotMeta.Ver != 1 || gotMeta.Params["biz_date"] != "2026-01" {
		t.Errorf("meta round-trip failed: %+v", gotMeta)
	}
	if len(headers) != 3 || len(dataRows) != 2 {
		t.Fatalf("shape = %d headers, %d rows", len(headers), len(dataRows))
	}
	if dataRows[0][0] != "40Cr" || dataRows[0][1] != "3,760" { // GetRows 返回格式化字符串
		t.Errorf("row0 = %v", dataRows[0])
	}
	if dataRows[1][1] != "" { // nil 导出为空
		t.Errorf("row1 d01 = %q, want empty", dataRows[1][1])
	}
}

// TestParseRejectsForeignFile 非本系统导出的文件必须被拒绝。
func TestParseRejectsForeignFile(t *testing.T) {
	f := excelize.NewFile()
	defer f.Close()
	_ = f.SetCellValue("Sheet1", "A1", "随便")
	var buf bytes.Buffer
	if err := f.Write(&buf); err != nil {
		t.Fatal(err)
	}
	if _, _, _, err := Parse(buf.Bytes()); err == nil {
		t.Error("expected error for file without _meta")
	}
}
