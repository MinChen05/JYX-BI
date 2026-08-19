package template

// 报表模板定义（对应 templates/*.yaml，规范见 docs/template-spec.md）。

type ReportDef struct {
	APIVersion string   `yaml:"apiVersion" json:"apiVersion"`
	Kind       string   `yaml:"kind" json:"kind"`
	Metadata   Metadata `yaml:"metadata" json:"metadata"`
	Spec       Spec     `yaml:"spec" json:"spec"`

	Raw []byte `yaml:"-" json:"-"` // 原始 YAML（落库审计用）
}

type Metadata struct {
	Code    string `yaml:"code" json:"code"`
	Name    string `yaml:"name" json:"name"`
	Version int    `yaml:"version" json:"version"`
	// Group 目录板块（门户左侧树分组，空 = 未分组）
	Group string `yaml:"group" json:"group"`
}

type Spec struct {
	Params     []ParamDef      `yaml:"params"`
	Rows       RowsDef         `yaml:"rows"`
	Columns    []ColumnDef     `yaml:"columns"`
	Validation []ValidationDef `yaml:"validation"`
	Import     ImportDef       `yaml:"import"`
	Export     ExportDef       `yaml:"export"`
	Submit     SubmitDef       `yaml:"submit"`
	Push       []PushDef       `yaml:"push"`
}

type ParamDef struct {
	Key      string `yaml:"key" json:"key"`
	Label    string `yaml:"label" json:"label"`
	Type     string `yaml:"type" json:"type"` // month | date | text
	Required bool   `yaml:"required" json:"required"`
}

type RowsDef struct {
	Source       string           `yaml:"source" json:"source"` // sql(Doris) | mssql | static
	Query        string           `yaml:"query" json:"query"`
	EditableRows bool             `yaml:"editable_rows" json:"editable_rows"`
	StaticRows   []map[string]any `yaml:"static_rows" json:"static_rows"`
}

type ColumnDef struct {
	Key      string      `yaml:"key" json:"key"`
	Label    string      `yaml:"label" json:"label"`
	Type     string      `yaml:"type" json:"type"` // text | money | int | date | month | auto
	Readonly bool        `yaml:"readonly" json:"readonly"`
	Formula  string      `yaml:"formula" json:"formula"`
	Dynamic  *DynamicDef `yaml:"dynamic" json:"dynamic"`
	Width    int         `yaml:"width" json:"width"`
}

// DynamicDef 参数驱动列展开。v1 仅支持 expr: days(param.<key>)。
type DynamicDef struct {
	Expr  string `yaml:"expr" json:"expr"`
	Key   string `yaml:"key" json:"key"`     // 如 d{day}
	Label string `yaml:"label" json:"label"` // 如 {biz_date:MM}-{day:02}
}

type ValidationDef struct {
	Cols []string `yaml:"cols" json:"cols"` // 列 key 或范围/通配，如 ["m01..m12"]、["day.*"]
	Rule string   `yaml:"rule" json:"rule"` // 表达式；特殊值: required / unique(c1, c2)
}

type ImportDef struct {
	Mode        string   `yaml:"mode" json:"mode"` // overwrite
	MatchKeys   []string `yaml:"match_keys" json:"match_keys"`
	OnUnmatched string   `yaml:"on_unmatched" json:"on_unmatched"` // report | reject | add
}

type ExportDef struct {
	Layout ExportLayout `yaml:"layout" json:"layout"`
}

type ExportLayout struct {
	FreezeHeader bool   `yaml:"freeze_header" json:"freeze_header"`
	NumberFormat string `yaml:"number_format" json:"number_format"`
}

type SubmitDef struct {
	Target    string     `yaml:"target" json:"target"` // doris | mssql；缺省按 DorisDef 存在与否判断
	Mode      string     `yaml:"mode" json:"mode"`     // upsert(缺省) | unpivot(宽表→长表)
	Doris     DorisDef   `yaml:"doris" json:"doris"`
	Mssql     MssqlWrite `yaml:"mssql" json:"mssql"`
	LockAfter bool       `yaml:"lock_after" json:"lock_after"`
}

// DorisDef 提交写回配置。mapping 支持动态列模板与 "param.x" / "now()" 特殊源。
type DorisDef struct {
	Table   string            `yaml:"table" json:"table"`
	Mapping map[string]string `yaml:"mapping" json:"mapping"`
}

// MssqlWrite MSSQL 写回配置。
// upsert: keys 为匹配列（目标表列名），值取自 mapping 的源列。
// unpivot: 每个日列的非空值展开为一行长表记录。
//
// 删除语义（可选，二者/三者都不配 = 只增改不删）：
//   - delete: all    提交后删除全表中键值组合不在提交集内的行（全表行集报表）
//   - delete: month  仅 unpivot：删除 pivot 日期列所在月份区间内不在提交集内的行
//   - delete_where   期间范围条件（列 → param.x 或字面量，多列 AND），
//     在其范围内删除键值组合不在提交集内的行
//
// 保护：提交集为空时跳过删除（防止清空期间数据）。
type MssqlWrite struct {
	Table       string            `yaml:"table" json:"table"`
	Keys        []string          `yaml:"keys" json:"keys"`
	Mapping     map[string]string `yaml:"mapping" json:"mapping"` // 网格列/param.x/now() → 目标列（unpivot 时用于行级静态字段）
	Pivot       *PivotDef         `yaml:"pivot" json:"pivot"`     // mode=unpivot 必填
	Delete      string            `yaml:"delete" json:"delete"`   // all | month
	DeleteWhere map[string]string `yaml:"delete_where" json:"delete_where"`
}

type PivotDef struct {
	MonthParam string `yaml:"month_param" json:"month_param"` // 月份参数名，缺省 biz_date
	DayColTpl  string `yaml:"day_cols" json:"day_cols"`       // 如 d{day}，与动态列 key 同构
	DateCol    string `yaml:"date_col" json:"date_col"`       // 目标日期列
	ValueCol   string `yaml:"value_col" json:"value_col"`     // 目标值列
}

type PushDef struct {
	Channel string `yaml:"channel" json:"channel"` // email | dingtalk
	To      string `yaml:"to" json:"to"`
	On      string `yaml:"on" json:"on"` // submit
}
