// Package agent defines the interfaces, types, and registry for AI coding
// agent adapters used by Conductor.
package agent

import (
	"context"
	"io"
	"time"
)

// EventType classifies a streaming event from an agent.
type EventType string

const (
	EventText       EventType = "text"     // plain text output
	EventToolUse    EventType = "tool_use" // agent is using a tool
	EventToolResult EventType = "tool_result"
	EventDone       EventType = "done"  // agent finished responding
	EventError      EventType = "error" // agent errored
)

// Event is a single streamed unit of output from an agent session.
type Event struct {
	Type      EventType `json:"type"`
	Text      string    `json:"text,omitempty"`
	ToolName  string    `json:"tool_name,omitempty"`
	ToolInput string    `json:"tool_input,omitempty"`
	Error     string    `json:"error,omitempty"`
	Timestamp time.Time `json:"timestamp"`
}

// Status represents the lifecycle state of a session.
type Status string

const (
	StatusStarting Status = "starting"
	StatusIdle     Status = "idle"
	StatusBusy     Status = "busy"
	// StatusWaiting means the agent has paused and is waiting for user
	// approval or input (e.g. a y/n prompt before running a command).
	StatusWaiting Status = "waiting"
	StatusStopped Status = "stopped"
	StatusError   Status = "error"
)

// SpawnConfig holds parameters for starting an agent session.
type SpawnConfig struct {
	Name       string            // human-readable session name
	Workspace  string            // working directory for the agent
	Model      string            // model to use (if agent supports selection)
	Env        map[string]string // extra env vars
	Worktree   bool              // create an isolated git worktree for this session (local node only)
	NodeName   string            // target node; empty means "local"
	BranchName string            // git branch slug for the worktree; auto-derived from Name if empty
}

// Capabilities describes what an agent supports.
type Capabilities struct {
	Streaming   bool
	MultiTurn   bool
	ToolUse     bool
	Interactive bool
}

// Session represents a running agent instance.
type Session interface {
	ID() string
	Name() string
	AgentName() string
	Status() Status
	// Send sends a prompt and returns a channel of streaming events.
	// The channel is closed when the agent finishes responding.
	Send(ctx context.Context, prompt string) (<-chan Event, error)
	// Stop terminates the session.
	Stop() error
	// Logs returns a reader for raw agent output.
	Logs() io.Reader
	// Workspace returns the working directory.
	Workspace() string
}

// PTYSession is optionally implemented by sessions that run under a local PTY.
// Remote (gRPC) sessions do not implement this interface.
// It enables the bidirectional raw-terminal WebSocket bridge.
type PTYSession interface {
	// WriteInput writes raw bytes to the agent's PTY stdin.
	// Used to forward keystrokes from a connected browser terminal.
	WriteInput(p []byte) error

	// SubscribeRaw registers a subscriber to receive raw PTY output bytes.
	// bufSize controls the subscriber channel's buffer size.
	// The returned cancel function removes the subscriber; call it when done.
	SubscribeRaw(bufSize int) (<-chan []byte, func())

	// ResizePTY sets the PTY window size.
	// Used to forward terminal-resize events from the browser.
	ResizePTY(cols, rows uint16) error
}

// Agent is a factory for creating sessions of a particular coding agent.
type Agent interface {
	Name() string
	Capabilities() Capabilities
	Spawn(ctx context.Context, cfg SpawnConfig) (Session, error)
}
