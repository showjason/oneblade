package service

import (
	"fmt"
	"log"
	"sync"

	"github.com/oneblade/config"
)

type ServiceMeta struct {
	Name        string
	Description string
}

type ServiceFactory func(meta ServiceMeta, opts interface{}) (Service, error)

var serviceRegistry = struct {
	mu       sync.RWMutex
	services map[ServiceType]ServiceFactory
}{
	services: make(map[ServiceType]ServiceFactory),
}

func RegisterService(serviceType ServiceType, factory ServiceFactory) {
	serviceRegistry.mu.Lock()
	defer serviceRegistry.mu.Unlock()

	if _, exists := serviceRegistry.services[serviceType]; exists {
		log.Printf("[service] warning: service factory for %s already registered", serviceType)
		return
	}

	serviceRegistry.services[serviceType] = factory
}

func getServiceFactory(serviceType ServiceType) (ServiceFactory, bool) {
	serviceRegistry.mu.RLock()
	defer serviceRegistry.mu.RUnlock()

	factory, ok := serviceRegistry.services[serviceType]
	return factory, ok
}

type Registry struct {
	mu       sync.RWMutex
	services map[string]Service
}

func NewRegistry() *Registry {
	return &Registry{
		services: make(map[string]Service),
	}
}

func (r *Registry) InitFromConfig(loader *config.Loader) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	cfg := loader.Get()
	if cfg == nil {
		return fmt.Errorf("config not loaded")
	}

	for name, serviceCfg := range cfg.Services {
		if !serviceCfg.Enabled {
			log.Printf("[service] %s is disabled, skipping", name)
			continue
		}

		// 获取原始配置数据
		primitive, meta, err := loader.GetServiceOptions(name)
		if err != nil {
			return fmt.Errorf("get options for %s: %w", name, err)
		}

		// 获取解析器
		serviceType := ServiceType(serviceCfg.Type)
		parser, ok := GetOptionsParser(serviceType)
		if !ok {
			return fmt.Errorf("no parser registered for service type: %s", serviceType)
		}

		// 统一调用解析器
		opts, err := parser(meta, primitive)
		if err != nil {
			return fmt.Errorf("parse options for %s: %w", name, err)
		}

		// 创建 service
		serviceMeta := ServiceMeta{
			Name:        name,
			Description: serviceCfg.Description,
		}
		service, err := r.createService(serviceType, serviceMeta, opts)
		if err != nil {
			return fmt.Errorf("create service %s: %w", name, err)
		}

		r.services[service.Name()] = service
		log.Printf("[service] initialized %s", name)
	}

	return nil
}

func (r *Registry) createService(serviceType ServiceType, meta ServiceMeta, opts interface{}) (Service, error) {
	factory, ok := getServiceFactory(serviceType)
	if !ok {
		return nil, fmt.Errorf("unknown service type: %s (no factory registered)", serviceType)
	}

	service, err := factory(meta, opts)
	if err != nil {
		return nil, fmt.Errorf("create service %s: %w", serviceType, err)
	}

	return service, nil
}

// All 获取所有已注册的服务
func (r *Registry) All() []Service {
	r.mu.RLock()
	defer r.mu.RUnlock()

	result := make([]Service, 0, len(r.services))
	for _, s := range r.services {
		result = append(result, s)
	}
	return result
}

// Close 关闭所有服务
func (r *Registry) Close() error {
	r.mu.Lock()
	defer r.mu.Unlock()

	var errs []error
	for name, s := range r.services {
		if err := s.Close(); err != nil {
			errs = append(errs, fmt.Errorf("close %s: %w", name, err))
		}
	}

	if len(errs) > 0 {
		return fmt.Errorf("errors: %v", errs)
	}
	return nil
}
