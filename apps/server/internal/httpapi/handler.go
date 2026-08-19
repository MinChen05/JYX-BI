package httpapi

import (
	"io"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"

	"github.com/MinChen05/kingdee-rpt/internal/service"

	"github.com/MinChen05/kingdee-rpt/internal/engine"
)

func (d *deps) respondOK(c *gin.Context, data any) {
	c.JSON(http.StatusOK, gin.H{"ok": true, "data": data})
}

func (d *deps) respondErr(c *gin.Context, err error) {
	if ae, ok2 := err.(*service.AppError); ok2 {
		c.JSON(http.StatusBadRequest, gin.H{"ok": false, "error": ae})
		return
	}
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
