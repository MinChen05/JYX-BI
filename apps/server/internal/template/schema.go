package template

// 报表模板定义（对应 templates/*.yaml，规范见 docs/template-spec.md）。

type ReportDef struct {
	APIVersion string   `yaml:"apiVersion"`
	Kind       string   `yaml:"kind"`
	Metadata   Metadata `yaml:"metadata"`
	Spec       Spec     `yaml:"spec"`

	Raw []byte `yaml:"-"` // 原始 YAML（落库审计用）
}

type Metadata struct {
	Code    string `yaml:"code"`
	Name    string `yaml:"name"`
	Version int    `yaml:"version"`
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
	Key      string `yaml:"key"`
	Label    string `yaml:"label"`
	Type     string `yaml:"type"` // month | date | text
	Required bool   `yaml:"required"`
}

type RowsDef struct {
	Source       string         `yaml:"source"` // sql | static
	Query        string         `yaml:"query"`
	EditableRows bool           `yaml:"editable_rows"`
	StaticRows   []map[string]any `yaml:"static_rows"`
}

type ColumnDef struct {
	Key      string      `yaml:"key"`
	Label    string      `yaml:"label"`
	Type     string      `yaml:"type"` // text | money | int | date | month | auto
	Readonly bool        `yaml:"readonly"`
	Formula  string      `yaml:"formula"`
	Dynamic  *DynamicDef `yaml:"dynamic"`
	Width    int         `yaml:"width"`
}

// DynamicDef 参数驱动列展开。v1 仅支持 expr: days(param.<key>)。
type DynamicDef struct {
	Expr  string `yaml:"expr"`
	Key   string `yaml:"key"`   // 如 d{day}
	Label string `yaml:"label"` // 如 {biz_date:MM}-{day:02}
}

type ValidationDef struct {
	Cols []string `yaml:"cols"` // 列 key 或范围/通配，如 ["m01..m12"]、["day.*"]
	Rule string   `yaml:"rule"` // 表达式；特殊值: required / unique(c1, c2)
}

type ImportDef struct {
	Mode        string   `yaml:"mode"` // overwrite
	MatchKeys   []string `yaml:"match_keys"`
	OnUnmatched string   `yaml:"on_unmatched"` // report | reject | add
}

type ExportDef struct {
	Layout ExportLayout `yaml:"layout"`
}

type ExportLayout struct {
	FreezeHeader bool   `yaml:"freeze_header"`
	NumberFormat string `yaml:"number_format"`
}

type SubmitDef struct {
	Doris     DorisDef `yaml:"doris"`
	LockAfter bool     `yaml:"lock_after"`
}

// DorisDef 提交写回配置。mapping 支持 day.{day} 模板（按动态列展开）。
type DorisDef struct {
	Table   string            `yaml:"table"`
	Mapping map[string]string `yaml:"mapping"`
}

type PushDef struct {
	Channel string `yaml:"channel"` // email | dingtalk
	To      string `yaml:"to"`
	On      string `yaml:"on"` // submit
}
