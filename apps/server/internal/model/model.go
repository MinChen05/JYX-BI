package model

import "time"

// 实例状态
const (
	StatusDraft     int8 = 0 // 草稿
	StatusSubmitted int8 = 1 // 已提交（锁定）
	// StatusWithdrawn 不单独落库：撤回后回到 Draft，并在 submission 表留痕
)

// RptInstance 填报实例：一个 (模板, 参数组合) 唯一一份。
type RptInstance struct {
	ID          int64           `gorm:"primaryKey" json:"id"`
	ReportCode  string          `gorm:"size:64;uniqueIndex:uk_code_params" json:"report_code"`
	TplVersion  int             `json:"tpl_version"`
	Params      string          `gorm:"type:json" json:"-"` // JSON 字符串
	ParamsHash  string          `gorm:"size:64;uniqueIndex:uk_code_params" json:"params_hash"`
	Status      int8            `json:"status"`
	Data        string          `gorm:"type:json" json:"-"` // 网格值 JSON
	UpdatedAt   time.Time       `json:"updated_at"`
	UpdatedBy   string          `gorm:"size:64" json:"updated_by"`
	SubmittedAt *time.Time      `json:"submitted_at"`
}

func (RptInstance) TableName() string { return "rpt_instance" }

// RptSubmission 提交/撤回审计快照。
type RptSubmission struct {
	ID         int64     `gorm:"primaryKey" json:"id"`
	InstanceID int64     `gorm:"index" json:"instance_id"`
	Action     string    `gorm:"size:16" json:"action"` // submit / withdraw
	Snapshot   string    `gorm:"type:json" json:"-"`
	Op         string    `gorm:"size:64" json:"op"`
	CreatedAt  time.Time `json:"created_at"`
}

func (RptSubmission) TableName() string { return "rpt_submission" }

// RptImportJob 覆盖导入作业（两阶段：validated → imported / rejected）。
type RptImportJob struct {
	ID         int64     `gorm:"primaryKey" json:"id"`
	InstanceID int64     `gorm:"index" json:"instance_id"`
	FileName   string    `gorm:"size:255" json:"file_name"`
	FileSHA256 string    `gorm:"size:64" json:"file_sha256"`
	MetaTpl    string    `gorm:"size:64" json:"meta_tpl"`
	MetaVer    int       `json:"meta_ver"`
	MetaParams string    `gorm:"type:json" json:"-"`
	Data       string    `gorm:"type:json" json:"-"` // 校验通过的网格值（rowKey→cells），confirm 时落地
	Status     string    `gorm:"size:16" json:"status"` // validated / imported / rejected
	ErrorRpt   string    `gorm:"type:json" json:"-"`
	DiffSum    string    `gorm:"type:json" json:"-"`
	Op         string    `gorm:"size:64" json:"op"`
	CreatedAt  time.Time `json:"created_at"`
}

func (RptImportJob) TableName() string { return "rpt_import_job" }

// RptTemplate 模板版本（templates/*.yaml 的落库记录）。
type RptTemplate struct {
	Code      string    `gorm:"size:64;primaryKey" json:"code"`
	Version   int       `gorm:"primaryKey" json:"version"`
	YAML      string    `gorm:"type:text" json:"-"`
	Checksum  string    `gorm:"size:64" json:"checksum"`
	UpdatedAt time.Time `json:"updated_at"`
}

func (RptTemplate) TableName() string { return "rpt_template" }
