package push

import (
	"bytes"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"time"

	"github.com/MinChen05/kingdee-rpt/internal/config"
	"github.com/MinChen05/kingdee-rpt/internal/template"
)

// sendDingTalk 群机器人 markdown 通知（支持加签）。
func sendDingTalk(cfg *config.Config, def *template.ReportDef, params map[string]string) error {
	if cfg.DingWebhook == "" {
		return fmt.Errorf("dingtalk webhook 未配置")
	}
	u := cfg.DingWebhook
	if cfg.DingSecret != "" {
		ts := fmt.Sprintf("%d", time.Now().UnixMilli())
		mac := hmac.New(sha256.New, []byte(cfg.DingSecret))
		mac.Write([]byte(ts + "\n" + cfg.DingSecret))
		sign := url.QueryEscape(base64.StdEncoding.EncodeToString(mac.Sum(nil)))
		sep := "&"
		if !bytes.Contains([]byte(u), []byte("?")) {
			sep = "?"
		}
		u = fmt.Sprintf("%s%stimestamp=%s&sign=%s", u, sep, ts, sign)
	}
	title := fmt.Sprintf("【报表提交】%s", def.Metadata.Name)
	text := fmt.Sprintf("### %s\n\n已提交，请到系统查看。\n\n%s", def.Metadata.Name, paramsString(params))
	payload := map[string]any{
		"msgtype": "markdown",
		"markdown": map[string]string{"title": title, "text": text},
	}
	body, _ := json.Marshal(payload)
	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Post(u, "application/json", bytes.NewReader(body))
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		return fmt.Errorf("dingtalk http %d", resp.StatusCode)
	}
	return nil
}
