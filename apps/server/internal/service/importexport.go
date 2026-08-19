package service

import (
	"bytes"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/MinChen05/JYX-BI/internal/engine"
	"github.com/MinChen05/JYX-BI/internal/model"
	"github.com/MinChen05/JYX-BI/internal/store"
	"github.com/MinChen05/JYX-BI/internal/template"
	"github.com/MinChen05/JYX-BI/pkg/xlsxio"
)

// CellDiff 导入变更明细。
type CellDiff struct {
	RowKey string `json:"row_key"`
	Col    string `json:"col"`
	Old    any    `json:"old"`
	New    any    `json:"new"`
}

// ImportReport 导入阶段一结果（校验报告 + diff，未落库）。
type ImportReport struct {
	JobID     int64        `json:"job_id"`
	Status    string       `json:"status"`
	Errors    []template.Issue `json:"errors"`
	Changed   int          `json:"changed"`
	Unmatched []string     `json:"unmatched"`
	Cells     []CellDiff   `json:"cells"`
}

// ExportBytes 导出 xlsx（含 _meta）。
func (s *Service) ExportBytes(code string, params map[string]string) ([]byte, error) {
	spec, err := s.GetGrid(code, params)
	if err != nil {
		return nil, err
	}
	rows := make([]map[string]any, 0, len(spec.Rows))
	for _, r := range spec.Rows {
		rows = append(rows, r.Cells)
	}
	var buf bytes.Buffer
	meta := xlsxio.Meta{Tpl: code, Ver: spec.Version, Params: params}
	err = xlsxio.Write(&buf, meta, spec.Columns, rows, spec.NumberFormat, true)
	if err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

// ImportFile 阶段一：解析 + 校验 + diff，生成 ImportJob（不落业务数据）。
func (s *Service) ImportFile(code string, params map[string]string, fileName string, data []byte) (*ImportReport, error) {
	compiled, inst, err := s.load(code, params)
	if err != nil {
		return nil, err
	}
	if err := checkEditable(inst); err != nil {
		return nil, err
	}
	meta, headers, dataRows, err := xlsxio.Parse(data)
	if err != nil {
		return nil, errf("FILE", err.Error())
	}
	// _meta 一致性校验
	if meta.Tpl != code {
		return nil, errf("VERSION_MISMATCH", fmt.Sprintf("文件属于报表 %s，不能导入到 %s", meta.Tpl, code))
	}
	if meta.Ver != compiled.Def.Metadata.Version {
		return nil, errf("VERSION_MISMATCH", fmt.Sprintf("文件模板版本 %d，当前 %d，请重新导出", meta.Ver, compiled.Def.Metadata.Version))
	}
	for k, v := range params {
		if meta.Params[k] != v {
			return nil, errf("VERSION_MISMATCH", fmt.Sprintf("参数 %s 不一致（文件=%s 当前=%s）", k, meta.Params[k], v))
		}
	}

	base, err := engine.FetchBaseRows(s.Doris, compiled.Def, params)
	if err != nil {
		return nil, err
	}
	draft := s.draftData(inst)
	merged := engine.BuildRows(compiled, base, draft)
	rowKeyByMatch := map[string]string{} // 匹配键值 → rowKey
	for _, m := range merged {
		var b strings.Builder
		for _, k := range compiled.Def.MatchKeys() {
			b.WriteString(template.AsString(m.Cells[k]))
			b.WriteByte('|')
		}
		rowKeyByMatch[b.String()] = m.RowKey
	}

	// 表头 → 列 key
	labelToKey := map[string]string{}
	for _, c := range compiled.Cols {
		labelToKey[c.Label] = c.Key
	}
	editableKeys := map[string]string{}
	for _, c := range compiled.Cols {
		if !c.Readonly && c.Type != "auto" {
			editableKeys[c.Key] = c.Type
		}
	}

	newDraft := map[string]map[string]any{}
	var unmatched []string
	usedRows := map[string]bool{}
	for _, dr := range dataRows {
		row := map[string]any{}
		for ci, label := range headers {
			if ci >= len(dr) {
				break
			}
			key, ok := labelToKey[label]
			if !ok {
				continue
			}
			row[key] = parseCell(dr[ci], editableKeys[key])
		}
		// 行匹配
		var b strings.Builder
		all := true
		for _, k := range compiled.Def.MatchKeys() {
			sv := template.AsString(row[k])
			if sv == "" {
				all = false
				break
			}
			b.WriteString(sv)
			b.WriteByte('|')
		}
		rk := ""
		if all {
			rk = rowKeyByMatch[b.String()]
		}
		if rk == "" {
			unmatched = append(unmatched, template.AsString(row[compiled.Def.MatchKeys()[0]]))
			continue
		}
		usedRows[rk] = true
		cells := map[string]any{}
		for k, v := range row {
			if _, ok := editableKeys[k]; ok {
				cells[k] = v
			}
		}
		newDraft[rk] = cells
	}
	if len(unmatched) > 0 && compiled.Def.Spec.Import.OnUnmatched == "reject" {
		return nil, errf("UNMATCHED", fmt.Sprintf("文件包含 %d 行无法匹配的记录，已拒绝", len(unmatched)))
	}

	// 全量校验（base + 导入值）
	mergedNew := engine.BuildRows(compiled, base, newDraft)
	issues := compiled.Validator.Run(engine.ToMaps(mergedNew))
	for i := range issues {
		issues[i].RowKey = mergedNew[issues[i].RowIdx].RowKey
	}

	// diff：导入值 vs 当前草稿
	var diffs []CellDiff
	for rk, cells := range newDraft {
		oldCells := draft[rk]
		for ck, nv := range cells {
			ov, existed := oldCells[ck]
			if !existed || !valueEqual(ov, nv) {
				diffs = append(diffs, CellDiff{RowKey: rk, Col: ck, Old: ov, New: nv})
			}
		}
	}
	if len(diffs) > 200 {
		diffs = diffs[:200]
	}

	job := &model.RptImportJob{
		InstanceID: instID(inst),
		FileName:   fileName,
		FileSHA256: fmt.Sprintf("%x", sha256.Sum256(data)),
		MetaTpl:    meta.Tpl, MetaVer: meta.Ver,
		MetaParams: mustJSON(meta.Params),
		Status:     "validated",
		Op:         "admin", CreatedAt: time.Now(),
	}
	dataJSON, _ := json.Marshal(newDraft)
	job.Data = string(dataJSON)
	errJSON, _ := json.Marshal(issues)
	job.ErrorRpt = string(errJSON)
	diffJSON, _ := json.Marshal(map[string]any{"changed": len(diffs), "unmatched": len(unmatched), "cells": diffs})
	job.DiffSum = string(diffJSON)
	if err := s.MySQL.Create(job).Error; err != nil {
		return nil, err
	}
	return &ImportReport{JobID: job.ID, Status: job.Status, Errors: issues, Changed: len(diffs), Unmatched: unmatched, Cells: diffs}, nil
}

// ConfirmImport 阶段二：校验通过的作业落地到草稿。
func (s *Service) ConfirmImport(code string, params map[string]string, jobID int64) error {
	job := &model.RptImportJob{}
	if err := s.MySQL.First(job, jobID).Error; err != nil {
		return errf("NOT_FOUND", "导入作业不存在")
	}
	if job.Status != "validated" {
		return errf("BAD_STATE", "作业状态不允许确认: "+job.Status)
	}
	if job.ErrorRpt != "" && job.ErrorRpt != "null" {
		var issues []template.Issue
		_ = json.Unmarshal([]byte(job.ErrorRpt), &issues)
		if len(issues) > 0 {
			return errf("VALIDATION_FAILED", fmt.Sprintf("存在 %d 项校验错误，不能确认导入", len(issues)))
		}
	}
	compiled, inst, err := s.load(code, params)
	if err != nil {
		return err
	}
	if err := checkEditable(inst); err != nil {
		return err
	}
	_ = compiled
	var newDraft map[string]map[string]any
	_ = json.Unmarshal([]byte(job.Data), &newDraft)
	dataJSON, _ := json.Marshal(newDraft)
	now := time.Now()
	if inst == nil {
		inst = &model.RptInstance{
			ReportCode: code, TplVersion: compiled.Def.Metadata.Version,
			Params: mustJSON(params), ParamsHash: template.ParamsHash(params),
			Status: model.StatusDraft, Data: string(dataJSON),
			UpdatedAt: now, UpdatedBy: "admin",
		}
	} else {
		inst.Data = string(dataJSON)
		inst.UpdatedBy = "admin"
	}
	if err := store.SaveInstance(s.MySQL, inst); err != nil {
		return err
	}
	return s.MySQL.Model(&model.RptImportJob{}).Where("id = ?", job.ID).
		Update("status", "imported").Error
}

// SelfCheck 导出打印检测：导出 → 重解析 → 逐格 diff。
func (s *Service) SelfCheck(code string, params map[string]string) (map[string]any, error) {
	spec, err := s.GetGrid(code, params)
	if err != nil {
		return nil, err
	}
	buf, err := s.ExportBytes(code, params)
	if err != nil {
		return nil, err
	}
	_, headers, dataRows, err := xlsxio.Parse(buf)
	if err != nil {
		return nil, err
	}
	labelToKey := map[string]string{}
	for _, c := range spec.Columns {
		labelToKey[c.Label] = c.Key
	}
	var diffs []map[string]any
	for ri, dr := range dataRows {
		if ri >= len(spec.Rows) {
			break
		}
		for ci, label := range headers {
			if ci >= len(dr) {
				break
			}
			key, ok := labelToKey[label]
			if !ok {
				continue
			}
			want := spec.Rows[ri].Cells[key]
			got := parseCell(dr[ci], "")
			if !valueEqual(want, got) {
				diffs = append(diffs, map[string]any{
					"row": spec.Rows[ri].RowKey, "col": key, "want": want, "got": got,
				})
			}
		}
	}
	return map[string]any{
		"ok":    len(diffs) == 0,
		"rows":  len(spec.Rows),
		"diffs": diffs,
	}, nil
}

func parseCell(s, colType string) any {
	s = strings.TrimSpace(s)
	if s == "" {
		return nil
	}
	switch colType {
	case "money", "int":
		// GetRows 返回带格式的字符串（如 3,760），去掉千分位再解析
		if f, err := strconv.ParseFloat(strings.ReplaceAll(s, ",", ""), 64); err == nil {
			return f
		}
	}
	return s
}

func valueEqual(a, b any) bool {
	// nil 与空字符串视为同义
	if a == nil || template.AsString(a) == "" {
		return b == nil || template.AsString(b) == ""
	}
	if b == nil || template.AsString(b) == "" {
		return false
	}
	if fa, ok := toFloat(a); ok {
		if fb, ok2 := toFloat(b); ok2 {
			// xlsx 存储精度有限，浮点比较带相对容差
			diff := fa - fb
			if diff < 0 {
				diff = -diff
			}
			scale := fb
			if scale < 1 {
				scale = 1
			}
			if diff <= 1e-9*fa || diff <= 1e-9*scale {
				return true
			}
			return false
		}
	}
	return template.AsString(a) == template.AsString(b)
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
		f, err := strconv.ParseFloat(strings.TrimSpace(x), 64)
		return f, err == nil
	}
	return 0, false
}

func instID(inst *model.RptInstance) int64 {
	if inst == nil {
		return 0
	}
	return inst.ID
}
