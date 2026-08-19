package xlsxio

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"strconv"
	"time"

	"github.com/MinChen05/JYX-BI/internal/template"
	"github.com/xuri/excelize/v2"
)

const (
	MetaSheet = "_meta"
	DataSheet = "报表"
)

// Meta 文件身份：覆盖导入时用于校验模板/版本/参数一致性。
type Meta struct {
	Tpl         string            `json:"tpl"`
	Ver         int               `json:"ver"`
	Params      map[string]string `json:"params"`
	Columns     []Col             `json:"columns"`
	GeneratedAt string            `json:"generated_at"`
	Generator   string            `json:"generator"`
}

type Col struct {
	Key   string `json:"key"`
	Label string `json:"label"`
}

// Write 按表单格式导出：表头 + 数据 + 隐藏 _meta sheet。
func Write(w io.Writer, meta Meta, cols []template.ColumnSpec, rows []map[string]any, numberFormat string, freeze bool) error {
	meta.GeneratedAt = time.Now().Format(time.RFC3339)
	meta.Generator = "jyx-bi/1.0"
	if meta.Columns == nil {
		meta.Columns = make([]Col, 0, len(cols))
		for _, c := range cols {
			meta.Columns = append(meta.Columns, Col{Key: c.Key, Label: c.Label})
		}
	}
	f := excelize.NewFile()
	defer f.Close()
	f.SetSheetName("Sheet1", DataSheet)

	for ci, c := range cols {
		cell, _ := excelize.CoordinatesToCellName(ci+1, 1)
		if err := f.SetCellValue(DataSheet, cell, c.Label); err != nil {
			return err
		}
		width := c.Width
		if width <= 0 {
			width = 14
		}
		_ = f.SetColWidth(DataSheet, colLetter(ci+1), colLetter(ci+1), float64(width))
	}

	for ri, row := range rows {
		for ci, c := range cols {
			v := row[c.Key]
			if v == nil {
				continue
			}
			cell, _ := excelize.CoordinatesToCellName(ci+1, ri+2)
			if c.Type == "money" || c.Type == "int" {
				if n, err := toFloatAny(v); err == nil {
					if err := f.SetCellValue(DataSheet, cell, n); err != nil {
						return err
					}
					if numberFormat != "" {
						style, _ := f.NewStyle(&excelize.Style{NumFmt: numFmtID(numberFormat)})
						_ = f.SetCellStyle(DataSheet, cell, cell, style)
					}
					continue
				}
			}
			if err := f.SetCellValue(DataSheet, cell, v); err != nil {
				return err
			}
		}
	}
	if freeze {
		_ = f.SetPanes(DataSheet, &excelize.Panes{Freeze: true, YSplit: 1, TopLeftCell: "A2", ActivePane: "bottomLeft"})
	}

	if _, err := f.NewSheet(MetaSheet); err != nil {
		return err
	}
	_ = f.SetSheetVisible(MetaSheet, false)
	metaJSON, _ := json.Marshal(meta)
	if err := f.SetCellValue(MetaSheet, "A1", string(metaJSON)); err != nil {
		return err
	}
	return f.Write(w)
}

// Parse 解析导出的文件：返回 meta、表头行、数据行（字符串）。
func Parse(data []byte) (Meta, []string, [][]string, error) {
	f, err := excelize.OpenReader(bytes.NewReader(data))
	if err != nil {
		return Meta{}, nil, nil, fmt.Errorf("解析 xlsx: %w", err)
	}
	defer f.Close()

	var meta Meta
	if sheets := f.GetSheetList(); containsStr(sheets, MetaSheet) {
		if v, err := f.GetCellValue(MetaSheet, "A1"); err == nil && v != "" {
			if err := json.Unmarshal([]byte(v), &meta); err != nil {
				return Meta{}, nil, nil, fmt.Errorf("_meta 损坏: %w", err)
			}
		}
	} else {
		return Meta{}, nil, nil, fmt.Errorf("文件缺少 _meta sheet，不是本系统导出的表单")
	}

	dataSheet := DataSheet
	if _, err := f.GetSheetIndex(DataSheet); err != nil {
		for _, s := range f.GetSheetList() {
			if s != MetaSheet {
				dataSheet = s
				break
			}
		}
	}
	all, err := f.GetRows(dataSheet, excelize.Options{RawCellValue: true})
	if err != nil {
		return Meta{}, nil, nil, err
	}
	if len(all) == 0 {
		return meta, nil, nil, fmt.Errorf("数据区为空")
	}
	return meta, all[0], all[1:], nil
}

// numFmtID 映射模板数字格式到 Excel 内置格式 ID（0=General）。
var numFmtMap = map[string]int{"#,##0": 3, "#,##0.00": 4, "0": 1, "0.00": 2}

func numFmtID(format string) int {
	if id, ok := numFmtMap[format]; ok {
		return id
	}
	return 0
}

func colLetter(i int) string {
	// 1→A, 26→Z, 27→AA
	name := ""
	for i > 0 {
		i--
		name = string(rune('A'+i%26)) + name
		i /= 26
	}
	return name
}

func containsStr(ss []string, s string) bool {
	for _, x := range ss {
		if x == s {
			return true
		}
	}
	return false
}

func toFloatAny(v any) (float64, error) {
	switch x := v.(type) {
	case float64:
		return x, nil
	case float32:
		return float64(x), nil
	case int:
		return float64(x), nil
	case int64:
		return float64(x), nil
	case string:
		return strconv.ParseFloat(x, 64)
	}
	return 0, fmt.Errorf("not a number")
}
