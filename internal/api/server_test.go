package api_test

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	gogit "github.com/go-git/go-git/v6"
	"github.com/go-git/go-git/v6/plumbing/object"

	"github.com/ash-thakur-rh/meru/internal/agent"
	"github.com/ash-thakur-rh/meru/internal/api"
	"github.com/ash-thakur-rh/meru/internal/node"
	"github.com/ash-thakur-rh/meru/internal/session"
	"github.com/ash-thakur-rh/meru/internal/store"
	"github.com/ash-thakur-rh/meru/internal/testutil"
)

const testAgent = "mock-api"

// setupServer registers a mock agent + local node, returns a test HTTP server
// and its underlying store.
func setupServer(t *testing.T, events ...agent.Event) (*httptest.Server, *store.Store) {
	t.Helper()

	agent.Register(testutil.NewMockAgent(testAgent, events...))
	node.Register(node.NewLocalNode())
	t.Cleanup(func() { agent.Unregister(testAgent) })

	st, err := store.Open(":memory:")
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	t.Cleanup(func() { st.Close() })

	mgr := session.New(st)
	srv := api.New(mgr, st)
	ts := httptest.NewServer(srv.Handler(nil))
	t.Cleanup(ts.Close)

	return ts, st
}

func postJSON(t *testing.T, url string, body any) *http.Response {
	t.Helper()
	b, _ := json.Marshal(body)
	resp, err := http.Post(url, "application/json", bytes.NewReader(b))
	if err != nil {
		t.Fatalf("POST %s: %v", url, err)
	}
	return resp
}

func decode(t *testing.T, resp *http.Response, out any) {
	t.Helper()
	if err := json.NewDecoder(resp.Body).Decode(out); err != nil {
		t.Fatalf("decode response: %v", err)
	}
}

// --- Health ---

func TestHealthz(t *testing.T) {
	ts, _ := setupServer(t)
	resp, err := http.Get(ts.URL + "/healthz")
	if err != nil {
		t.Fatalf("GET /healthz: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Errorf("status = %d, want 200", resp.StatusCode)
	}
}

// --- Sessions ---

func TestSpawn_OK(t *testing.T) {
	ts, _ := setupServer(t)

	resp := postJSON(t, ts.URL+"/sessions", map[string]any{
		"agent":     testAgent,
		"name":      "api-bot",
		"workspace": t.TempDir(),
	})
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusCreated {
		body, _ := io.ReadAll(resp.Body)
		t.Fatalf("status = %d, body = %s", resp.StatusCode, body)
	}

	var got map[string]string
	decode(t, resp, &got)
	if got["name"] != "api-bot" {
		t.Errorf("name = %q, want api-bot", got["name"])
	}
	if got["id"] == "" {
		t.Error("expected non-empty id")
	}
}

func TestSpawn_MissingAgent(t *testing.T) {
	ts, _ := setupServer(t)
	resp := postJSON(t, ts.URL+"/sessions", map[string]any{
		"workspace": t.TempDir(),
	})
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("status = %d, want 400", resp.StatusCode)
	}
}

func TestSpawn_UnknownAgent(t *testing.T) {
	ts, _ := setupServer(t)
	resp := postJSON(t, ts.URL+"/sessions", map[string]any{
		"agent":     "no-such-agent",
		"workspace": t.TempDir(),
	})
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusInternalServerError {
		t.Errorf("status = %d, want 500", resp.StatusCode)
	}
}

