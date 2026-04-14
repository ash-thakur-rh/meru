package session_test

import (
	"context"
	"testing"
	"time"

	"github.com/ash-thakur-rh/meru/internal/agent"
	"github.com/ash-thakur-rh/meru/internal/node"
	"github.com/ash-thakur-rh/meru/internal/session"
	"github.com/ash-thakur-rh/meru/internal/store"
	"github.com/ash-thakur-rh/meru/internal/testutil"
)

// setup registers a mock agent + local node, opens an in-memory store,
// and returns a Manager ready for testing.
func setup(t *testing.T, events ...agent.Event) (*session.Manager, string) {
	t.Helper()

	const agentName = "mock"
	agent.Register(testutil.NewMockAgent(agentName, events...))
	node.Register(node.NewLocalNode())
	t.Cleanup(func() { agent.Unregister(agentName) })

	st, err := store.Open(":memory:")
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	t.Cleanup(func() { st.Close() })

	return session.New(st), agentName
}

func TestSpawn_LocalNode(t *testing.T) {
	mgr, agentName := setup(t)

	sess, err := mgr.Spawn(context.Background(), agentName, agent.SpawnConfig{
		Name:      "test-bot",
		Workspace: t.TempDir(),
	})
	if err != nil {
		t.Fatalf("Spawn: %v", err)
	}

	if sess.Name() != "test-bot" {
		t.Errorf("Name = %q, want %q", sess.Name(), "test-bot")
	}
	if sess.AgentName() != agentName {
		t.Errorf("AgentName = %q, want %q", sess.AgentName(), agentName)
	}
	if sess.Status() != agent.StatusIdle {
		t.Errorf("Status = %q, want idle", sess.Status())
	}
}

func TestSpawn_AutoName(t *testing.T) {
	mgr, agentName := setup(t)

	sess, err := mgr.Spawn(context.Background(), agentName, agent.SpawnConfig{
		Workspace: t.TempDir(),
	})
	if err != nil {
		t.Fatalf("Spawn: %v", err)
	}
	if sess.Name() == "" {
		t.Error("expected auto-generated name, got empty string")
	}
}

func TestSpawn_UnknownAgent(t *testing.T) {
	mgr, _ := setup(t)
	_, err := mgr.Spawn(context.Background(), "no-such-agent", agent.SpawnConfig{Workspace: "/"})
	if err == nil {
		t.Error("expected error for unknown agent, got nil")
	}
}

func TestSpawn_UnknownNode(t *testing.T) {
	mgr, agentName := setup(t)
	_, err := mgr.Spawn(context.Background(), agentName, agent.SpawnConfig{
		Workspace: "/",
		NodeName:  "no-such-node",
	})
	if err == nil {
		t.Error("expected error for unknown node, got nil")
	}
}

func TestSend_ReceivesEvents(t *testing.T) {
	evs := testutil.TextEvents("hello ", "world")
	mgr, agentName := setup(t, evs...)

	sess, _ := mgr.Spawn(context.Background(), agentName, agent.SpawnConfig{Workspace: t.TempDir()})

	ch, err := mgr.Send(context.Background(), sess.ID(), "do something")
	if err != nil {
		t.Fatalf("Send: %v", err)
	}

	var got []agent.Event
	for ev := range ch {
		got = append(got, ev)
	}

	// Expect: text "hello ", text "world", done
	if len(got) < 3 {
		t.Fatalf("expected ≥3 events, got %d: %v", len(got), got)
	}
	if got[0].Type != agent.EventText || got[0].Text != "hello " {
		t.Errorf("event[0] = %+v", got[0])
	}
	if got[len(got)-1].Type != agent.EventDone {
		t.Errorf("last event type = %q, want done", got[len(got)-1].Type)
	}
}

func TestSend_PersistsEvents(t *testing.T) {
	evs := testutil.TextEvents("persisted!")
	mgr, agentName := setup(t, evs...)

	sess, _ := mgr.Spawn(context.Background(), agentName, agent.SpawnConfig{Workspace: t.TempDir()})

	ch, _ := mgr.Send(context.Background(), sess.ID(), "save this")
	for range ch {
	}

	history, err := mgr.History(sess.ID())
	if err != nil {
		t.Fatalf("History: %v", err)
	}
	if len(history) == 0 {
		t.Fatal("expected persisted events, got none")
	}
	found := false
	for _, ev := range history {
		if ev.Type == "text" && ev.Text == "persisted!" {
			found = true
		}
	}
	if !found {
		t.Errorf("persisted events = %+v, expected text event with 'persisted!'", history)
	}
}

