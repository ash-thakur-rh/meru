//go:build smoke

// Package tests contains full-stack smoke tests that build and run the real
// meru binary. Run with: go test -tags smoke ./tests/...
package tests

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"
)

// buildBinary compiles cmd/meru into a temp directory and returns the path.
func buildBinary(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	out := filepath.Join(dir, "meru")
	cmd := exec.Command("go", "build", "-o", out, "../cmd/meru")
	cmd.Dir = filepath.Join(os.Getenv("GOPATH"), "src/github.com/ash-thakur-rh/meru")
	if b, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("build meru: %v\n%s", err, b)
	}
	return out
}

// freePort picks a random available TCP port.
func freePort(t *testing.T) int {
	t.Helper()
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("free port: %v", err)
	}
	defer l.Close()
	return l.Addr().(*net.TCPAddr).Port
}

// startDaemon starts the meru daemon on a random port and returns
// the base URL. The process is killed when the test finishes.
func startDaemon(t *testing.T) string {
	t.Helper()

	// Build relative to the module root (one directory up from tests/)
	moduleRoot, err := filepath.Abs("..")
	if err != nil {
		t.Fatalf("abs path: %v", err)
	}

	dir := t.TempDir()
	binPath := filepath.Join(dir, "meru")
	build := exec.Command("go", "build", "-o", binPath, "./cmd/meru")
	build.Dir = moduleRoot
	if out, err := build.CombinedOutput(); err != nil {
		t.Fatalf("build: %v\n%s", err, out)
	}

	port := freePort(t)
	addr := fmt.Sprintf("127.0.0.1:%d", port)

	// Use a temp data dir so the daemon doesn't touch ~/.meru
	dataDir := filepath.Join(dir, "data")

	cmd := exec.Command(binPath, "serve", "--addr", addr)
	cmd.Env = append(os.Environ(), "HOME="+dir) // redirect ~/.meru
	cmd.Dir = dataDir
	if err := os.MkdirAll(dataDir, 0o755); err != nil {
		t.Fatalf("mkdir data: %v", err)
	}

	cmd.Stdout = io.Discard
	cmd.Stderr = io.Discard

	if err := cmd.Start(); err != nil {
		t.Fatalf("start meru: %v", err)
	}
	t.Cleanup(func() {
		cmd.Process.Kill() //nolint:errcheck
		cmd.Wait()         //nolint:errcheck
	})

	baseURL := "http://" + addr
	waitForReady(t, baseURL+"/healthz", 10*time.Second)
	return baseURL
}

// waitForReady polls url until it returns 200 or the deadline passes.
func waitForReady(t *testing.T, url string, timeout time.Duration) {
	t.Helper()
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		resp, err := http.Get(url)
		if err == nil && resp.StatusCode == http.StatusOK {
			resp.Body.Close()
			return
		}
		if resp != nil {
			resp.Body.Close()
		}
		time.Sleep(100 * time.Millisecond)
	}
	t.Fatalf("meru did not become ready within %s", timeout)
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

// --- Tests ---

func TestSmoke_Healthz(t *testing.T) {
	base := startDaemon(t)

	resp, err := http.Get(base + "/healthz")
	if err != nil {
		t.Fatalf("GET /healthz: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("status = %d, want 200", resp.StatusCode)
	}

	var payload map[string]string
	json.NewDecoder(resp.Body).Decode(&payload) //nolint:errcheck
	if payload["status"] != "ok" {
		t.Errorf("status field = %q, want ok", payload["status"])
	}
}

func TestSmoke_Sessions_Empty(t *testing.T) {
	base := startDaemon(t)

	resp, err := http.Get(base + "/sessions")
	if err != nil {
		t.Fatalf("GET /sessions: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("status = %d, want 200", resp.StatusCode)
	}

	var sessions []any
	json.NewDecoder(resp.Body).Decode(&sessions) //nolint:errcheck
	if len(sessions) != 0 {
		t.Errorf("expected 0 sessions, got %d", len(sessions))
	}
}

func TestSmoke_Nodes_Empty(t *testing.T) {
	base := startDaemon(t)

	resp, err := http.Get(base + "/nodes")
	if err != nil {
		t.Fatalf("GET /nodes: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("status = %d, want 200", resp.StatusCode)
	}

	var nodes []any
	json.NewDecoder(resp.Body).Decode(&nodes) //nolint:errcheck
	if len(nodes) != 0 {
		t.Errorf("expected 0 remote nodes, got %d", len(nodes))
	}
}

func TestSmoke_Spawn_UnknownAgent(t *testing.T) {
	base := startDaemon(t)

	resp := postJSON(t, base+"/sessions", map[string]any{
		"agent":     "no-such-agent",
		"workspace": t.TempDir(),
	})
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusInternalServerError {
		t.Errorf("status = %d, want 500", resp.StatusCode)
	}
}

func TestSmoke_Spawn_MissingAgent(t *testing.T) {
	base := startDaemon(t)

	resp := postJSON(t, base+"/sessions", map[string]any{
		"workspace": t.TempDir(),
	})
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("status = %d, want 400", resp.StatusCode)
	}
}

func TestSmoke_AddNode_MissingFields(t *testing.T) {
	base := startDaemon(t)

	resp := postJSON(t, base+"/nodes", map[string]any{
		"name": "gpu",
	})
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("status = %d, want 400", resp.StatusCode)
	}
}

func TestSmoke_AddNode_ThenPingFails(t *testing.T) {
	base := startDaemon(t)

	// grpc.NewClient is lazy — adding a node always succeeds.
	// The connection failure surfaces on the first real RPC (Ping).
	resp := postJSON(t, base+"/nodes", map[string]any{
		"name":  "ghost",
		"addr":  "127.0.0.1:19999",
		"token": "x",
	})
	body, _ := io.ReadAll(resp.Body)
	resp.Body.Close()

	// 201 is expected: node stored + lazy gRPC connection created.
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("expected 201 adding node, got %d: %s", resp.StatusCode, body)
	}

	// Now ping it — the actual TCP dial happens here and should fail.
	pingResp := postJSON(t, base+"/nodes/ghost/ping", nil)
	defer pingResp.Body.Close()
	if pingResp.StatusCode == http.StatusOK {
		t.Errorf("expected ping to unreachable node to fail, got 200")
	}
}

func TestSmoke_PingNode_Local(t *testing.T) {
	base := startDaemon(t)

	resp := postJSON(t, base+"/nodes/local/ping", nil)
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		t.Fatalf("status = %d, body = %s", resp.StatusCode, body)
	}
}

func TestSmoke_Broadcast_NoSessions(t *testing.T) {
	base := startDaemon(t)

	resp := postJSON(t, base+"/broadcast", map[string]any{
		"prompt": "hello",
	})
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("status = %d, want 200", resp.StatusCode)
	}

	var results []any
	json.NewDecoder(resp.Body).Decode(&results) //nolint:errcheck
	if len(results) != 0 {
		t.Errorf("expected 0 broadcast results (no sessions), got %d", len(results))
	}
}
