// Package session manages the lifecycle of all active agent sessions.
package session

import (
	"context"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"github.com/google/uuid"

	"github.com/ash-thakur-rh/meru/internal/agent"
	"github.com/ash-thakur-rh/meru/internal/node"
	"github.com/ash-thakur-rh/meru/internal/notify"
	"github.com/ash-thakur-rh/meru/internal/store"
	"github.com/ash-thakur-rh/meru/internal/workspace"
)

// Manager owns all running sessions and persists them via the store.
type Manager struct {
	mu       sync.RWMutex
	sessions map[string]*entry // session ID → entry
	store    *store.Store
	wt       *workspace.Manager

	// subscribers receive events from any session; keyed by a subscriber ID
	subsMu sync.RWMutex
	subs   map[string]chan SessionEvent
}

// entry pairs a live Session with its last-known status.
type entry struct {
	sess      agent.Session
	repoRoot  string // non-empty if a worktree was created
	sessionID string // copy for cleanup
}

// SessionEvent is broadcast to subscribers when something happens.
type SessionEvent struct {
	SessionID string
	Event     agent.Event
}

// New creates a Manager backed by st.
func New(st *store.Store) *Manager {
	return &Manager{
		sessions: make(map[string]*entry),
		store:    st,
		wt:       workspace.New(),
		subs:     make(map[string]chan SessionEvent),
	}
}

// Spawn creates a new session using the named agent on the specified node.
// If cfg.NodeName is empty it defaults to "local".
// If cfg.Worktree is true and the workspace is a git repo (local only),
// an isolated git worktree is created for this session.
func (m *Manager) Spawn(ctx context.Context, agentName string, cfg agent.SpawnConfig) (agent.Session, error) {
	nodeName := cfg.NodeName
	if nodeName == "" {
		nodeName = node.LocalNodeName
	}

	n, err := node.Get(nodeName)
	if err != nil {
		return nil, err
	}

	if cfg.Name == "" {
		cfg.Name = agentName + "-" + uuid.New().String()[:8]
	}

	// For local nodes, create the worktree here so the manager can clean it up on stop.
	// Remote nodes handle worktree creation themselves (via the Worktree field in SpawnRequest).
	var repoRoot string
	if nodeName == node.LocalNodeName && cfg.Worktree && workspace.IsGitRepo(cfg.Workspace) {
		root, err := workspace.RepoRoot(cfg.Workspace)
		if err != nil {
			return nil, fmt.Errorf("find repo root: %w", err)
		}
		tmpID := uuid.New().String()
		wtPath, err := m.wt.CreateWorktree(root, tmpID)
		if err != nil {
			return nil, fmt.Errorf("create worktree: %w", err)
		}
		cfg.Workspace = wtPath
		repoRoot = root
		defer func() {
			if repoRoot != "" {
				_ = m.wt.RemoveWorktree(root, tmpID)
			}
		}()
	}

	// Allocate the session ID on the control plane so we can reconnect later.
	sessionID := uuid.New().String()

	slog.Info("spawning session",
		"agent", agentName,
		"name", cfg.Name,
		"workspace", cfg.Workspace,
		"node", nodeName,
		"worktree", cfg.Worktree,
	)

	sess, err := n.Spawn(ctx, sessionID, agentName, cfg)
	if err != nil {
		slog.Error("spawn failed", "agent", agentName, "node", nodeName, "error", err)
		return nil, fmt.Errorf("spawn %s on node %s: %w", agentName, nodeName, err)
	}

	slog.Info("session spawned", "session", sess.ID(), "name", sess.Name(), "agent", agentName)

	now := time.Now()
	if err := m.store.CreateSession(store.Session{
		ID:        sess.ID(),
		Name:      sess.Name(),
		Agent:     agentName,
		Workspace: sess.Workspace(),
		NodeName:  nodeName,
		Status:    string(agent.StatusIdle),
		CreatedAt: now,
		UpdatedAt: now,
	}); err != nil {
		return nil, fmt.Errorf("persist session: %w", err)
	}

	m.mu.Lock()
	m.sessions[sess.ID()] = &entry{sess: sess, repoRoot: repoRoot, sessionID: sess.ID()}
	repoRoot = ""
	m.mu.Unlock()

	return sess, nil
}

// Send delivers a prompt to a session and streams events to callers.
// Events are also persisted and broadcast to all subscribers.
func (m *Manager) Send(ctx context.Context, sessionID, prompt string) (<-chan agent.Event, error) {
	m.mu.RLock()
	e, ok := m.sessions[sessionID]
	m.mu.RUnlock()
	if !ok {
		return nil, fmt.Errorf("session %s not found", sessionID)
	}

	raw, err := e.sess.Send(ctx, prompt)
	if err != nil {
		return nil, err
	}

	_ = m.store.UpdateSessionStatus(sessionID, string(agent.StatusBusy))

	out := make(chan agent.Event, 64)
	sessName := e.sess.Name()
	agentName := e.sess.AgentName()

	go func() {
		defer close(out)
		defer m.store.UpdateSessionStatus(sessionID, string(agent.StatusIdle)) //nolint:errcheck
		for ev := range raw {
			// Persist
			m.store.AppendEvent(store.Event{ //nolint:errcheck
				SessionID: sessionID,
				Type:      string(ev.Type),
				Text:      ev.Text,
				ToolName:  ev.ToolName,
				ToolInput: ev.ToolInput,
				Error:     ev.Error,
				Timestamp: ev.Timestamp,
			})
			// Desktop notifications on terminal events
			switch ev.Type {
			case agent.EventDone:
				slog.Info("session task done", "session", sessionID, "name", sessName)
				notify.TaskDone(sessName, agentName)
			case agent.EventError:
				slog.Error("session error event", "session", sessionID, "name", sessName, "error", ev.Error)
				notify.Error(sessName, ev.Error)
			}
			// Broadcast to subscribers
			m.broadcast(SessionEvent{SessionID: sessionID, Event: ev})
			// Forward to caller
			out <- ev
		}
	}()

	return out, nil
}