func TestSend_UnknownSession(t *testing.T) {
	mgr, _ := setup(t)
	_, err := mgr.Send(context.Background(), "no-such-session-id", "hello")
	if err == nil {
		t.Error("expected error sending to unknown session")
	}
}

func TestStop_Session(t *testing.T) {
	mgr, agentName := setup(t)

	sess, _ := mgr.Spawn(context.Background(), agentName, agent.SpawnConfig{Workspace: t.TempDir()})

	if err := mgr.Stop(sess.ID()); err != nil {
		t.Fatalf("Stop: %v", err)
	}

	// Session should no longer be in the live registry.
	_, err := mgr.Get(sess.ID())
	if err == nil {
		t.Error("expected error getting stopped session from live registry")
	}

	// Stopped sessions are kept in the store with status "stopped" so they
	// appear in history and can be re-spawned or purged from the UI.
	sessions, _ := mgr.ListFromStore()
	var found bool
	for _, s := range sessions {
		if s.ID == sess.ID() {
			found = true
			if s.Status != "stopped" {
				t.Errorf("expected store status %q, got %q", "stopped", s.Status)
			}
		}
	}
	if !found {
		t.Error("stopped session should remain in store")
	}

	// Purge permanently removes it.
	if err := mgr.Purge(sess.ID()); err != nil {
		t.Fatalf("Purge: %v", err)
	}
	sessions, _ = mgr.ListFromStore()
	for _, s := range sessions {
		if s.ID == sess.ID() {
			t.Error("purged session should not be in store")
		}
	}
}

func TestStop_UnknownSession(t *testing.T) {
	mgr, _ := setup(t)
	err := mgr.Stop("no-such-id")
	if err == nil {
		t.Error("expected error stopping unknown session")
	}
}

func TestBroadcast_AllActiveSessions(t *testing.T) {
	evs := testutil.TextEvents("pong")
	mgr, agentName := setup(t, evs...)

	sess1, _ := mgr.Spawn(context.Background(), agentName, agent.SpawnConfig{Name: "s1", Workspace: t.TempDir()})
	sess2, _ := mgr.Spawn(context.Background(), agentName, agent.SpawnConfig{Name: "s2", Workspace: t.TempDir()})

	results := mgr.Broadcast(context.Background(), "ping", nil)

	if len(results) != 2 {
		t.Fatalf("expected 2 results, got %d", len(results))
	}
	for _, r := range results {
		if r.Err != nil {
			t.Errorf("session %s error: %v", r.SessionID, r.Err)
		}
	}
	_ = sess1
	_ = sess2
}

func TestBroadcast_TargetedSessions(t *testing.T) {
	evs := testutil.TextEvents("response")
	mgr, agentName := setup(t, evs...)

	s1, _ := mgr.Spawn(context.Background(), agentName, agent.SpawnConfig{Name: "s1", Workspace: t.TempDir()})
	s2, _ := mgr.Spawn(context.Background(), agentName, agent.SpawnConfig{Name: "s2", Workspace: t.TempDir()})

	results := mgr.Broadcast(context.Background(), "only s1", []string{s1.ID()})

	if len(results) != 1 {
		t.Errorf("expected 1 result, got %d", len(results))
	}
	if results[0].SessionID != s1.ID() {
		t.Errorf("got result for %s, want %s", results[0].SessionID, s1.ID())
	}
	_ = s2
}

func TestSubscribe_ReceivesEvents(t *testing.T) {
	evs := testutil.TextEvents("sub-event")
	mgr, agentName := setup(t, evs...)

	sess, _ := mgr.Spawn(context.Background(), agentName, agent.SpawnConfig{Workspace: t.TempDir()})

	_, subCh, cancel := mgr.Subscribe()
	defer cancel()

	ch, _ := mgr.Send(context.Background(), sess.ID(), "trigger")
	for range ch {
	} // drain

	// Give the broadcast goroutine a moment
	timeout := time.After(2 * time.Second)
	found := false
	for !found {
		select {
		case ev := <-subCh:
			if ev.SessionID == sess.ID() {
				found = true
			}
		case <-timeout:
			t.Fatal("timed out waiting for subscriber event")
		}
	}
}