func TestList_Empty(t *testing.T) {
	ts, _ := setupServer(t)
	resp, err := http.Get(ts.URL + "/sessions")
	if err != nil {
		t.Fatalf("GET /sessions: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Errorf("status = %d, want 200", resp.StatusCode)
	}
}

func TestList_AfterSpawn(t *testing.T) {
	ts, _ := setupServer(t)

	postJSON(t, ts.URL+"/sessions", map[string]any{
		"agent": testAgent, "workspace": t.TempDir(),
	}).Body.Close()
	postJSON(t, ts.URL+"/sessions", map[string]any{
		"agent": testAgent, "workspace": t.TempDir(),
	}).Body.Close()

	resp, err := http.Get(ts.URL + "/sessions")
	if err != nil {
		t.Fatalf("GET /sessions: %v", err)
	}
	defer resp.Body.Close()
	var sessions []map[string]any
	decode(t, resp, &sessions)
	if len(sessions) != 2 {
		t.Errorf("len = %d, want 2", len(sessions))
	}
}

func TestGet_Session(t *testing.T) {
	ts, _ := setupServer(t)

	spawn := postJSON(t, ts.URL+"/sessions", map[string]any{
		"agent": testAgent, "name": "get-test", "workspace": t.TempDir(),
	})
	defer spawn.Body.Close()
	var created map[string]string
	decode(t, spawn, &created)
	id := created["id"]

	resp, err := http.Get(ts.URL + "/sessions/" + id)
	if err != nil {
		t.Fatalf("GET session: %v", err)
	}
	defer resp.Body.Close()
	var got map[string]string
	decode(t, resp, &got)
	if got["id"] != id {
		t.Errorf("id = %q, want %q", got["id"], id)
	}
}

func TestGet_UnknownSession(t *testing.T) {
	ts, _ := setupServer(t)
	resp, err := http.Get(ts.URL + "/sessions/no-such-id")
	if err != nil {
		t.Fatalf("GET unknown session: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusNotFound {
		t.Errorf("status = %d, want 404", resp.StatusCode)
	}
}

func TestStop_Session(t *testing.T) {
	ts, _ := setupServer(t)

	spawn := postJSON(t, ts.URL+"/sessions", map[string]any{
		"agent": testAgent, "workspace": t.TempDir(),
	})
	defer spawn.Body.Close()
	var created map[string]string
	decode(t, spawn, &created)
	id := created["id"]

	req, _ := http.NewRequest(http.MethodDelete, ts.URL+"/sessions/"+id, nil)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("DELETE session: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusNoContent {
		t.Errorf("status = %d, want 204", resp.StatusCode)
	}

	// After stopping, the session record is kept in the store as "stopped".
	resp2, err2 := http.Get(ts.URL + "/sessions/" + id)
	if err2 != nil {
		t.Fatalf("GET stopped session: %v", err2)
	}
	defer resp2.Body.Close()
	if resp2.StatusCode != http.StatusOK {
		t.Errorf("after stop, status = %d, want 200 (kept in store)", resp2.StatusCode)
	}
	var stored map[string]string
	decode(t, resp2, &stored)
	if stored["status"] != "stopped" {
		t.Errorf("stored status = %q, want %q", stored["status"], "stopped")
	}
}

func TestStop_UnknownSession(t *testing.T) {
	ts, _ := setupServer(t)
	req, _ := http.NewRequest(http.MethodDelete, ts.URL+"/sessions/no-such-id", nil)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("DELETE unknown session: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusNotFound {
		t.Errorf("status = %d, want 404", resp.StatusCode)
	}
}

func TestSend_StreamsEvents(t *testing.T) {
	evs := testutil.TextEvents("chunk1", "chunk2")
	ts, _ := setupServer(t, evs...)

	spawn := postJSON(t, ts.URL+"/sessions", map[string]any{
		"agent": testAgent, "workspace": t.TempDir(),
	})
	defer spawn.Body.Close()
	var created map[string]string
	decode(t, spawn, &created)
	id := created["id"]

	resp := postJSON(t, ts.URL+"/sessions/"+id+"/send", map[string]any{
		"prompt": "hello",
	})
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		t.Fatalf("status = %d, body = %s", resp.StatusCode, body)
	}

	body, _ := io.ReadAll(resp.Body)
	lines := strings.Split(strings.TrimSpace(string(body)), "\n")
	// Expect at least chunk1, chunk2, done
	if len(lines) < 3 {
		t.Fatalf("expected ≥3 nd-JSON lines, got %d: %s", len(lines), body)
	}

	var firstEv agent.Event
	if err := json.Unmarshal([]byte(lines[0]), &firstEv); err != nil {
		t.Fatalf("decode first event: %v", err)
	}
	if firstEv.Text != "chunk1" {
		t.Errorf("first event text = %q, want chunk1", firstEv.Text)
	}
}

func TestSend_MissingPrompt(t *testing.T) {
	ts, _ := setupServer(t)
	spawn := postJSON(t, ts.URL+"/sessions", map[string]any{
		"agent": testAgent, "workspace": t.TempDir(),
	})
	defer spawn.Body.Close()
	var created map[string]string
	decode(t, spawn, &created)
	id := created["id"]

	resp := postJSON(t, ts.URL+"/sessions/"+id+"/send", map[string]any{})
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("status = %d, want 400", resp.StatusCode)
	}
}