// Get returns the live session by ID.
func (m *Manager) Get(id string) (agent.Session, error) {
	m.mu.RLock()
	e, ok := m.sessions[id]
	m.mu.RUnlock()
	if !ok {
		return nil, fmt.Errorf("session %s not found", id)
	}
	return e.sess, nil
}

// Stop terminates a live session and marks it as stopped in the store.
// The record is kept so it can appear in history and be re-spawned later.
func (m *Manager) Stop(id string) error {
	m.mu.Lock()
	e, ok := m.sessions[id]
	if ok {
		delete(m.sessions, id)
	}
	m.mu.Unlock()
	if !ok {
		return fmt.Errorf("session %s not found", id)
	}
	slog.Info("stopping session", "session", id, "name", e.sess.Name())
	_ = m.store.UpdateSessionStatus(id, string(agent.StatusStopped))
	if e.repoRoot != "" {
		_ = m.wt.RemoveWorktree(e.repoRoot, e.sessionID)
	}
	return e.sess.Stop()
}

// Purge permanently removes a stopped session record from the store.
// It returns an error if the session is still live (use Stop first).
func (m *Manager) Purge(id string) error {
	m.mu.RLock()
	_, live := m.sessions[id]
	m.mu.RUnlock()
	if live {
		return fmt.Errorf("session %s is still running; stop it first", id)
	}
	return m.store.DeleteSession(id)
}

// ListFromStore returns all sessions stored in the DB (including stopped ones).
// For live sessions, the real-time status from the in-memory map is overlaid
// so the dashboard reflects transient states like "waiting" and "busy".
func (m *Manager) ListFromStore() ([]store.Session, error) {
	sessions, err := m.store.ListSessions()
	if err != nil {
		return nil, err
	}
	m.mu.RLock()
	defer m.mu.RUnlock()
	for i, s := range sessions {
		if e, ok := m.sessions[s.ID]; ok {
			sessions[i].Status = string(e.sess.Status())
		}
	}
	return sessions, nil
}

// BroadcastResult holds the outcome of sending a prompt to one session.
type BroadcastResult struct {
	SessionID   string
	SessionName string
	Events      []agent.Event
	Err         error
}

// Broadcast fans out a prompt to the given session IDs (or all active sessions
// if ids is empty) concurrently and collects the results.
func (m *Manager) Broadcast(ctx context.Context, prompt string, ids []string) []BroadcastResult {
	m.mu.RLock()
	targets := make([]*entry, 0)
	if len(ids) == 0 {
		for _, e := range m.sessions {
			if e.sess.Status() != agent.StatusStopped {
				targets = append(targets, e)
			}
		}
	} else {
		for _, id := range ids {
			if e, ok := m.sessions[id]; ok {
				targets = append(targets, e)
			}
		}
	}
	m.mu.RUnlock()

	results := make([]BroadcastResult, len(targets))
	var wg sync.WaitGroup
	for i, e := range targets {
		wg.Add(1)
		go func(idx int, entry *entry) {
			defer wg.Done()
			res := BroadcastResult{
				SessionID:   entry.sess.ID(),
				SessionName: entry.sess.Name(),
			}
			ch, err := m.Send(ctx, entry.sess.ID(), prompt)
			if err != nil {
				res.Err = err
				results[idx] = res
				return
			}
			for ev := range ch {
				res.Events = append(res.Events, ev)
			}
			results[idx] = res
		}(i, e)
	}
	wg.Wait()
	return results
}

// History returns persisted events for a session.
func (m *Manager) History(sessionID string) ([]store.Event, error) {
	return m.store.ListEvents(sessionID)
}

// Subscribe returns a channel that receives events from all sessions.
// Call the returned cancel func to unsubscribe.
func (m *Manager) Subscribe() (id string, ch <-chan SessionEvent, cancel func()) {
	c := make(chan SessionEvent, 128)
	id = uuid.New().String()

	m.subsMu.Lock()
	m.subs[id] = c
	m.subsMu.Unlock()

	cancel = func() {
		m.subsMu.Lock()
		delete(m.subs, id)
		m.subsMu.Unlock()
		close(c)
	}
	return id, c, cancel
}

func (m *Manager) broadcast(ev SessionEvent) {
	m.subsMu.RLock()
	defer m.subsMu.RUnlock()
	for _, ch := range m.subs {
		select {
		case ch <- ev:
		default: // drop if subscriber is slow
		}
	}
}
