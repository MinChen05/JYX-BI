package service

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/MinChen05/kingdee-rpt/internal/config"
	"github.com/MinChen05/kingdee-rpt/internal/engine"
	"github.com/MinChen05/kingdee-rpt/internal/model"
	"github.com/MinChen05/kingdee-rpt/internal/push"
	"github.com/MinChen05/kingdee-rpt/internal/store"
	"github.com/MinChen05/kingdee-rpt/internal/template"
	"gorm.io/gorm"
)

// AppError 业务错误（携带 code 供前端处理）。
type AppError struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

func (e *AppError) Error() string { return e.Code + ": " + e.Message }

func errf(code, msg string) *AppError { return &AppError{Code: code, Message: msg} }

// Service 填报业务。
type Service struct {
	Cfg    *config.Config
	MySQL  *gorm.DB
	Doris  *gorm.DB
	Engine *template.Engine
	Pusher *push.Pusher
}

func New(cfg *config.Config, mysqlDB, dorisDB *gorm.DB, eng *template.Engine, pusher *push.Pusher) *Service {
	return &Service{Cfg: cfg, MySQL: mysqlDB, Doris: dorisDB, Engine: eng, Pusher: pusher}
}

// ReportInfo 报表清单条目。
type ReportInfo struct {
	Code      string                `json:"code"`
	Name      string                `json:"name"`
	Version   int                   `json:"version"`
	Params    []template.ParamDef   `json:"params"`
	Instances []InstanceBrief       `json:"instances"`
}

type InstanceBrief struct {
	Params      map[string]string `json:"params"`
	Status      int8              `json:"status"`
	UpdatedAt   time.Time         `json:"updated_at"`
	UpdatedBy   string            `json:"updated_by"`
	SubmittedAt *time.Time        `json:"submitted_at"`
}

// ListReports 报表清单（模板 + 各参数实例状态）。
func (s *Service) ListReports() ([]ReportInfo, error) {
	infos := make([]ReportInfo, 0)
	for _, code := range s.Engine.Codes() {
		def, _ := s.Engine.Get(code)
		ri := ReportInfo{Code: code, Name: def.Metadata.Name, Version: def.Metadata.Version, Params: def.Spec.Params}
		infos = append(infos, ri)
	}
	insts, err := store.ListInstances(s.MySQL)
	if err != nil {
		return nil, err
	}
	for i := range infos {
		for _, inst := range insts {
			if inst.ReportCode != infos[i].Code {
				continue
			}
			var params map[string]string
			_ = json.Unmarshal([]byte(inst.Params), &params)
			infos[i].Instances = append(infos[i].Instances, InstanceBrief{
				Params: params, Status: inst.Status, UpdatedAt: inst.UpdatedAt,
				UpdatedBy: inst.UpdatedBy, SubmittedAt: inst.SubmittedAt,
			})
		}
	}
	return infos, nil
}

// load 模板编译 + 参数校验 + 实例加载。
func (s *Service) load(code string, params map[string]string) (*template.Compiled, *model.RptInstance, error) {
	def, ok := s.Engine.Get(code)
	if !ok {
		return nil, nil, errf("NOT_FOUND", "报表不存在: "+code)
	}
	if err := engine.ValidateParams(def, params); err != nil {
		return nil, nil, errf("BAD_PARAMS", err.Error())
	}
	compiled, err := s.Engine.Compile(code, params)
	if err != nil {
		return nil, nil, errf("TEMPLATE", err.Error())
	}
	hash := template.ParamsHash(params)
	inst, err := store.GetInstance(s.MySQL, code, hash)
	if err != nil {
		return nil, nil, err
	}
	return compiled, inst, nil
}

func (s *Service) draftData(inst *model.RptInstance) map[string]map[string]any {
	out := map[string]map[string]any{}
	if inst == nil || inst.Data == "" {
		return out
	}
	_ = json.Unmarshal([]byte(inst.Data), &out)
	return out
}

// GetGrid 取网格。
func (s *Service) GetGrid(code string, params map[string]string) (*engine.GridSpec, error) {
	compiled, inst, err := s.load(code, params)
	if err != nil {
		return nil, err
	}
	base, err := engine.FetchBaseRows(s.Doris, compiled.Def, params)
	if err != nil {
		return nil, errf("ROWS", err.Error())
	}
	merged := engine.BuildRows(compiled, base, s.draftData(inst))
	spec := &engine.GridSpec{
		Report:  code,
		Name:    compiled.Def.Metadata.Name,
		Version: compiled.Def.Metadata.Version,
		Params:  params,
		Columns: compiled.Cols,
		Rows:    engine.ToGridRows(compiled, merged),
		RowOps:       engine.RowOps{Add: compiled.Def.Spec.Rows.EditableRows, Delete: compiled.Def.Spec.Rows.EditableRows},
		NumberFormat: compiled.Def.Spec.Export.Layout.NumberFormat,
	}
	if inst != nil {
		spec.Instance = &engine.InstanceInfo{ID: inst.ID, Status: inst.Status, UpdatedAt: inst.UpdatedAt}
	}
	return spec, nil
}

