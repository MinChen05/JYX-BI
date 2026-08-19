package httpapi

import (
	"github.com/gin-gonic/gin"

	"github.com/MinChen05/kingdee-rpt/internal/service"
)

type deps struct {
	svc *service.Service
}

// NewRouter 构建全部路由。
func NewRouter(svc *service.Service, ginMode, version string) *gin.Engine {
	gin.SetMode(ginMode)
	r := gin.New()
	r.Use(gin.Recovery())

	r.GET("/api/health", func(c *gin.Context) {
		c.JSON(200, gin.H{"ok": true, "version": version})
	})

	d := &deps{svc: svc}
	api := r.Group("/api/reports")
	{
		api.GET("", d.listReports)
		api.GET("/:code/grid", d.getGrid)
		api.PUT("/:code/draft", d.saveDraft)
		api.POST("/:code/validate", d.validate)
		api.POST("/:code/submit", d.submit)
		api.POST("/:code/withdraw", d.withdraw)
		api.GET("/:code/export.xlsx", d.exportXlsx)
		api.POST("/:code/import", d.importFile)
		api.POST("/:code/import/:id/confirm", d.confirmImport)
		api.GET("/:code/selfcheck", d.selfcheck)
		api.GET("/:code/submissions", d.submissions)
	}
	return r
}
