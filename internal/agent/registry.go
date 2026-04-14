package agent

import (
	"fmt"
	"sync"
)

var (
	mu       sync.RWMutex
	registry = map[string]Agent{}
)

// Register adds an agent to the global registry.
func Register(a Agent) {
	mu.Lock()
	registry[a.Name()] = a
	mu.Unlock()
}

// Unregister removes an agent from the global registry.
// Primarily used in tests to restore registry state.
func Unregister(name string) {
	mu.Lock()
	delete(registry, name)
	mu.Unlock()
}

// Get returns a registered agent by name.
func Get(name string) (Agent, error) {
	mu.RLock()
	a, ok := registry[name]
	mu.RUnlock()
	if !ok {
		return nil, fmt.Errorf("unknown agent %q — registered agents: %v", name, List())
	}
	return a, nil
}

// List returns all registered agent names.
func List() []string {
	mu.RLock()
	defer mu.RUnlock()
	names := make([]string, 0, len(registry))
	for n := range registry {
		names = append(names, n)
	}
	return names
}