// locked 已提交锁定提示。
func lockedErr() *AppError { return errf("LOCKED", "报表已提交并锁定，请先撤回再编辑") }

// checkEditable 状态检查。
func checkEditable(inst *model.RptInstance) error {
	if inst != nil && inst.Status == model.StatusSubmitted {
		return lockedErr()
	}
	return nil
}

// optimistic 乐观锁检查。
func optimistic(inst *model.RptInstance, expected *time.Time) error {
	if inst == nil {
		return nil
	}
	if expected != nil && !expected.Equal(inst.UpdatedAt) && inst.UpdatedAt.Sub(*expected) > time.Millisecond {
		return errf("VERSION_CONFLICT", "数据已被他人更新，请刷新后重试")
	}
	return nil
}

// SaveDraft 保存草稿（仅存可编辑列；提交前不写 Doris）。
func (s *Service) SaveDraft(code string, params map[string]string, req *engine.DraftRequest) (*engine.InstanceInfo, error) {
	compiled, inst, err := s.load(code, params)
	if err != nil {
		return nil, err
	}
	if err := checkEditable(inst); err != nil {
		return nil, err
	}
	if err := optimistic(inst, req.ExpectedUpdatedAt); err != nil {
		return nil, err
	}
	rows, err := engine.NormalizePayload(compiled, req.Rows)
	if err != nil {
		return nil, errf("BAD_REQUEST", err.Error())
	}
	dataJSON, _ := json.Marshal(rows)
	if inst == nil {
		inst = &model.RptInstance{
			ReportCode: code, TplVersion: compiled.Def.Metadata.Version,
			Params: mustJSON(params), ParamsHash: template.ParamsHash(params),
			Status: model.StatusDraft, Data: string(dataJSON),
			UpdatedAt: time.Now(), UpdatedBy: "admin",
		}
	} else {
		inst.TplVersion = compiled.Def.Metadata.Version
		inst.Data = string(dataJSON)
		inst.UpdatedBy = "admin"
	}
	if err := store.SaveInstance(s.MySQL, inst); err != nil {
		return nil, err
	}
	return &engine.InstanceInfo{ID: inst.ID, Status: inst.Status, UpdatedAt: inst.UpdatedAt}, nil
}

// ValidateGrid 干跑校验（不落库）。rows 为空则校验现有草稿。
func (s *Service) ValidateGrid(code string, params map[string]string, rows []engine.RowPayload) ([]template.Issue, error) {
	compiled, inst, err := s.load(code, params)
	if err != nil {
		return nil, err
	}
	base, err := engine.FetchBaseRows(s.Doris, compiled.Def, params)
	if err != nil {
		return nil, err
	}
	draft := s.draftData(inst)
	if len(rows) > 0 {
		if n, err := engine.NormalizePayload(compiled, rows); err == nil {
			draft = n
		}
	}
	merged := engine.BuildRows(compiled, base, draft)
	issues := compiled.Validator.Run(toMaps(merged))
	for i := range issues {
		issues[i].RowKey = merged[issues[i].RowIdx].RowKey
	}
	return issues, nil
}

