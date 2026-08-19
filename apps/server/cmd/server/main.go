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
	if cfg.MySQLDSN == "" {
		log.Fatal("config.ini 缺少 mysql.dsn")
	}
	mysqlDB, err := store.InitMySQL(cfg.MySQLDSN)
	if err != nil {
		log.Fatalf("MySQL 初始化失败: %v", err)
	}
	dorisDB, err := store.InitDoris(cfg.DorisDSN)
	if err != nil {
		log.Fatalf("Doris 初始化失败: %v", err)
	}
	if dorisDB == nil {
		log.Println("警告: 未配置 doris.dsn，SQL 行集模板不可用")
	}

	eng, err := template.NewEngine(cfg.TplDir)
	if err != nil {
		log.Fatalf("模板加载失败: %v", err)
	}
	log.Printf("已加载 %d 个报表模板: %v", len(eng.Codes()), eng.Codes())
	registerTemplates(mysqlDB, eng)

	svc := service.New(cfg, mysqlDB, dorisDB, eng, push.NewPusher(cfg))
	router := httpapi.NewRouter(svc, cfg.ServerMode, version)

	srv := &http.Server{Addr: cfg.ServerAddr, Handler: router}
	log.Printf("kingdee-rpt %s 监听 %s", version, cfg.ServerAddr)
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
