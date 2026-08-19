package config

import (
	"fmt"
	"os"
	"strings"

	"gopkg.in/ini.v1"
)

// Config 由 config.ini 加载。ini.v1 的 MapTo 按 section 映射到嵌套 struct。
type Config struct {
	Server    ServerConfig    `ini:"server"`
	System    SystemConfig    `ini:"system"`
	MySQL     LegacyConfig    `ini:"mysql"`
	Doris     DorisConfig     `ini:"doris"`
	SqlServer SqlServerConfig `ini:"sqlserver"`
	SMTP      SMTPConfig      `ini:"smtp"`
	DingTalk  DingTalkConfig  `ini:"dingtalk"`
	Templates TemplatesConfig `ini:"templates"`
}

type ServerConfig struct {
	Addr string `ini:"addr"`
	Mode string `ini:"mode"` // gin mode: debug/release
}

// SystemConfig 系统库（草稿/状态/审计/模板版本）。
// dsn 以 "/" 开头或 ".db" 结尾 → SQLite 文件；否则按 MySQL DSN 解析。
type SystemConfig struct {
	DSN string `ini:"dsn"`
}

type LegacyConfig struct {
	DSN string `ini:"dsn"` // 兼容旧配置
}

type DorisConfig struct {
	DSN string `ini:"dsn"`
}

type SqlServerConfig struct {
	Host   string `ini:"host"`
	Port   int    `ini:"port"`
	User   string `ini:"user"`
	Password string `ini:"password"`
	// PassFile 优先级高于 password：密码含 # 等特殊字符时写文件（0600）引用，
	// 避开 ini 解析把 # 当注释截断的问题。
	PassFile string `ini:"pass_file"`
	Database string `ini:"database"`
	Encrypt  string `ini:"encrypt"` // disable | require | strict
}

// MssqlPassword 解析 MSSQL 密码：pass_file 优先，读文件并去掉尾部换行。
func (c *Config) MssqlPassword() (string, error) {
	if c.SqlServer.PassFile != "" {
		b, err := os.ReadFile(c.SqlServer.PassFile)
		if err != nil {
			return "", fmt.Errorf("read pass_file: %w", err)
		}
		return strings.TrimRight(string(b), "\r\n"), nil
	}
	return c.SqlServer.Password, nil
}

type SMTPConfig struct {
	Host string `ini:"host"`
	Port int    `ini:"port"`
	User string `ini:"user"`
	Pass string `ini:"pass"`
	From string `ini:"from"`
}

type DingTalkConfig struct {
	Webhook string `ini:"webhook"`
	Secret  string `ini:"secret"`
}

type TemplatesConfig struct {
	Dir string `ini:"dir"`
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
	if cfg.Server.Addr == "" {
		cfg.Server.Addr = ":8090"
	}
	if cfg.Templates.Dir == "" {
		cfg.Templates.Dir = "../../templates"
	}
	if cfg.System.DSN == "" {
		cfg.System.DSN = cfg.MySQL.DSN
	}
	return &cfg, nil
}

// SystemIsSQLite 系统库是否为本地 SQLite 文件。
func (c *Config) SystemIsSQLite() bool {
	d := c.System.DSN
	return len(d) > 3 && (d[0] == '/' || len(d) > 3 && d[len(d)-3:] == ".db")
}

// MssqlDSN 构造 go-mssqldb 连接串（密码做 URL 编码）。
func (c *Config) MssqlDSN() (string, error) {
	if c.SqlServer.Host == "" {
		return "", nil
	}
	pwd, err := c.MssqlPassword()
	if err != nil {
		return "", err
	}
	encrypt := c.SqlServer.Encrypt
	if encrypt == "" {
		encrypt = "disable"
	}
	return fmt.Sprintf("sqlserver://%s:%s@%s:%d?database=%s&encrypt=%s&trustservercertificate=true",
		c.SqlServer.User, urlQuote(pwd), c.SqlServer.Host,
		c.SqlServer.Port, urlQuote(c.SqlServer.Database), encrypt), nil
}

func urlQuote(s string) string {
	const hex = "0123456789abcdef"
	var b []byte
	for i := 0; i < len(s); i++ {
		ch := s[i]
		if (ch >= 'A' && ch <= 'Z') || (ch >= 'a' && ch <= 'z') || (ch >= '0' && ch <= '9') ||
			ch == '-' || ch == '_' || ch == '.' || ch == '~' {
			b = append(b, ch)
			continue
		}
		b = append(b, '%', hex[ch>>4], hex[ch&0xF])
	}
	return string(b)
}
