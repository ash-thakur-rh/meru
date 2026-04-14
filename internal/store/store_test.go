package store_test

import (
	"testing"
	"time"

	"github.com/ash-thakur-rh/meru/internal/store"
)

func openTestDB(t *testing.T) *store.Store {
	t.Helper()
	st, err := store.Open(":memory:")
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	t.Cleanup(func() { st.Close() })
	return st
}

// --- Session tests ---

func TestSession_CreateAndGet(t *testing.T) {
	st := openTestDB(t)
	now := time.Now().Truncate(time.Second)

	sess := store.Session{
		ID:        "sess-1",
		Name:      "my-bot",
		Agent:     "claude",
		Workspace: "/tmp/work",
		NodeName:  "local",
		Status:    "idle",
		CreatedAt: now,
		UpdatedAt: now,
	}
	if err := st.CreateSession(sess); err != nil {
		t.Fatalf("CreateSession: %v", err)
	}

	got, err := st.GetSession("sess-1")
	if err != nil {
		t.Fatalf("GetSession: %v", err)
	}

	if got.ID != sess.ID || got.Name != sess.Name || got.Agent != sess.Agent {
		t.Errorf("got %+v, want %+v", got, sess)
	}
	if got.NodeName != "local" {
		t.Errorf("NodeName = %q, want %q", got.NodeName, "local")
	}
}

func TestSession_DefaultNodeName(t *testing.T) {
	st := openTestDB(t)
	now := time.Now()

	// Create with no NodeName — should default to "local"
	err := st.CreateSession(store.Session{
		ID: "sess-2", Name: "b", Agent: "aider", Workspace: "/",
		Status: "idle", CreatedAt: now, UpdatedAt: now,
	})
	if err != nil {
		t.Fatalf("CreateSession: %v", err)
	}

	got, err := st.GetSession("sess-2")
	if err != nil {
		t.Fatalf("GetSession: %v", err)
	}
	if got.NodeName != "local" {
		t.Errorf("NodeName = %q, want \"local\"", got.NodeName)
	}
}

func TestSession_UpdateStatus(t *testing.T) {
	st := openTestDB(t)
	now := time.Now()
	_ = st.CreateSession(store.Session{
		ID: "s3", Name: "b", Agent: "claude", Workspace: "/", NodeName: "local",
		Status: "idle", CreatedAt: now, UpdatedAt: now,
	})

	if err := st.UpdateSessionStatus("s3", "busy"); err != nil {
		t.Fatalf("UpdateSessionStatus: %v", err)
	}
	got, _ := st.GetSession("s3")
	if got.Status != "busy" {
		t.Errorf("Status = %q, want \"busy\"", got.Status)
	}
}

func TestSession_List(t *testing.T) {
	st := openTestDB(t)
	now := time.Now()

	for i, id := range []string{"a1", "a2", "a3"} {
		_ = st.CreateSession(store.Session{
			ID: id, Name: id, Agent: "claude", Workspace: "/",
			NodeName: "local", Status: "idle",
			CreatedAt: now.Add(time.Duration(i) * time.Second),
			UpdatedAt: now,
		})
	}

	sessions, err := st.ListSessions()
	if err != nil {
		t.Fatalf("ListSessions: %v", err)
	}
	if len(sessions) != 3 {
		t.Errorf("len = %d, want 3", len(sessions))
	}
}

func TestSession_Delete(t *testing.T) {
	st := openTestDB(t)
	now := time.Now()
	_ = st.CreateSession(store.Session{
		ID: "del", Name: "x", Agent: "claude", Workspace: "/",
		NodeName: "local", Status: "idle", CreatedAt: now, UpdatedAt: now,
	})

	if err := st.DeleteSession("del"); err != nil {
		t.Fatalf("DeleteSession: %v", err)
	}
	sessions, _ := st.ListSessions()
	if len(sessions) != 0 {
		t.Errorf("expected 0 sessions after delete, got %d", len(sessions))
	}
}

// --- Event tests ---

func TestEvent_AppendAndList(t *testing.T) {
	st := openTestDB(t)
	now := time.Now()
	_ = st.CreateSession(store.Session{
		ID: "ev-sess", Name: "x", Agent: "claude", Workspace: "/",
		NodeName: "local", Status: "idle", CreatedAt: now, UpdatedAt: now,
	})

	events := []store.Event{
		{SessionID: "ev-sess", Type: "text", Text: "hello", Timestamp: now},
		{SessionID: "ev-sess", Type: "text", Text: "world", Timestamp: now},
		{SessionID: "ev-sess", Type: "done", Timestamp: now},
	}
	for _, ev := range events {
		if err := st.AppendEvent(ev); err != nil {
			t.Fatalf("AppendEvent: %v", err)
		}
	}

	got, err := st.ListEvents("ev-sess")
	if err != nil {
		t.Fatalf("ListEvents: %v", err)
	}
	if len(got) != 3 {
		t.Fatalf("len = %d, want 3", len(got))
	}
	if got[0].Text != "hello" || got[2].Type != "done" {
		t.Errorf("unexpected events: %+v", got)
	}
}

// --- Node tests ---

func TestNode_UpsertAndGet(t *testing.T) {
	st := openTestDB(t)

	rec := store.NodeRecord{Name: "gpu-box", Addr: "10.0.0.5:9090", Token: "secret", TLS: false}
	if err := st.UpsertNode(rec); err != nil {
		t.Fatalf("UpsertNode: %v", err)
	}

	got, err := st.GetNode("gpu-box")
	if err != nil {
		t.Fatalf("GetNode: %v", err)
	}
	if got.Addr != rec.Addr || got.Token != rec.Token {
		t.Errorf("got %+v, want %+v", got, rec)
	}
}

func TestNode_Upsert_Updates(t *testing.T) {
	st := openTestDB(t)
	_ = st.UpsertNode(store.NodeRecord{Name: "n1", Addr: "old:9090", Token: "tok"})
	_ = st.UpsertNode(store.NodeRecord{Name: "n1", Addr: "new:9090", Token: "tok2"})

	got, _ := st.GetNode("n1")
	if got.Addr != "new:9090" || got.Token != "tok2" {
		t.Errorf("upsert didn't update: got %+v", got)
	}
}

func TestNode_List(t *testing.T) {
	st := openTestDB(t)
	for _, name := range []string{"node-a", "node-b"} {
		_ = st.UpsertNode(store.NodeRecord{Name: name, Addr: name + ":9090", Token: "x"})
	}

	nodes, err := st.ListNodes()
	if err != nil {
		t.Fatalf("ListNodes: %v", err)
	}
	if len(nodes) != 2 {
		t.Errorf("len = %d, want 2", len(nodes))
	}
}

func TestNode_Delete(t *testing.T) {
	st := openTestDB(t)
	_ = st.UpsertNode(store.NodeRecord{Name: "rm-me", Addr: "x:9090", Token: "t"})
	if err := st.DeleteNode("rm-me"); err != nil {
		t.Fatalf("DeleteNode: %v", err)
	}
	nodes, _ := st.ListNodes()
	if len(nodes) != 0 {
		t.Errorf("expected 0 nodes after delete, got %d", len(nodes))
	}
}

func TestNode_TLS(t *testing.T) {
	st := openTestDB(t)
	_ = st.UpsertNode(store.NodeRecord{Name: "tls-node", Addr: "x:9090", Token: "t", TLS: true})
	got, _ := st.GetNode("tls-node")
	if !got.TLS {
		t.Error("TLS should be true")
	}
}
