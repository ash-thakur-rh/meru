// Package node abstracts where agent sessions run — locally or on a remote machine.
//
// Every spawn request is routed through a Node. The local node delegates
// directly to the agent adapters; a gRPC node proxies the call to a
// meru-node daemon running on a remote machine.
package node

import (
	"context"

	"github.com/ash-thakur-rh/meru/internal/agent"
)

// Info holds the persisted configuration for a node.
type Info struct {
	Name    string   // unique identifier, e.g. "local", "gpu-box"
	Addr    string   // gRPC address (host:port); empty for local node
	Token   string   // bearer token for authentication
	TLS     bool     // whether to use TLS
	Version string   // reported by Ping
	Agents  []string // agents available on this node
}

// Node is the interface that both the local node and remote gRPC nodes implement.
type Node interface {
	// Name returns the node's unique identifier.
	Name() string

	// Ping checks connectivity and returns the node's reported capabilities.
	Ping(ctx context.Context) (*Info, error)

	// Spawn creates a new agent session on this node.
	// sessionID is provided by the control plane so it can reconnect after restart.
	Spawn(ctx context.Context, sessionID string, agentName string, cfg agent.SpawnConfig) (agent.Session, error)

	// ListDir returns the contents of a directory on this node's filesystem.
	// An empty path resolves to the user's home directory.
	ListDir(ctx context.Context, path string) (*DirListing, error)

	// GitClone clones a git repository on this node.
	// dest is the target directory; if empty, a path under ~/meru-workspaces is generated.
	// username and password are used for HTTPS authentication; leave empty for public repos or SSH.
	GitClone(ctx context.Context, url, dest, username, password string) (string, error)

	// Close releases any persistent connections (no-op for local node).
	Close() error
}

// DirEntry is a single filesystem entry returned by ListDir.
type DirEntry struct {
	Name  string `json:"name"`
	Path  string `json:"path"`
	IsDir bool   `json:"is_dir"`
}

// DirListing is the result of a ListDir call.
type DirListing struct {
	Path    string     `json:"path"`
	Parent  string     `json:"parent"`
	Entries []DirEntry `json:"entries"`
}
