package collector

import (
	"fmt"
	"sync"

	"github.com/BurntSushi/toml"
)

// OptionsParser 解析器函数类型
type OptionsParser func(meta *toml.MetaData, primitive toml.Primitive) (interface{}, error)

var optionsParsers = struct {
	mu      sync.RWMutex
	parsers map[CollectorType]OptionsParser
}{
	parsers: make(map[CollectorType]OptionsParser),
}

// RegisterOptionsParser 注册 Options 解析器
func RegisterOptionsParser(collectorType CollectorType, parser OptionsParser) {
	optionsParsers.mu.Lock()
	defer optionsParsers.mu.Unlock()
	optionsParsers.parsers[collectorType] = parser
}

// GetOptionsParser 获取解析器
func GetOptionsParser(collectorType CollectorType) (OptionsParser, bool) {
	optionsParsers.mu.RLock()
	defer optionsParsers.mu.RUnlock()
	parser, ok := optionsParsers.parsers[collectorType]
	return parser, ok
}

// ParseOptions 泛型函数：解析 TOML Primitive 到具体的配置结构
func ParseOptions[T any](meta *toml.MetaData, primitive toml.Primitive, typeName CollectorType) (*T, error) {
	var opts T
	if err := meta.PrimitiveDecode(primitive, &opts); err != nil {
		return nil, fmt.Errorf("decode %s options: %w", typeName, err)
	}
	return &opts, nil
}
