package service

import (
	"context"
	"database/sql"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/MinChen05/JYX-BI/internal/engine"
	"github.com/MinChen05/JYX-BI/internal/model"
	"github.com/MinChen05/JYX-BI/internal/store"
	"github.com/MinChen05/JYX-BI/internal/template"
)

// 设计器（模板管理 + SQL 预览）相关服务。

var tplCodeRe = regexp.MustCompile(`^[a-z][a-z0-9_]*$`)

// trailingOrderRe 末尾的 ORDER BY 子句（列列表不含括号）。
var trailingOrderRe = regexp.MustCompile(`(?is)\s+ORDER\s+BY\s+[^()]+\s*$`)

// TemplateSummary 模板列表项。
type TemplateSummary struct {
	Code      string `json:"code"`
	Name      string `json:"name"`
	Version   int    `json:"version"`
	Group     string `json:"group"`
	HasSubmit bool   `json:"has_submit"`
}

// PreviewColumn SQL 预览列（含推断的设计器列类型，供"生成列"用）。
type PreviewColumn struct {
	Name   string `json:"name"`
	DBType string `json:"db_type"`
	Type   string `json:"type"` // text | int | money | date
}

type SqlPreviewResult struct {
	Columns []PreviewColumn `json:"columns"`
	Rows    [][]any         `json:"rows"`
}

// ListTemplates 模板列表。
func (s *Service) ListTemplates() []TemplateSummary {
	out := []TemplateSummary{}
	for _, code := range s.Engine.Codes() {
		def, _ := s.Engine.Get(code)
		out = append(out, TemplateSummary{
			Code: code, Name: def.Metadata.Name, Version: def.Metadata.Version,
			Group: def.Metadata.Group, HasSubmit: def.Spec.Submit.Target != "",
		})
	}
	return out
}

// GetTemplate 模板定义 + 原始 YAML。
func (s *Service) GetTemplate(code string) (*template.ReportDef, string, error) {
	def, ok := s.Engine.Get(code)
	if !ok {
		return nil, "", fmt.Errorf("模板不存在: %s", code)
	}
	return def, string(def.Raw), nil
}

func sampleParams(params []template.ParamDef) map[string]string {
	out := map[string]string{}
	for _, p := range params {
		switch p.Type {
		case "month":
			out[p.Key] = "2026-01"
		case "date":
			out[p.Key] = "2026-01-01"
		case "text":
			out[p.Key] = "x"
		}
	}
	return out
}

// SaveTemplate 保存模板：原始 YAML 直接落文件（保留注释与格式）→ 解析 → 热重载 → 样例参数编译校验。
// 任一环节失败都回滚文件并恢复旧模板。
func (s *Service) SaveTemplate(code string, raw []byte) error {
	if !tplCodeRe.MatchString(code) {
		return fmt.Errorf("报表 code 只能用小写字母数字下划线且以字母开头: %s", code)
	}
	if len(raw) == 0 {
		return fmt.Errorf("模板内容不能为空")
	}
	def, err := template.Parse(raw)
	if err != nil {
		return fmt.Errorf("模板解析校验失败: %w", err)
	}
	if def.Metadata.Code != code {
		return fmt.Errorf("YAML 内 metadata.code(%s) 与文件 code(%s) 不一致", def.Metadata.Code, code)
	}
	if def.Metadata.Name == "" {
		return fmt.Errorf("报表名称不能为空")
	}
	if def.Metadata.Version < 1 {
		def.Metadata.Version = 1
	}

	dir := s.Cfg.Templates.Dir
	path := filepath.Join(dir, code+".yaml")
	prev, _ := os.ReadFile(path)

	if err := os.WriteFile(path, raw, 0o644); err != nil {
		return err
	}

	rollback := func() {
		if prev != nil {
			_ = os.WriteFile(path, prev, 0o644)
		} else {
			_ = os.Remove(path)
		}
		_ = s.Engine.Reload(dir)
	}

	if err := s.Engine.Reload(dir); err != nil {
		rollback()
		return fmt.Errorf("模板加载失败: %w", err)
	}
	if _, err := s.Engine.Compile(code, sampleParams(def.Spec.Params)); err != nil {
		rollback()
		return fmt.Errorf("模板编译校验失败: %w", err)
	}
	s.registerAll()
	log.Printf("[admin] 模板已保存并热加载: %s (v%d)", code, def.Metadata.Version)
	return nil
}

// DeleteTemplate 删除模板文件并热重载。
func (s *Service) DeleteTemplate(code string) error {
	if !tplCodeRe.MatchString(code) {
		return fmt.Errorf("非法 code: %s", code)
	}
	dir := s.Cfg.Templates.Dir
	path := filepath.Join(dir, code+".yaml")
	if _, err := os.Stat(path); err != nil {
		return fmt.Errorf("模板文件不存在: %s", code)
	}
	if err := os.Remove(path); err != nil {
		return err
	}
	if err := s.Engine.Reload(dir); err != nil {
		return err
	}
	s.registerAll()
	log.Printf("[admin] 模板已删除并热加载: %s", code)
	return nil
}

