package config

import (
	"time"

	"github.com/BurntSushi/toml"
)

// Config 根配置结构
type Config struct {
	Server   ServerConfig             `toml:"server" validate:"required"`
	Data     DataConfig               `toml:"data"`
	Services map[string]ServiceConfig `toml:"services" validate:"required,dive"`
}

// ServerConfig 服务器配置
type ServerConfig struct {
	Addr    string        `toml:"addr" validate:"required,hostname_port"`
	Timeout time.Duration `toml:"timeout"`
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
	Addr         string        `toml:"addr"`
	ReadTimeout  time.Duration `toml:"read_timeout"`
	WriteTimeout time.Duration `toml:"write_timeout"`
}

type ServiceConfig struct {
	Type    string         `toml:"type" validate:"required,oneof=prometheus pagerduty opensearch"`
	Enabled bool           `toml:"enabled"`
	Options toml.Primitive `toml:"options"`
}
