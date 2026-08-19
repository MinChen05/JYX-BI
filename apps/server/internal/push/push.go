package push

import (
	"fmt"
	"log"

	"github.com/MinChen05/JYX-BI/internal/config"
	"github.com/MinChen05/JYX-BI/internal/template"
)

// Pusher 提交后推送（邮件/钉钉）。
type Pusher struct {
	Cfg *config.Config
}

func NewPusher(cfg *config.Config) *Pusher { return &Pusher{Cfg: cfg} }

// Push 按模板 push 配置执行（调用方负责放到 goroutine）。
func (p *Pusher) Push(def *template.ReportDef, params map[string]string, xlsx []byte) {
	for _, pd := range def.Spec.Push {
		if pd.On != "submit" {
			continue
		}
		var err error
		switch pd.Channel {
		case "email":
			err = sendEmail(p.Cfg, pd.To, def.Metadata.Name, params, xlsx)
		case "dingtalk":
			err = sendDingTalk(p.Cfg, def, params)
		default:
			err = fmt.Errorf("未知推送通道 %s", pd.Channel)
		}
		if err != nil {
			log.Printf("[push] %s/%s -> %s(%s) 失败: %v", def.Metadata.Code, paramsString(params), pd.Channel, pd.To, err)
		}
	}
}

func paramsString(params map[string]string) string {
	if len(params) == 0 {
		return "-"
	}
	first := ""
	for k, v := range params {
		first = k + "=" + v
		break
	}
	return first
}
