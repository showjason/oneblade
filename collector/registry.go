package collector

import (
	"fmt"
	"log"
	"sync"

	"github.com/oneblade/config"
)

type CollectorFactory func(opts interface{}) (Collector, error)

var collectorRegistry = struct {
	mu         sync.RWMutex
	collectors map[CollectorType]CollectorFactory
}{
	collectors: make(map[CollectorType]CollectorFactory),
}

func RegisterCollector(collectorType CollectorType, collector CollectorFactory) {
	collectorRegistry.mu.Lock()
	defer collectorRegistry.mu.Unlock()

	if _, exists := collectorRegistry.collectors[collectorType]; exists {
		log.Printf("[collector] warning: collector instance for %s already registered", collectorType)
		return
	}

	collectorRegistry.collectors[collectorType] = collector
}

func getCollector(collectorType CollectorType) (CollectorFactory, bool) {
	collectorRegistry.mu.RLock()
	defer collectorRegistry.mu.RUnlock()

	factory, ok := collectorRegistry.collectors[collectorType]
	return factory, ok
}

type Registry struct {
	mu         sync.RWMutex
	collectors map[CollectorType]Collector
	loader     *config.Loader
}

func NewRegistry() *Registry {
	return &Registry{
		collectors: make(map[CollectorType]Collector),
	}
}

func (r *Registry) InitFromConfig(loader *config.Loader) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	cfg := loader.Get()
	if cfg == nil {
		return fmt.Errorf("config not loaded")
	}

	r.loader = loader

	for name, collectorCfg := range cfg.Collectors {
		if !collectorCfg.Enabled {
			log.Printf("[collector] %s is disabled, skipping", name)
			continue
		}

		opts, err := loader.ParseCollectorOptions(name, collectorCfg.Type, collectorCfg.Options)
		if err != nil {
			return fmt.Errorf("parse options for %s: %w", name, err)
		}

		collectorType := CollectorType(collectorCfg.Type)
		collector, err := r.createCollector(collectorType, opts)
		if err != nil {
			return fmt.Errorf("create collector %s: %w", name, err)
		}

		r.collectors[collector.Name()] = collector
		log.Printf("[collector] initialized %s", name)
	}

	return nil
}

func (r *Registry) createCollector(collectorType CollectorType, opts interface{}) (Collector, error) {
	collector, ok := getCollector(collectorType)
	if !ok {
		return nil, fmt.Errorf("unknown collector type: %s (no factory registered)", collectorType)
	}

	collectorInstance, err := collector(opts)
	if err != nil {
		return nil, fmt.Errorf("create collector %s: %w", collectorType, err)
	}

	return collectorInstance, nil
}

// All 获取所有已注册的采集器
func (r *Registry) All() []Collector {
	r.mu.RLock()
	defer r.mu.RUnlock()

	result := make([]Collector, 0, len(r.collectors))
	for _, c := range r.collectors {
		result = append(result, c)
	}
	return result
}

// Close 关闭所有采集器
func (r *Registry) Close() error {
	r.mu.Lock()
	defer r.mu.Unlock()

	var errs []error
	for name, c := range r.collectors {
		if err := c.Close(); err != nil {
			errs = append(errs, fmt.Errorf("close %s: %w", name, err))
		}
	}

	if len(errs) > 0 {
		return fmt.Errorf("errors: %v", errs)
	}
	return nil
}
