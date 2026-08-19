package service

import (
	"encoding/json"
	"fmt"
	"log"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/MinChen05/JYX-BI/internal/config"
	"github.com/MinChen05/JYX-BI/internal/engine"
	"github.com/MinChen05/JYX-BI/internal/model"
	"github.com/MinChen05/JYX-BI/internal/push"
	"github.com/MinChen05/JYX-BI/internal/store"
	"github.com/MinChen05/JYX-BI/internal/template"
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
	MySQL  *gorm.DB // 系统库（SQLite 或 MySQL）
	Doris  *gorm.DB
	Mssql  *gorm.DB
	Engine *template.Engine
	Pusher *push.Pusher
}

func New(cfg *config.Config, systemDB, dorisDB, mssqlDB *gorm.DB, eng *template.Engine, pusher *push.Pusher) *Service {
	return &Service{Cfg: cfg, MySQL: systemDB, Doris: dorisDB, Mssql: mssqlDB, Engine: eng, Pusher: pusher}
}

// rowsSourceDB 按模板行集来源选库。
func (s *Service) rowsSourceDB(def *template.ReportDef) (*gorm.DB, error) {
	switch def.Spec.Rows.Source {
	case "mssql":
		if s.Mssql == nil {
			return nil, errf("NO_SOURCE", "mssql 数据源未配置")
		}
		return s.Mssql, nil
	case "sql":
		if s.Doris == nil {
			return nil, errf("NO_SOURCE", "doris 数据源未配置")
		}
		return s.Doris, nil
	default:
		return nil, nil // static
	}
}

