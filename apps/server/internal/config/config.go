package config

import (
	"fmt"

	"gopkg.in/ini.v1"
)

// Config 由 config.ini 加载，结构扁平化便于 ini MapTo。
type Config struct {
	ServerAddr string `ini:"server:addr"`
	ServerMode string `ini:"server:mode"` // gin mode: debug/release

	MySQLDSN string `ini:"mysql:dsn"`
	DorisDSN string `ini:"doris:dsn"`

	SMTPHost string `ini:"smtp:host"`
	SMTPPort int    `ini:"smtp:port"`
	SMTPUser string `ini:"smtp:user"`
	SMTPPass string `ini:"smtp:pass"`
	SMTPFrom string `ini:"smtp:from"`

	DingWebhook string `ini:"dingtalk:webhook"`
	DingSecret  string `ini:"dingtalk:secret"`

	TplDir string `ini:"templates:dir"`
}

func Load(path string) (*Config, error) {
	f, err := ini.Load(path)
	if err != nil {
		return nil, fmt.Errorf("load config %s: %w", path, err)
	}
	var cfg Config
	if err := f.MapTo(&cfg); err != nil {
		return nil, fmt.Errorf("map config: %w", err)
	}
	if cfg.ServerAddr == "" {
		cfg.ServerAddr = ":8090"
	}
	if cfg.TplDir == "" {
		cfg.TplDir = "../../templates"
	}
	return &cfg, nil
}
