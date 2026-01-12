package service

import (
	"fmt"
	"log/slog"
	"sync"

	"github.com/oneblade/config"
)

type ServiceMeta struct {
	Name        string
	Description string
}

type ServiceFactory func(meta ServiceMeta, opts interface{}) (Service, error)

var factoryRegistry = struct {
	mu       sync.RWMutex
	services map[ServiceType]ServiceFactory
}{
	services: make(map[ServiceType]ServiceFactory),
}

func RegisterService(serviceType ServiceType, factory ServiceFactory) {
	factoryRegistry.mu.Lock()
	defer factoryRegistry.mu.Unlock()

	if _, exists := factoryRegistry.services[serviceType]; exists {
		slog.Warn("service factory already registered", "type", serviceType)
		return
	}

	factoryRegistry.services[serviceType] = factory
}

func getServiceFactory(serviceType ServiceType) (ServiceFactory, bool) {
	factoryRegistry.mu.RLock()
	defer factoryRegistry.mu.RUnlock()

	factory, ok := factoryRegistry.services[serviceType]
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
	cfg, err := loader.Get()
	if err != nil {
		return fmt.Errorf("failed to get config: %w", err)
	}

	// Note: loader.Get() already filters out disabled services
	enabledCount := len(cfg.Services)
	if enabledCount == 0 {
		return nil
	}

	var (
		initErrors  []error
		newServices = make(map[string]Service)
	)

	// Sequential Initialization
	for name, serviceCfg := range cfg.Services {
		svc, err := r.initService(loader, name, serviceCfg)
		if err != nil {
			slog.Error("service initialization failed", "service", name, "error", err)
			initErrors = append(initErrors, fmt.Errorf("service %s: %w", name, err))
			continue
		}
		newServices[name] = svc
		slog.Info("service initialized", "service", name, "type", serviceCfg.Type)
	}

	// Handle results
	successCount := len(newServices)
	if successCount == 0 && enabledCount > 0 {
		return fmt.Errorf("all %d enabled service(s) failed to initialize: %v", enabledCount, initErrors)
	}

	if len(initErrors) > 0 {
		slog.Warn("some services failed to initialize",
			"failed_count", len(initErrors),
			"success_count", successCount,
		)
	}

	// Update registry
	r.mu.Lock()
	defer r.mu.Unlock()
	for name, s := range newServices {
		r.services[name] = s
	}

	return nil
}

// initService handles the initialization logic for a single service
func (r *Registry) initService(loader *config.Loader, name string, serviceCfg config.ServiceConfig) (Service, error) {
	// Get primitive options
	primitive, meta, err := loader.GetServiceOptions(name)
	if err != nil {
		return nil, fmt.Errorf("get options: %w", err)
	}

	// Get parser
	serviceType := ServiceType(serviceCfg.Type)
	parser, ok := GetOptionsParser(serviceType)
	if !ok {
		return nil, fmt.Errorf("no parser registered for type %s", serviceType)
	}

	// Parse options
	opts, err := parser(meta, primitive)
	if err != nil {
		return nil, fmt.Errorf("parse options: %w", err)
	}

	// Create service
	serviceMeta := ServiceMeta{
		Name:        name,
		Description: serviceCfg.Description,
	}

	factory, ok := getServiceFactory(serviceType)
	if !ok {
		return nil, fmt.Errorf("unknown service type %s", serviceType)
	}

	service, err := factory(serviceMeta, opts)
	if err != nil {
		return nil, fmt.Errorf("create service: %w", err)
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
		return fmt.Errorf("close errors: %v", errs)
	}
	return nil
}
