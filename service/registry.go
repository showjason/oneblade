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
	cfg, err := loader.Get()
	if err != nil {
		return fmt.Errorf("failed to get config: %w", err)
	}

	// 1. Initialize services concurrently
	// Note: loader.Get() already filters out disabled services
	enabledCount := len(cfg.Services)
	if enabledCount == 0 {
		return nil
	}

	type result struct {
		name    string
		service Service
		err     error
	}

	resultCh := make(chan result, enabledCount)
	var wg sync.WaitGroup

	for name, serviceCfg := range cfg.Services {
		wg.Add(1)
		go func(n string, sCfg config.ServiceConfig) {
			defer wg.Done()
			svc, err := r.initService(loader, n, sCfg)
			resultCh <- result{name: n, service: svc, err: err}
		}(name, serviceCfg)
	}

	// Close channel when all goroutines are done
	go func() {
		wg.Wait()
		close(resultCh)
	}()

	// 3. Collect results
	var (
		initErrors  []error
		newServices = make(map[string]Service)
	)

	for res := range resultCh {
		if res.err != nil {
			initErrors = append(initErrors, res.err)
		} else {
			newServices[res.name] = res.service
			log.Printf("[service] initialized %s", res.name)
		}
	}

	// 4. Handle results
	successCount := len(newServices)
	if successCount == 0 && enabledCount > 0 {
		return fmt.Errorf("all %d enabled service(s) failed to initialize: %v", enabledCount, initErrors)
	}

	if len(initErrors) > 0 {
		log.Printf("[service] warning: %d service(s) failed to initialize, %d succeeded", len(initErrors), successCount)
		for _, err := range initErrors {
			log.Printf("  - %v", err)
		}
	}

	// 5. Update registry
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
		return nil, fmt.Errorf("get options for %s: %w", name, err)
	}

	// Get parser
	serviceType := ServiceType(serviceCfg.Type)
	parser, ok := GetOptionsParser(serviceType)
	if !ok {
		return nil, fmt.Errorf("no parser registered for service type %s (service: %s)", serviceType, name)
	}

	// Parse options
	opts, err := parser(meta, primitive)
	if err != nil {
		return nil, fmt.Errorf("parse options for %s: %w", name, err)
	}

	// Create service
	serviceMeta := ServiceMeta{
		Name:        name,
		Description: serviceCfg.Description,
	}

	service, err := r.createService(serviceType, serviceMeta, opts)
	if err != nil {
		return nil, fmt.Errorf("create service %s: %w", name, err)
	}

	return service, nil
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
