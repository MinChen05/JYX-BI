package httpapi

import (
	"io"
	"log"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"

	"github.com/MinChen05/JYX-BI/internal/service"

	"github.com/MinChen05/JYX-BI/internal/engine"
	"github.com/MinChen05/JYX-BI/internal/template"
)

func (d *deps) respondOK(c *gin.Context, data any) {
	c.JSON(http.StatusOK, gin.H{"ok": true, "data": data})
}

func (d *deps) respondErr(c *gin.Context, err error) {
	if ae, ok2 := err.(*service.AppError); ok2 {
		log.Printf("[api-error] %s %s code=%s message=%s", c.Request.Method, c.Request.URL.Path, ae.Code, ae.Message)
		c.JSON(http.StatusBadRequest, gin.H{"ok": false, "error": ae})
		return
	}
	log.Printf("[api-error] %s %s INTERNAL: %v", c.Request.Method, c.Request.URL.Path, err)
	c.JSON(http.StatusInternalServerError, gin.H{"ok": false, "error": gin.H{"code": "INTERNAL", "message": err.Error()}})
}

// paramsFromQuery 从 query 收集模板参数。
func paramsFromQuery(c *gin.Context) map[string]string {
	out := map[string]string{}
	for k, vs := range c.Request.URL.Query() {
		if len(vs) > 0 && vs[0] != "" {
			out[k] = vs[0]
		}
	}
	return out
}

func (d *deps) listReports(c *gin.Context) {
	out, err := d.svc.ListReports()
	if err != nil {
		d.respondErr(c, err)
		return
	}
	d.respondOK(c, out)
}

func (d *deps) getGrid(c *gin.Context) {
	spec, err := d.svc.GetGrid(c.Param("code"), paramsFromQuery(c))
	if err != nil {
		d.respondErr(c, err)
		return
	}
	d.respondOK(c, spec)
}

func (d *deps) saveDraft(c *gin.Context) {
	var req engine.DraftRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		d.respondErr(c, err)
		return
	}
	info, err := d.svc.SaveDraft(c.Param("code"), paramsFromQuery(c), &req)
	if err != nil {
		d.respondErr(c, err)
		return
	}
	d.respondOK(c, info)
}

func (d *deps) validate(c *gin.Context) {
	var req engine.DraftRequest
	_ = c.ShouldBindJSON(&req)
	issues, err := d.svc.ValidateGrid(c.Param("code"), paramsFromQuery(c), req.Rows)
	if err != nil {
		d.respondErr(c, err)
		return
	}
	d.respondOK(c, gin.H{"ok": len(issues) == 0, "issues": issues})
}

func (d *deps) submit(c *gin.Context) {
	var req engine.DraftRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		d.respondErr(c, err)
		return
	}
	info, err := d.svc.Submit(c.Param("code"), paramsFromQuery(c), &req)
	if err != nil {
		d.respondErr(c, err)
		return
	}
	d.respondOK(c, info)
}

func (d *deps) withdraw(c *gin.Context) {
	info, err := d.svc.Withdraw(c.Param("code"), paramsFromQuery(c))
	if err != nil {
		d.respondErr(c, err)
		return
	}
	d.respondOK(c, info)
}

func (d *deps) exportXlsx(c *gin.Context) {
	data, err := d.svc.ExportBytes(c.Param("code"), paramsFromQuery(c))
	if err != nil {
		d.respondErr(c, err)
		return
	}
	c.Header("Content-Type", "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet")
	c.Header("Content-Disposition", "attachment; filename="+c.Param("code")+".xlsx")
	c.Data(http.StatusOK, "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet", data)
}

func (d *deps) importFile(c *gin.Context) {
	file, err := c.FormFile("file")
	if err != nil {
		d.respondErr(c, err)
		return
	}
	f, err := file.Open()
	if err != nil {
		d.respondErr(c, err)
		return
	}
	defer f.Close()
	buf, err := io.ReadAll(f)
	if err != nil {
		d.respondErr(c, err)
		return
	}
	report, err := d.svc.ImportFile(c.Param("code"), paramsFromQuery(c), file.Filename, buf)
	if err != nil {
		d.respondErr(c, err)
		return
	}
	d.respondOK(c, report)
}

func (d *deps) confirmImport(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		d.respondErr(c, err)
		return
	}
	if err := d.svc.ConfirmImport(c.Param("code"), paramsFromQuery(c), id); err != nil {
		d.respondErr(c, err)
		return
	}
	d.respondOK(c, gin.H{"confirmed": true})
}

func (d *deps) selfcheck(c *gin.Context) {
	out, err := d.svc.SelfCheck(c.Param("code"), paramsFromQuery(c))
	if err != nil {
		d.respondErr(c, err)
		return
	}
	d.respondOK(c, out)
}

func (d *deps) submissions(c *gin.Context) {
	out, err := d.svc.Submissions(c.Param("code"), paramsFromQuery(c))
	if err != nil {
		d.respondErr(c, err)
		return
	}
	d.respondOK(c, out)
}

// ===== 设计器（模板管理 + SQL 预览） =====

func (d *deps) listTemplates(c *gin.Context) {
	d.respondOK(c, d.svc.ListTemplates())
}

func (d *deps) getTemplate(c *gin.Context) {
	def, raw, err := d.svc.GetTemplate(c.Param("code"))
	if err != nil {
		d.respondErr(c, err)
		return
	}
	d.respondOK(c, gin.H{"def": def, "raw": raw})
}

func (d *deps) saveTemplate(c *gin.Context) {
	var req struct {
		Code string `json:"code"`
		YAML string `json:"yaml"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		d.respondErr(c, err)
		return
	}
	if err := d.svc.SaveTemplate(req.Code, []byte(req.YAML)); err != nil {
		d.respondErr(c, err)
		return
	}
	d.respondOK(c, gin.H{"saved": req.Code})
}

func (d *deps) deleteTemplate(c *gin.Context) {
	if err := d.svc.DeleteTemplate(c.Param("code")); err != nil {
		d.respondErr(c, err)
		return
	}
	d.respondOK(c, gin.H{"deleted": c.Param("code")})
}

func (d *deps) sqlPreview(c *gin.Context) {
	var req struct {
		Source    string             `json:"source"`
		SQL       string             `json:"sql"`
		ParamsDef []template.ParamDef `json:"params_def"`
		Values    map[string]string  `json:"values"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		d.respondErr(c, err)
		return
	}
	out, err := d.svc.SqlPreview(req.Source, req.SQL, req.ParamsDef, req.Values)
	if err != nil {
		d.respondErr(c, err)
		return
	}
	d.respondOK(c, out)
}

func (d *deps) reloadTemplates(c *gin.Context) {
	if err := d.svc.ReloadTemplates(); err != nil {
		d.respondErr(c, err)
		return
	}
	d.respondOK(c, gin.H{"reloaded": true})
}
