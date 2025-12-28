package config

import (
	"github.com/BurntSushi/toml"
	"github.com/oneblade/utils"
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
	LogLevel string `toml:"log_level" validate:"required,oneof=debug info warn error"`
}

// ServerConfig 服务器配置
type ServerConfig struct {
	Addr    string         `toml:"addr" validate:"required,hostname_port"`
	Timeout utils.Duration `toml:"timeout"`
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
	Addr         string         `toml:"addr"`
	ReadTimeout  utils.Duration `toml:"read_timeout"`
	WriteTimeout utils.Duration `toml:"write_timeout"`
}

type CollectorConfig struct {
	Type    string         `toml:"type" validate:"required,oneof=prometheus pagerduty opensearch"`
	Enabled bool           `toml:"enabled"`
	Options toml.Primitive `toml:"options"`
}
