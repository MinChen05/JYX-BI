package main

import (
	"flag"
	"log"
	"net/http"
	"time"

	"github.com/MinChen05/kingdee-rpt/internal/config"
	"github.com/MinChen05/kingdee-rpt/internal/httpapi"
	"github.com/MinChen05/kingdee-rpt/internal/model"
	"github.com/MinChen05/kingdee-rpt/internal/push"
	"github.com/MinChen05/kingdee-rpt/internal/service"
	"github.com/MinChen05/kingdee-rpt/internal/store"
	"github.com/MinChen05/kingdee-rpt/internal/template"
	"gorm.io/gorm"
)

var version = "dev"

func main() {
	cfgPath := flag.String("config", "config.ini", "配置文件路径")
	flag.StringVar(&version, "version", version, "版本号（构建时注入）")
	flag.Parse()

	cfg, err := config.Load(*cfgPath)
	if err != nil {
		log.Fatalf("配置加载失败: %v", err)
	}
	if cfg.System.DSN == "" {
		log.Fatal("config.ini 缺少 system.dsn（SQLite 路径或 MySQL DSN）")
	}
	systemDB, err := store.InitSystemDB(cfg.System.DSN)
	if err != nil {
		log.Fatalf("系统库初始化失败: %v", err)
	}
	dorisDB, err := store.InitDoris(cfg.Doris.DSN)
	if err != nil {
		log.Fatalf("Doris 初始化失败: %v", err)
	}
	if dorisDB == nil {
		log.Println("提示: 未配置 doris.dsn（仅 SQL 行集模板会不可用）")
	}
	mssqlDSN, err := cfg.MssqlDSN()
	if err != nil {
		log.Fatalf("MSSQL 配置错误: %v", err)
	}
	mssqlDB, err := store.InitMssql(mssqlDSN)
	if err != nil {
		log.Fatalf("MSSQL 初始化失败: %v", err)
	}
	if mssqlDB == nil {
		log.Println("提示: 未配置 sqlserver 段（mssql 行集/写回模板会不可用）")
	}

	eng, err := template.NewEngine(cfg.Templates.Dir)
	if err != nil {
		log.Fatalf("模板加载失败: %v", err)
	}
	log.Printf("已加载 %d 个报表模板: %v", len(eng.Codes()), eng.Codes())
	registerTemplates(systemDB, eng)

	svc := service.New(cfg, systemDB, dorisDB, mssqlDB, eng, push.NewPusher(cfg))
	router := httpapi.NewRouter(svc, cfg.Server.Mode, version)

	srv := &http.Server{Addr: cfg.Server.Addr, Handler: router}
	log.Printf("kingdee-rpt %s 监听 %s", version, cfg.Server.Addr)
	if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		log.Fatalf("服务退出: %v", err)
	}
}

// registerTemplates 模板版本落库（审计与追溯）。
func registerTemplates(db *gorm.DB, eng *template.Engine) {
	for _, code := range eng.Codes() {
		def, _ := eng.Get(code)
		t := &model.RptTemplate{
			Code:      def.Metadata.Code,
			Version:   def.Metadata.Version,
			YAML:      string(def.Raw),
			Checksum:  def.Checksum(def.Raw),
			UpdatedAt: time.Now(),
		}
		if err := store.UpsertTemplate(db, t); err != nil {
			log.Printf("模板落库失败 %s: %v", code, err)
		}
	}
}