// ReportInfo 报表清单条目。
type ReportInfo struct {
	Code      string                `json:"code"`
	Name      string                `json:"name"`
	Version   int                   `json:"version"`
	Group     string                `json:"group"`
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
		params := def.Spec.Params
		if params == nil {
			params = []template.ParamDef{}
		}
		// 空切片保证 JSON 输出 [] 而非 null，前端可直接 .length
		ri := ReportInfo{
			Code: code, Name: def.Metadata.Name, Version: def.Metadata.Version,
			Group: def.Metadata.Group,
			Params: params, Instances: []InstanceBrief{},
		}
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
	srcDB, err := s.rowsSourceDB(compiled.Def)
	if err != nil {
		return nil, err
	}
	base, err := engine.FetchBaseRows(srcDB, compiled.Def, params)
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
	for _, c := range compiled.Cols {
		if !c.Readonly && c.Type != "auto" {
			spec.Editable = true
			break
		}
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
	srcDB, err := s.rowsSourceDB(compiled.Def)
	if err != nil {
		return nil, err
	}
	base, err := engine.FetchBaseRows(srcDB, compiled.Def, params)
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
	if len(compiled.Doris) == 0 && compiled.Def.Spec.Submit.Mssql.Table == "" {
		return nil, errf("READONLY", "展示报表不支持提交")
	}
	if err := optimistic(inst, req.ExpectedUpdatedAt); err != nil {
		return nil, err
	}
	srcDB, err := s.rowsSourceDB(compiled.Def)
	if err != nil {
		return nil, err
	}
	base, err := engine.FetchBaseRows(srcDB, compiled.Def, params)
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
	if err := s.writeBack(compiled, params, merged); err != nil {
		// 写回失败必须把实例回滚为草稿，避免"系统库已提交、生产库没数据"的不一致
		inst.Status = model.StatusDraft
		inst.SubmittedAt = nil
		inst.UpdatedAt = time.Now()
		_ = store.SaveInstance(s.MySQL, inst)
		return nil, errf("WRITEBACK_FAILED", "生产库写回失败，未提交: "+err.Error())
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

// writeBack 按模板配置写回目标库（doris upsert / mssql upsert / mssql unpivot）。
func (s *Service) writeBack(compiled *template.Compiled, params map[string]string, rows []engine.MergedRow) error {
	st := compiled.Def.Spec.Submit
	target := st.Target
	switch target {
	case "mssql":
		if s.Mssql == nil {
			return errf("NO_TARGET", "mssql 未配置，无法写回")
		}
		mode := st.Mode
		if mode == "" {
			mode = "upsert"
		}
		if mode == "unpivot" {
			return s.writeMssqlUnpivot(st.Mssql, compiled, params, rows)
		}
		return s.writeMssqlUpsert(st.Mssql, compiled, params, rows)
	case "", "doris":
		if len(compiled.Doris) == 0 {
			return nil
		}
		return s.writeDoris(compiled, params, rows)
	default:
		return errf("BAD_CONFIG", "未知写回目标 "+target)
	}
}

// writeDoris 按模板映射 upsert 事实表。
func (s *Service) writeDoris(compiled *template.Compiled, params map[string]string, rows []engine.MergedRow) error {
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

// resolveSrc 解析 mapping 源：param.x / now() / 网格列。
func resolveSrc(src string, row map[string]any, params map[string]string) any {
	if strings.HasPrefix(src, "param.") {
		return params[strings.TrimPrefix(src, "param.")]
	}
	if src == "now()" {
		return time.Now().Local()
	}
	return row[src]
}

// writeMssqlUpsert 宽行 → 单行 MERGE。
func (s *Service) writeMssqlUpsert(mw template.MssqlWrite, compiled *template.Compiled, params map[string]string, rows []engine.MergedRow) error {
	// 目标列 = mapping 的目标值集合（保序）
	var cols []string
	seen := map[string]bool{}
	for _, dst := range mw.Mapping {
		if !seen[dst] {
			seen[dst] = true
			cols = append(cols, dst)
		}
	}
	for _, k := range mw.Keys {
		if !seen[k] {
			return fmt.Errorf("mssql keys 列 %s 不在 mapping 目标中", k)
		}
	}
	var keyTuples [][]any
	for _, r := range rows {
		rowMap := map[string]any{}
		for src, dst := range mw.Mapping {
			rowMap[dst] = resolveSrc(src, r.Cells, params)
		}
		if err := store.UpsertRowMssql(s.Mssql, mw.Table, mw.Keys, cols, rowMap); err != nil {
			return err
		}
		keyTuples = append(keyTuples, keyTupleOf(mw.Keys, rowMap))
	}
	return s.deleteStaleRows(mw, params, keyTuples)
}

var mssqlIdentRe = regexp.MustCompile(`^[A-Za-z0-9_]+$`)

// keyTupleOf 按 keys 顺序从 rowMap 取键值组合。
func keyTupleOf(keys []string, rowMap map[string]any) []any {
	t := make([]any, len(keys))
	for i, k := range keys {
		t[i] = rowMap[k]
	}
	return t
}

// buildDeleteScope 按模板配置构建删除范围条件（SQL 片段 + 参数，占位符 @p1..@pN 按序）。
// 未配置任何删除语义时返回 ("", nil, false, nil)。
func buildDeleteScope(mw template.MssqlWrite, params map[string]string) (string, []any, bool, error) {
	if mw.Delete == "" && len(mw.DeleteWhere) == 0 {
		return "", nil, false, nil
	}
	switch mw.Delete {
	case "", "all", "month":
	default:
		return "", nil, false, fmt.Errorf("非法 delete 值 %q（可选 all|month）", mw.Delete)
	}
	var conds []string
	var args []any
	pi := 0
	nextP := func() string {
		pi++
		return fmt.Sprintf("@p%d", pi)
	}
	if mw.Delete == "month" {
		if mw.Pivot == nil {
			return "", nil, false, fmt.Errorf("delete:month 需要 pivot 配置（unpivot 模式）")
		}
		monthParam := "biz_date"
		if mw.Pivot.MonthParam != "" {
			monthParam = mw.Pivot.MonthParam
		}
		t, err := time.Parse("2006-01", params[monthParam])
		if err != nil {
			return "", nil, false, fmt.Errorf("delete:month 解析月份参数 %s 失败: %w", monthParam, err)
		}
		if !mssqlIdentRe.MatchString(mw.Pivot.DateCol) {
			return "", nil, false, fmt.Errorf("非法日期列名 %q", mw.Pivot.DateCol)
		}
		conds = append(conds, fmt.Sprintf("[%s] >= %s AND [%s] < %s",
			mw.Pivot.DateCol, nextP(), mw.Pivot.DateCol, nextP()))
		args = append(args, t.Format("2006-01-02"), t.AddDate(0, 1, 0).Format("2006-01-02"))
	}
	// map 遍历序随机，但条件之间是 AND，参数在循环内同步追加，不影响正确性
	for col, expr := range mw.DeleteWhere {
		if !mssqlIdentRe.MatchString(col) {
			return "", nil, false, fmt.Errorf("非法 delete_where 列名 %q", col)
		}
		val := expr
		if strings.HasPrefix(expr, "param.") {
			v, ok := params[strings.TrimPrefix(expr, "param.")]
			if !ok {
				return "", nil, false, fmt.Errorf("delete_where 引用了不存在的参数 %s", expr)
			}
			val = v
		}
		conds = append(conds, fmt.Sprintf("[%s] = %s", col, nextP()))
		args = append(args, val)
	}
	return strings.Join(conds, " AND "), args, true, nil
}

// deleteStaleRows 删除"期间"内键值组合不在提交集内的存量行（配置了删除语义时）。
// 保护：提交集为空时跳过（防止把期间数据清空）。
func (s *Service) deleteStaleRows(mw template.MssqlWrite, params map[string]string, submitted [][]any) error {
	scopeSQL, scopeArgs, enabled, err := buildDeleteScope(mw, params)
	if err != nil {
		return err
	}
	if !enabled {
		return nil
	}
	if len(submitted) == 0 {
		log.Printf("[submit-delete] %s: 提交集为空，跳过删除（安全保护）", mw.Table)
		return nil
	}
	existing, err := store.SelectKeyRows(s.Mssql, mw.Table, mw.Keys, scopeSQL, scopeArgs)
	if err != nil {
		return fmt.Errorf("查询存量键值失败: %w", err)
	}
	var stale [][]any
	for _, ex := range existing {
		found := false
		for _, sub := range submitted {
			if store.KeyTupleEq(ex, sub) {
				found = true
				break
			}
		}
		if !found {
			stale = append(stale, ex)
		}
	}
	if len(stale) == 0 {
		return nil
	}
	n, err := store.DeleteByKeys(s.Mssql, mw.Table, mw.Keys, scopeSQL, scopeArgs, stale)
	if err != nil {
		return fmt.Errorf("删除失效行失败: %w", err)
	}
	scopeDesc := scopeSQL
	if scopeDesc == "" {
		scopeDesc = "全表"
	}
	log.Printf("[submit-delete] %s: 删除 %d 行失效数据（范围: %s）", mw.Table, n, scopeDesc)
	return nil
}

// writeMssqlUnpivot 宽表日列 → 长表（每行材料 × 每个非空日 → 一条记录）。
func (s *Service) writeMssqlUnpivot(mw template.MssqlWrite, compiled *template.Compiled, params map[string]string, rows []engine.MergedRow) error {
	pv := mw.Pivot
	monthParam := "biz_date"
	if pv.MonthParam != "" {
		monthParam = pv.MonthParam
	}
	month := params[monthParam]
	if month == "" {
		return fmt.Errorf("unpivot 缺少月份参数 %s", monthParam)
	}
	pvTable := mw.Table
	var cols []string
	seen := map[string]bool{}
	addCol := func(c string) {
		if !seen[c] {
			seen[c] = true
			cols = append(cols, c)
		}
	}
	for _, dst := range mw.Mapping {
		addCol(dst)
	}
	addCol(pv.DateCol)
	addCol(pv.ValueCol)
	for _, k := range mw.Keys {
		addCol(k)
	}
	var keyTuples [][]any
	for _, r := range rows {
		for _, c := range compiled.Cols {
			if !strings.Contains(c.Key, pv.DayColTpl[:strings.Index(pv.DayColTpl, "{")]) {
				continue
			}
			day := dayFromKey(c.Key, pv.DayColTpl)
			if day <= 0 {
				continue
			}
			val := r.Cells[c.Key]
			if val == nil || fmt.Sprintf("%v", val) == "" {
				continue
			}
			date := fmt.Sprintf("%s-%02d", month, day)
			rowMap := map[string]any{}
			for src, dst := range mw.Mapping {
				rowMap[dst] = resolveSrc(src, r.Cells, params)
			}
			rowMap[pv.DateCol] = date
			rowMap[pv.ValueCol] = val
			if err := store.UpsertRowMssql(s.Mssql, pvTable, mw.Keys, cols, rowMap); err != nil {
				return err
			}
			keyTuples = append(keyTuples, keyTupleOf(mw.Keys, rowMap))
		}
	}
	return s.deleteStaleRows(mw, params, keyTuples)
}

// dayFromKey 从动态列 key（如 d05）解析 day 号，依据 DayColTpl（d{day}）定位数字后缀。
func dayFromKey(key, tpl string) int {
	base := tpl[:strings.Index(tpl, "{")]
	if !strings.HasPrefix(key, base) {
		return 0
	}
	suffix := key[len(base):]
	n, err := strconv.Atoi(suffix)
	if err != nil {
		return 0
	}
	return n
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
