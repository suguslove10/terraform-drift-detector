package provider

import (
	"fmt"
	"sync"
)

var (
	registry = make(map[string]Provider)
	mu       sync.RWMutex
)

// Register adds a provider to the global registry.
func Register(name string, p Provider) {
	mu.Lock()
	defer mu.Unlock()
	registry[name] = p
}

// Get retrieves a provider from the registry by name.
func Get(name string) (Provider, error) {
	mu.RLock()
	defer mu.RUnlock()
	p, ok := registry[name]
	if !ok {
		available := make([]string, 0, len(registry))
		for k := range registry {
			available = append(available, k)
		}
		return nil, fmt.Errorf("provider %q not found (available: %v)", name, available)
	}
	return p, nil
}

// List returns all registered provider names.
func List() []string {
	mu.RLock()
	defer mu.RUnlock()
	names := make([]string, 0, len(registry))
	for k := range registry {
		names = append(names, k)
	}
	return names
}
