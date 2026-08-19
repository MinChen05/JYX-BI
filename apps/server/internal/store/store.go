package store

import (
	"fmt"
	"time"

	"github.com/MinChen05/kingdee-rpt/internal/model"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

// InitMySQL 系统库（草稿/状态/审计/模板版本）。
func InitMySQL(dsn string) (*gorm.DB, error) {
	db, err := gorm.Open(mysql.Open(dsn), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Warn),
	})
	if err != nil {
		return nil, fmt.Errorf("connect mysql: %w", err)
	}
	if err := db.AutoMigrate(
		&model.RptInstance{},
		&model.RptSubmission{},
		&model.RptImportJob{},
		&model.RptTemplate{},
	); err != nil {
		return nil, fmt.Errorf("migrate: %w", err)
	}
	return db, nil
}

// InitDoris 事实库（金蝶主数据 + 提交事实）。Doris 走 MySQL 协议。
// 连接失败不致命：静态行模板仍可用，SQL 行集模板在运行时报错。
func InitDoris(dsn string) (*gorm.DB, error) {
	if dsn == "" {
		return nil, nil
	}
	db, err := gorm.Open(mysql.Open(dsn), &gorm.Config{
		Logger:                 logger.Default.LogMode(logger.Silent),
		SkipDefaultTransaction: true,
		PrepareStmt:            true,
	})
	if err != nil {
		return nil, fmt.Errorf("connect doris: %w", err)
	}
	return db, nil
}

// GetInstance 按 (模板, 参数) 取实例。
func GetInstance(db *gorm.DB, code, paramsHash string) (*model.RptInstance, error) {
	var inst model.RptInstance
	err := db.Where("report_code = ? AND params_hash = ?", code, paramsHash).First(&inst).Error
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, nil
		}
		return nil, err
	}
	return &inst, nil
}

// SaveInstance 新建或整体覆盖实例。
func SaveInstance(db *gorm.DB, inst *model.RptInstance) error {
	if inst.ID == 0 {
		return db.Create(inst).Error
	}
	inst.UpdatedAt = time.Now()
	return db.Model(&model.RptInstance{}).Where("id = ?", inst.ID).
		Updates(map[string]any{
			"tpl_version":  inst.TplVersion,
			"status":       inst.Status,
			"data":         inst.Data,
			"updated_by":   inst.UpdatedBy,
			"updated_at":   inst.UpdatedAt,
			"submitted_at": inst.SubmittedAt,
		}).Error
}

// AddSubmission 记录提交/撤回快照。
func AddSubmission(db *gorm.DB, sub *model.RptSubmission) error {
	return db.Create(sub).Error
}

// UpsertTemplate 模板版本落库。
func UpsertTemplate(db *gorm.DB, t *model.RptTemplate) error {
	return db.Where("code = ? AND version = ?", t.Code, t.Version).
		Attrs(map[string]any{"yaml": t.YAML, "checksum": t.Checksum, "updated_at": t.UpdatedAt}).
		FirstOrCreate(t).Error
}

// ListInstances 全部实例（量小，直接全量拉取做报表清单）。
func ListInstances(db *gorm.DB) ([]model.RptInstance, error) {
	var out []model.RptInstance
	err := db.Order("report_code, params_hash").Find(&out).Error
	return out, err
}