// Submit 提交：全量校验 → MySQL 事务 → Doris upsert → 异步推送。
func (s *Service) Submit(code string, params map[string]string, req *engine.DraftRequest) (*engine.InstanceInfo, error) {
	compiled, inst, err := s.load(code, params)
	if err != nil {
		return nil, err
	}
	if err := optimistic(inst, req.ExpectedUpdatedAt); err != nil {
		return nil, err
	}
	base, err := engine.FetchBaseRows(s.Doris, compiled.Def, params)
	if err != nil {
		return nil, err
	}
	draft, err := engine.NormalizePayload(compiled, req.Rows)
	if err != nil {
		return nil, errf("BAD_REQUEST", err.Error())
	}
	merged := engine.BuildRows(compiled, base, draft)
	if issues := compiled.Validator.Run(toMaps(merged)); len(issues) > 0 {
		for i := range issues {
			issues[i].RowKey = merged[issues[i].RowIdx].RowKey
		}
		ae := errf("VALIDATION_FAILED", "存在校验错误，提交被拒绝")
		ae.Message = fmt.Sprintf("%s（%d 项）", ae.Message, len(issues))
		_ = issues
		return nil, ae
	}
	dataJSON, _ := json.Marshal(draft)
	now := time.Now()
	if inst == nil {
		inst = &model.RptInstance{
			ReportCode: code, TplVersion: compiled.Def.Metadata.Version,
			Params: mustJSON(params), ParamsHash: template.ParamsHash(params),
			Status: model.StatusDraft, Data: string(dataJSON),
			UpdatedAt: now, UpdatedBy: "admin",
		}
	}
	snapshot, _ := json.Marshal(map[string]any{"params": params, "rows": merged})
	inst.Status = model.StatusSubmitted
	inst.Data = string(dataJSON)
	inst.SubmittedAt = &now
	inst.UpdatedBy = "admin"
	if err := store.SaveInstance(s.MySQL, inst); err != nil {
		return nil, err
	}
	_ = store.AddSubmission(s.MySQL, &model.RptSubmission{
		InstanceID: inst.ID, Action: "submit", Snapshot: string(snapshot), Op: "admin", CreatedAt: now,
	})
	if err := s.writeDoris(compiled, params, merged); err != nil {
		return nil, errf("DORIS_WRITE", "Doris 写入失败: "+err.Error())
	}
	// 推送（异步，失败不阻塞）
	if s.Pusher != nil && len(compiled.Def.Spec.Push) > 0 {
		xlsxBytes, _ := s.ExportBytes(code, params)
		go s.Pusher.Push(compiled.Def, params, xlsxBytes)
	}
	return &engine.InstanceInfo{ID: inst.ID, Status: inst.Status, UpdatedAt: now}, nil
}

// Withdraw 撤回（已提交 → 草稿）。
func (s *Service) Withdraw(code string, params map[string]string) (*engine.InstanceInfo, error) {
	compiled, inst, err := s.load(code, params)
	if err != nil {
		return nil, err
	}
	if inst == nil || inst.Status != model.StatusSubmitted {
		return nil, errf("BAD_STATE", "当前不是已提交状态")
	}
	now := time.Now()
	inst.Status = model.StatusDraft
	inst.SubmittedAt = nil
	inst.UpdatedAt = now
	_ = store.SaveInstance(s.MySQL, inst)
	_ = store.AddSubmission(s.MySQL, &model.RptSubmission{
		InstanceID: inst.ID, Action: "withdraw", Snapshot: mustJSON(params), Op: "admin", CreatedAt: now,
	})
	_ = compiled
	return &engine.InstanceInfo{ID: inst.ID, Status: inst.Status, UpdatedAt: now}, nil
}

// Submissions 审计历史。
func (s *Service) Submissions(code string, params map[string]string) ([]model.RptSubmission, error) {
	_, inst, err := s.load(code, params)
	if err != nil {
		return nil, err
	}
	if inst == nil {
		return nil, nil
	}
	var out []model.RptSubmission
	err = s.MySQL.Where("instance_id = ?", inst.ID).Order("id DESC").Limit(50).Find(&out).Error
	return out, err
}

// writeDoris 按模板映射 upsert 事实表。
func (s *Service) writeDoris(compiled *template.Compiled, params map[string]string, rows []engine.MergedRow) error {
	if len(compiled.Doris) == 0 {
		return nil
	}
	cols := make([]string, 0, len(compiled.Doris))
	for _, t := range compiled.Doris {
		cols = append(cols, t.DstCol)
	}
	values := make([][]any, 0, len(rows))
	for _, r := range rows {
		val := make([]any, 0, len(cols))
		for _, t := range compiled.Doris {
			switch {
			case t.Now:
				val = append(val, time.Now().Format("2006-01-02 15:04:05"))
			case t.Param != "":
				val = append(val, engine.NormalizeForDoris(params[t.Param]))
			default:
				val = append(val, engine.NormalizeForDoris(r.Cells[t.SrcCol]))
			}
		}
		values = append(values, val)
	}
	return store.UpsertRows(s.Doris, compiled.Def.Spec.Submit.Doris.Table, cols, values)
}

func toMaps(merged []engine.MergedRow) []map[string]any {
	out := make([]map[string]any, len(merged))
	for i, m := range merged {
		out[i] = m.Cells
	}
	return out
}

func mustJSON(v any) string {
	b, _ := json.Marshal(v)
	return string(b)
}
