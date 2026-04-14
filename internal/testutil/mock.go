// Package testutil provides shared test helpers.
// It is only compiled into test binaries.
package testutil

import (
	"context"
	"io"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"

	"github.com/ash-thakur-rh/meru/internal/agent"
)

// MockAgent implements agent.Agent. It returns a MockSession whose Send
// method emits a configurable slice of events followed by EventDone.
type MockAgent struct {
	name   string
	Events []agent.Event // events the session will emit on every Send call
}

func NewMockAgent(name string, events ...agent.Event) *MockAgent {
	return &MockAgent{name: name, Events: events}
}

func (m *MockAgent) Name() string { return m.name }
func (m *MockAgent) Capabilities() agent.Capabilities {
	return agent.Capabilities{Streaming: true, MultiTurn: true}
}
func (m *MockAgent) Spawn(_ context.Context, cfg agent.SpawnConfig) (agent.Session, error) {
	return &MockSession{
		id:        uuid.New().String(),
		name:      cfg.Name,
		workspace: cfg.Workspace,
		agentName: m.name,
		events:    m.Events,
		status:    agent.StatusIdle,
	}, nil
}

// MockSession implements agent.Session. Send emits the configured events
// followed by EventDone and immediately closes the channel.
type MockSession struct {
	id        string
	name      string
	workspace string
	agentName string
	events    []agent.Event

	mu     sync.Mutex
	status agent.Status

	// SendCalls records prompts delivered to this session.
	SendCalls []string
}

func (s *MockSession) ID() string        { return s.id }
func (s *MockSession) Name() string      { return s.name }
func (s *MockSession) AgentName() string { return s.agentName }
func (s *MockSession) Workspace() string { return s.workspace }
func (s *MockSession) Logs() io.Reader   { return strings.NewReader("") }

func (s *MockSession) Status() agent.Status {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.status
}

func (s *MockSession) Stop() error {
	s.mu.Lock()
	s.status = agent.StatusStopped
	s.mu.Unlock()
	return nil
}

func (s *MockSession) Send(_ context.Context, prompt string) (<-chan agent.Event, error) {
	s.mu.Lock()
	s.status = agent.StatusBusy
	s.SendCalls = append(s.SendCalls, prompt)
	s.mu.Unlock()

	ch := make(chan agent.Event, len(s.events)+1)
	for _, ev := range s.events {
		ch <- ev
	}
	ch <- agent.Event{Type: agent.EventDone, Timestamp: time.Now()}
	close(ch)

	s.mu.Lock()
	s.status = agent.StatusIdle
	s.mu.Unlock()

	return ch, nil
}

// TextEvents returns a slice of EventText events from the given strings —
// useful for constructing MockAgent event lists.
func TextEvents(texts ...string) []agent.Event {
	evs := make([]agent.Event, len(texts))
	for i, t := range texts {
		evs[i] = agent.Event{Type: agent.EventText, Text: t, Timestamp: time.Now()}
	}
	return evs
}
