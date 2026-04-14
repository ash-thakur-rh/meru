package node

import (
	"fmt"
	"sync"
)

const LocalNodeName = "local"

var (
	mu       sync.RWMutex
	registry = map[string]Node{}
)

// Register adds a node to the in-memory registry.
// Call this at startup for the local node, and whenever a remote node is added.
func Register(n Node) {
	mu.Lock()
	registry[n.Name()] = n
	mu.Unlock()
}

// Unregister removes a node and closes its connection.
func Unregister(name string) error {
	mu.Lock()
	n, ok := registry[name]
	if ok {
		delete(registry, name)
	}
	mu.Unlock()
	if !ok {
		return fmt.Errorf("node %q not found", name)
	}
	return n.Close()
}

// Get returns a registered node by name.
func Get(name string) (Node, error) {
	if name == "" {
		name = LocalNodeName
	}
	mu.RLock()
	n, ok := registry[name]
	mu.RUnlock()
	if !ok {
		return nil, fmt.Errorf("node %q not registered — add it with: meru nodes add", name)
	}
	return n, nil
}

// List returns all registered node names.
func List() []string {
	mu.RLock()
	defer mu.RUnlock()
	names := make([]string, 0, len(registry))
	for n := range registry {
		names = append(names, n)
	}
	return names
}
