package store

import (
	"fmt"
	"sync"
)

// DefaultFactory is the default store factory
type DefaultFactory struct {
	builders map[string]Builder
	mu       sync.RWMutex
}

// Builder is a function that creates a store from config
type Builder func(config Config) (Store, error)

var (
	// Global factory instance
	globalFactory = &DefaultFactory{
		builders: make(map[string]Builder),
	}
)

func init() {
	// Register built-in store types
	RegisterStoreType("sqlite", func(config Config) (Store, error) {
		return NewSQLiteStore(config)
	})
	RegisterStoreType("postgresql", func(config Config) (Store, error) {
		return NewPostgreSQLStore(config)
	})
	RegisterStoreType("postgres", func(config Config) (Store, error) {
		return NewPostgreSQLStore(config)
	})
}

// RegisterStoreType registers a new store type with the global factory
func RegisterStoreType(storeType string, builder Builder) {
	globalFactory.RegisterStoreType(storeType, builder)
}

// CreateStore creates a store using the global factory
func CreateStore(config Config) (Store, error) {
	return globalFactory.CreateStore(config)
}

// SupportedTypes returns supported store types from the global factory
func SupportedTypes() []string {
	return globalFactory.SupportedTypes()
}

// RegisterStoreType registers a new store type
func (f *DefaultFactory) RegisterStoreType(storeType string, builder Builder) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.builders[storeType] = builder
}

// CreateStore creates a store based on the configuration
func (f *DefaultFactory) CreateStore(config Config) (Store, error) {
	f.mu.RLock()
	builder, exists := f.builders[config.Type]
	f.mu.RUnlock()

	if !exists {
		return nil, fmt.Errorf("unsupported store type: %s (supported: %v)", config.Type, f.SupportedTypes())
	}

	return builder(config)
}

// SupportedTypes returns a list of supported store types
func (f *DefaultFactory) SupportedTypes() []string {
	f.mu.RLock()
	defer f.mu.RUnlock()

	types := make([]string, 0, len(f.builders))
	for storeType := range f.builders {
		types = append(types, storeType)
	}
	return types
}

// Wrapper provides additional functionality around a base store
type Wrapper struct {
	Store
	name   string
	config Config
}

// Name returns the store name
func (w *Wrapper) Name() string {
	return w.name
}

// Config returns the store configuration
func (w *Wrapper) Config() Config {
	return w.config
}

// Type returns the store type
func (w *Wrapper) Type() string {
	return w.config.Type
}
