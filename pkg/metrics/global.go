package metrics

import (
	"sync"
)

var (
	// Global metrics registry instance
	globalRegistry *Registry
	once           sync.Once
)

// GetRegistry returns the global metrics registry, initializing it if necessary
func GetRegistry() *Registry {
	once.Do(func() {
		globalRegistry = NewRegistry()
	})
	return globalRegistry
}