// ReloadTemplates 重新加载模板目录（手工改 YAML 后生效）。
func (s *Service) ReloadTemplates() error {
	if err := s.Engine.Reload(s.Cfg.Templates.Dir); err != nil {
		return err
	}
	s.registerAll()
	return nil
}

// registerAll 模板版本落库（审计与追溯）。
func (s *Service) registerAll() {
	for _, code := range s.Engine.Codes() {
		def, _ := s.Engine.Get(code)
		t := &model.RptTemplate{
			Code: def.Metadata.Code, Version: def.Metadata.Version,
			YAML: string(def.Raw), Checksum: def.Checksum(def.Raw), UpdatedAt: time.Now(),
		}
		if err := store.UpsertTemplate(s.MySQL, t); err != nil {
			log.Printf("模板落库失败 %s: %v", code, err)
		}
	}
}

// SqlPreview 只读 SQL 预览（设计器"预览数据"）：
// 仅允许单条 SELECT/WITH，参数过白名单校验，结果限 100 行、15 秒超时。
func (s *Service) SqlPreview(source, rawSQL string, paramsDef []template.ParamDef, values map[string]string) (*SqlPreviewResult, error) {
	synthetic := &template.ReportDef{}
	synthetic.Spec.Params = paramsDef
	if err := engine.ValidateParams(synthetic, values); err != nil {
		return nil, err
	}
	q := strings.TrimRight(strings.TrimSpace(rawSQL), ";")
	if q == "" {
		return nil, fmt.Errorf("SQL 不能为空")
	}
	up := strings.ToUpper(q)
	if !strings.HasPrefix(up, "SELECT") && !strings.HasPrefix(up, "WITH") {
		return nil, fmt.Errorf("安全限制：仅允许 SELECT 查询")
	}
	if strings.Contains(q, ";") {
		return nil, fmt.Errorf("安全限制：不允许多条语句")
	}
	q = engine.ReplaceTokens(q, values)

	var sd *sql.DB
	switch source {
	case "mssql":
		if s.Mssql == nil {
			return nil, fmt.Errorf("mssql 未配置")
		}
		sd, _ = s.Mssql.DB()
		// 派生表内 ORDER BY 会被 SQL Server 拒绝（除非带 TOP）；预览只看列结构与样例行，去掉末尾排序
		q = trailingOrderRe.ReplaceAllString(q, "")
		q = "SELECT TOP 100 * FROM ( " + q + " ) AS jyx_preview"
	case "sql", "doris":
		if s.Doris == nil {
			return nil, fmt.Errorf("doris 未配置")
		}
		sd, _ = s.Doris.DB()
		q = "SELECT * FROM ( " + q + " ) jyx_preview LIMIT 100"
	default:
		return nil, fmt.Errorf("未知数据源 %q（mssql|doris）", source)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	rows, err := sd.QueryContext(ctx, q)
	if err != nil {
		return nil, fmt.Errorf("查询失败: %w", err)
	}
	defer rows.Close()

	names, _ := rows.Columns()
	colTypes, _ := rows.ColumnTypes()
	columns := make([]PreviewColumn, len(names))
	for i, ct := range colTypes {
		columns[i] = PreviewColumn{Name: names[i], DBType: ct.DatabaseTypeName(), Type: inferEditorType(ct.DatabaseTypeName())}
	}
	vals := make([]any, len(names))
	ptrs := make([]any, len(names))
	for i := range vals {
		ptrs[i] = &vals[i]
	}
	var out [][]any
	for rows.Next() {
		if err := rows.Scan(ptrs...); err != nil {
			return nil, err
		}
		r := make([]any, len(names))
		for i, v := range vals {
			r[i] = engine.NormalizeCell(v)
		}
		out = append(out, r)
	}
	return &SqlPreviewResult{Columns: columns, Rows: out}, nil
}

// inferEditorType 数据库列类型 → 设计器列类型。
func inferEditorType(dbType string) string {
	switch strings.ToUpper(dbType) {
	case "INT", "BIGINT", "SMALLINT", "TINYINT", "MEDIUMINT", "BIT", "INTEGER":
		return "int"
	case "DECIMAL", "NUMERIC", "MONEY", "SMALLMONEY", "FLOAT", "REAL", "DOUBLE":
		return "money"
	case "DATE", "DATETIME", "DATETIME2", "SMALLDATETIME", "TIMESTAMP", "TIME":
		return "date"
	default:
		return "text"
	}
}
