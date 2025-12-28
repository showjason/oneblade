package utils

import "time"

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