func TestSend_UnknownSession(t *testing.T) {
	ts, _ := setupServer(t)
	resp := postJSON(t, ts.URL+"/sessions/no-such-id/send", map[string]any{
		"prompt": "hi",
	})
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusInternalServerError {
		t.Errorf("status = %d, want 500", resp.StatusCode)
	}
}

func TestBroadcast_AllSessions(t *testing.T) {
	evs := testutil.TextEvents("pong")
	ts, _ := setupServer(t, evs...)

	postJSON(t, ts.URL+"/sessions", map[string]any{
		"agent": testAgent, "workspace": t.TempDir(),
	}).Body.Close()
	postJSON(t, ts.URL+"/sessions", map[string]any{
		"agent": testAgent, "workspace": t.TempDir(),
	}).Body.Close()

	resp := postJSON(t, ts.URL+"/broadcast", map[string]any{
		"prompt": "ping",
	})
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		t.Fatalf("status = %d, body = %s", resp.StatusCode, body)
	}

	var results []map[string]any
	decode(t, resp, &results)
	if len(results) != 2 {
		t.Errorf("len = %d, want 2", len(results))
	}
}

func TestBroadcast_MissingPrompt(t *testing.T) {
	ts, _ := setupServer(t)
	resp := postJSON(t, ts.URL+"/broadcast", map[string]any{})
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("status = %d, want 400", resp.StatusCode)
	}
}

// --- Nodes ---

func TestListNodes_Empty(t *testing.T) {
	ts, _ := setupServer(t)
	resp, err := http.Get(ts.URL + "/nodes")
	if err != nil {
		t.Fatalf("GET /nodes: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Errorf("status = %d, want 200", resp.StatusCode)
	}
	var nodes []any
	decode(t, resp, &nodes)
	// zero remote nodes persisted
	if len(nodes) != 0 {
		t.Errorf("len = %d, want 0", len(nodes))
	}
}

func TestAddNode_MissingFields(t *testing.T) {
	ts, _ := setupServer(t)
	resp := postJSON(t, ts.URL+"/nodes", map[string]any{
		"name": "gpu",
	})
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("status = %d, want 400", resp.StatusCode)
	}
}

func TestAddNode_LocalReserved(t *testing.T) {
	ts, _ := setupServer(t)
	resp := postJSON(t, ts.URL+"/nodes", map[string]any{
		"name": "local", "addr": "localhost:9090", "token": "secret",
	})
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("status = %d, want 400", resp.StatusCode)
	}
}

func TestRemoveNode_Unknown(t *testing.T) {
	ts, _ := setupServer(t)
	req, _ := http.NewRequest(http.MethodDelete, ts.URL+"/nodes/no-such-node", nil)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("DELETE unknown node: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusNotFound {
		t.Errorf("status = %d, want 404", resp.StatusCode)
	}
}

func TestRemoveNode_LocalForbidden(t *testing.T) {
	ts, _ := setupServer(t)
	req, _ := http.NewRequest(http.MethodDelete, ts.URL+"/nodes/local", nil)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("DELETE local node: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("status = %d, want 400", resp.StatusCode)
	}
}

func TestPingNode_Local(t *testing.T) {
	ts, _ := setupServer(t)
	resp := postJSON(t, ts.URL+"/nodes/local/ping", nil)
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		t.Fatalf("status = %d, body = %s", resp.StatusCode, body)
	}
}

func TestPingNode_Unknown(t *testing.T) {
	ts, _ := setupServer(t)
	resp := postJSON(t, ts.URL+"/nodes/no-such/ping", nil)
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusNotFound {
		t.Errorf("status = %d, want 404", resp.StatusCode)
	}
}

