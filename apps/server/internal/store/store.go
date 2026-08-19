package store

import (
	"fmt"

	"github.com/MinChen05/kingdee-rpt/internal/model"
	"github.com/glebarez/sqlite"
	"gorm.io/driver/mysql"
	gsqlserver "gorm.io/driver/sqlserver"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

// InitSystemDB 系统库：dsn 以 "/" 或 ".db" 结尾 → SQLite 文件，否则 MySQL DSN。
func InitSystemDB(dsn string) (*gorm.DB, error) {
	var db *gorm.DB
	var err error
	if isSQLitePath(dsn) {
		db, err = gorm.Open(sqlite.Open(dsn), &gorm.Config{
			Logger: logger.Default.LogMode(logger.Warn),
		})
	} else {
		db, err = gorm.Open(mysql.Open(dsn), &gorm.Config{
			Logger: logger.Default.LogMode(logger.Warn),
		})
	}
	if err != nil {
		return nil, fmt.Errorf("connect system db: %w", err)
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

func isSQLitePath(dsn string) bool {
	return len(dsn) > 0 && (dsn[0] == '/' || len(dsn) > 3 && dsn[len(dsn)-3:] == ".db")
}

// InitDoris 事实/主数据库（MySQL 协议）。未配置返回 nil。
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

// InitMssql 生产 MSSQL（报表源表 + 写回）。未配置返回 nil。
func InitMssql(dsn string) (*gorm.DB, error) {
	if dsn == "" {
		return nil, nil
	}
	db, err := gorm.Open(gsqlserver.Open(dsn), &gorm.Config{
		Logger:                 logger.Default.LogMode(logger.Silent),
		SkipDefaultTransaction: true,
	})
	if err != nil {
		return nil, fmt.Errorf("connect mssql: %w", err)
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
