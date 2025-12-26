package config

import (
	"time"

	"github.com/BurntSushi/toml"
)

// Config 根配置结构
type Config struct {
	App        AppConfig                  `toml:"app" validate:"required"`
	Server     ServerConfig               `toml:"server" validate:"required"`
	Data       DataConfig                 `toml:"data"`
	Collectors map[string]CollectorConfig `toml:"collectors" validate:"dive"`
}

// AppConfig 应用全局配置
type AppConfig struct {
	Name     string `toml:"name" validate:"required,alpha"`
	Env      string `toml:"env" validate:"required,oneof=dev prod"`
	Debug    bool   `toml:"debug"`
	LogLevel string `toml:"log_level" validate:"required,oneof=debug info warn error"`
}

// ServerConfig 服务器配置
type ServerConfig struct {
	Addr    string   `toml:"addr" validate:"required,hostname_port"`
	Timeout Duration `toml:"timeout"`
}

// DataConfig 数据存储配置
type DataConfig struct {
	Database DatabaseConfig `toml:"database"`
	Redis    RedisConfig    `toml:"redis"`
}

// DatabaseConfig 数据库配置
type DatabaseConfig struct {
	Driver string `toml:"driver"`
	Source string `toml:"source"`
}

// RedisConfig Redis配置
type RedisConfig struct {
	Addr         string   `toml:"addr"`
	ReadTimeout  Duration `toml:"read_timeout"`
	WriteTimeout Duration `toml:"write_timeout"`
}

type CollectorConfig struct {
	Type    string         `toml:"type" validate:"required,oneof=prometheus pagerduty opensearch"`
	Enabled bool           `toml:"enabled"`
	Options toml.Primitive `toml:"options"`
}

// Duration 支持 TOML 字符串解析的 time.Duration 包装
type Duration struct {
	time.Duration
}

// UnmarshalText 实现 encoding.TextUnmarshaler 接口
func (d *Duration) UnmarshalText(text []byte) error {
	var err error
	d.Duration, err = time.ParseDuration(string(text))
	return err
}

// MarshalText 实现 encoding.TextMarshaler 接口
func (d Duration) MarshalText() ([]byte, error) {
	return []byte(d.Duration.String()), nil
}

// PrometheusOptions Prometheus 采集器选项
type PrometheusOptions struct {
	Address string   `toml:"address" validate:"required,url"`
	Timeout Duration `toml:"timeout"`
}

// PagerDutyOptions PagerDuty 采集器选项
type PagerDutyOptions struct {
	APIKey string `toml:"api_key" validate:"required"`
}

// OpenSearchOptions OpenSearch 采集器选项
type OpenSearchOptions struct {
	Addresses []string `toml:"addresses" validate:"required,min=1,dive,url"`
	Username  string   `toml:"username"`
	Password  string   `toml:"password"`
	Index     string   `toml:"index" validate:"required"`
}