// --- Filesystem browser (GET /fs) ---

func TestListDir_LocalNode(t *testing.T) {
	ts, _ := setupServer(t)
	dir := t.TempDir()
	// Create a file and a subdirectory inside dir
	if err := os.WriteFile(filepath.Join(dir, "a.txt"), []byte("x"), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}
	if err := os.MkdirAll(filepath.Join(dir, "sub"), 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}

	resp, err := http.Get(fmt.Sprintf("%s/fs?path=%s&node=local", ts.URL, dir))
	if err != nil {
		t.Fatalf("GET /fs: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		t.Fatalf("status = %d, body = %s", resp.StatusCode, body)
	}

	var listing map[string]any
	decode(t, resp, &listing)
	if listing["path"] != dir {
		t.Errorf("path = %v, want %q", listing["path"], dir)
	}
	entries, _ := listing["entries"].([]any)
	if len(entries) != 2 {
		t.Errorf("entries = %d, want 2", len(entries))
	}
}

func TestListDir_UnknownNode(t *testing.T) {
	ts, _ := setupServer(t)
	resp, err := http.Get(ts.URL + "/fs?path=/tmp&node=no-such-node")
	if err != nil {
		t.Fatalf("GET /fs: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusNotFound {
		t.Errorf("status = %d, want 404", resp.StatusCode)
	}
}

func TestListDir_DefaultsToLocal(t *testing.T) {
	ts, _ := setupServer(t)
	// Omit node param — should default to "local"
	resp, err := http.Get(ts.URL + "/fs?path=" + t.TempDir())
	if err != nil {
		t.Fatalf("GET /fs: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		t.Fatalf("status = %d, body = %s", resp.StatusCode, body)
	}
}

// --- Git clone (POST /git/clone) ---

// initAPITestRepo creates a local git repo with one commit for use as a clone source.
func initAPITestRepo(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	repo, err := gogit.PlainInit(dir, false)
	if err != nil {
		t.Fatalf("git init: %v", err)
	}
	wt, _ := repo.Worktree()
	if err := os.WriteFile(filepath.Join(dir, "file.txt"), []byte("hi"), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}
	wt.Add("file.txt")                      //nolint:errcheck
	wt.Commit("init", &gogit.CommitOptions{ //nolint:errcheck
		Author: &object.Signature{Name: "t", Email: "t@t.com", When: time.Now()},
	})
	return dir
}

func TestGitClone_Success(t *testing.T) {
	ts, _ := setupServer(t)
	src := initAPITestRepo(t)
	dest := filepath.Join(t.TempDir(), "cloned")

	resp := postJSON(t, ts.URL+"/git/clone", map[string]any{
		"url":  src,
		"dest": dest,
		"node": "local",
	})
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		t.Fatalf("status = %d, body = %s", resp.StatusCode, body)
	}

	var result map[string]string
	decode(t, resp, &result)
	if result["path"] != dest {
		t.Errorf("path = %q, want %q", result["path"], dest)
	}
	if _, err := os.Stat(filepath.Join(dest, "file.txt")); err != nil {
		t.Errorf("cloned file not found: %v", err)
	}
}

func TestGitClone_MissingURL(t *testing.T) {
	ts, _ := setupServer(t)
	resp := postJSON(t, ts.URL+"/git/clone", map[string]any{
		"dest": "/tmp/x",
	})
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("status = %d, want 400", resp.StatusCode)
	}
}

func TestGitClone_InvalidSource(t *testing.T) {
	ts, _ := setupServer(t)
	resp := postJSON(t, ts.URL+"/git/clone", map[string]any{
		"url":  "/no/such/repo/conductor-xyz",
		"dest": filepath.Join(t.TempDir(), "clone"),
	})
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusInternalServerError {
		t.Errorf("status = %d, want 500", resp.StatusCode)
	}
}

func TestGitClone_UnknownNode(t *testing.T) {
	ts, _ := setupServer(t)
	resp := postJSON(t, ts.URL+"/git/clone", map[string]any{
		"url":  "https://example.com/repo.git",
		"node": "no-such-node",
	})
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusNotFound {
		t.Errorf("status = %d, want 404", resp.StatusCode)
	}
}
