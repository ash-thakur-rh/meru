package claude

import (
	"context"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/ash-thakur-rh/meru/internal/agent"
)

func hasPTY() bool {
	f, err := os.OpenFile("/dev/ptmx", os.O_RDWR, 0)
	if err != nil {
		return false
	}
	f.Close()
	return true
}

// fakeClaudeScript writes a shell script named "claude" into a temp dir and
// returns the dir, ready to be prepended to PATH.
func fakeClaudeScript(t *testing.T, body string) string {
	t.Helper()
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "claude"), []byte("#!/bin/sh\n"+body), 0o755); err != nil {
		t.Fatal(err)
	}
	return dir
}

func TestAdapter_Name(t *testing.T) {
	if New().Name() != AgentName {
		t.Errorf("Name() = %q, want %q", New().Name(), AgentName)
	}
}

func TestAdapter_Capabilities(t *testing.T) {
	caps := New().Capabilities()
	if !caps.Streaming {
		t.Error("expected Streaming=true")
	}
	if !caps.MultiTurn {
		t.Error("expected MultiTurn=true")
	}
}

// TestSpawn_PersistentProcess verifies that Spawn() starts an interactive
// process that stays alive until Stop() is called.
func TestSpawn_PersistentProcess(t *testing.T) {
	if !hasPTY() {
		t.Skip("PTY not available")
	}
	if _, err := exec.LookPath("sh"); err != nil {
		t.Skip("sh not available")
	}

	// Script that prints a banner then waits forever (interactive mode).
	binDir := fakeClaudeScript(t, `echo "claude ready"; cat`)
	t.Setenv("PATH", binDir+":"+os.Getenv("PATH"))

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	sess, err := New().Spawn(ctx, agent.SpawnConfig{
		Workspace: t.TempDir(),
		Model:     "test-model",
	})
	if err != nil {
		t.Fatal(err)
	}

	if sess.Status() != agent.StatusIdle {
		t.Errorf("status after Spawn = %v, want idle", sess.Status())
	}

	if err := sess.Stop(); err != nil {
		t.Fatal(err)
	}
	if sess.Status() != agent.StatusStopped {
		t.Errorf("status after Stop = %v, want stopped", sess.Status())
	}
}

// TestSpawn_Startup_LogsCaptured verifies that startup output is captured in Logs().
func TestSpawn_Startup_LogsCaptured(t *testing.T) {
	if !hasPTY() {
		t.Skip("PTY not available")
	}

	binDir := fakeClaudeScript(t, `echo "startup banner"; cat`)
	// Prepend binDir so our fake "claude" wins, but keep system paths so the
	// shell can find cat, sh, etc.
	t.Setenv("PATH", binDir+":/usr/bin:/bin:/usr/sbin:/sbin")

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	sess, err := New().Spawn(ctx, agent.SpawnConfig{Workspace: t.TempDir(), Model: "m"})
	if err != nil {
		t.Fatal(err)
	}
	defer sess.Stop() //nolint:errcheck

	logBytes, _ := io.ReadAll(sess.Logs())
	if !strings.Contains(string(logBytes), "startup banner") {
		t.Errorf("startup output missing from Logs(): %q", string(logBytes))
	}
}

// TestSend_WritesToStdinAndStreams verifies that Send() writes to the
// process stdin and forwards PTY output as EventText events.
func TestSend_WritesToStdinAndStreams(t *testing.T) {
	if !hasPTY() {
		t.Skip("PTY not available")
	}

	// The fake claude: prints banner, then echoes each line of input prefixed with "resp: ".
	binDir := fakeClaudeScript(t, `
echo "ready"
while IFS= read -r line; do
  echo "resp: $line"
done
`)
	t.Setenv("PATH", binDir+":"+os.Getenv("PATH"))

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	sess, err := New().Spawn(ctx, agent.SpawnConfig{Workspace: t.TempDir(), Model: "m"})
	if err != nil {
		t.Fatal(err)
	}
	defer sess.Stop() //nolint:errcheck

	ch, err := sess.Send(ctx, "hello")
	if err != nil {
		t.Fatal(err)
	}

	var output strings.Builder
	var gotDone bool
	for ev := range ch {
		switch ev.Type {
		case agent.EventText:
			output.WriteString(ev.Text)
		case agent.EventDone:
			gotDone = true
		case agent.EventError:
			t.Fatalf("unexpected error: %s", ev.Error)
		}
	}

	if !strings.Contains(output.String(), "resp: hello") {
		t.Errorf("expected 'resp: hello' in output, got: %q", output.String())
	}
	if !gotDone {
		t.Error("expected EventDone after inactivity")
	}
}

// TestSend_BusyRejectsSecondCall verifies that a second Send() while one is
// in flight returns an error.
func TestSend_BusyRejectsSecondCall(t *testing.T) {
	if !hasPTY() {
		t.Skip("PTY not available")
	}

	binDir := fakeClaudeScript(t, `echo "ready"; cat`)
	t.Setenv("PATH", binDir+":"+os.Getenv("PATH"))

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	sess, err := New().Spawn(ctx, agent.SpawnConfig{Workspace: t.TempDir(), Model: "m"})
	if err != nil {
		t.Fatal(err)
	}
	defer sess.Stop() //nolint:errcheck

	// First Send — will stay busy since cat never produces output → inactivity timer fires.
	_, err = sess.Send(ctx, "first")
	if err != nil {
		t.Fatal(err)
	}

	// Second Send immediately — should be rejected.
	_, err = sess.Send(ctx, "second")
	if err == nil {
		t.Error("expected error on concurrent Send")
	}
}

// TestSend_ProcessDeath emits EventError when the process exits during a Send.
func TestSend_ProcessDeath(t *testing.T) {
	if !hasPTY() {
		t.Skip("PTY not available")
	}

	// Process survives startup (waits for first line), then exits immediately.
	binDir := fakeClaudeScript(t, `echo "ready"; read line; echo "dying"; exit 1`)
	t.Setenv("PATH", binDir+":"+os.Getenv("PATH"))

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	sess, err := New().Spawn(ctx, agent.SpawnConfig{Workspace: t.TempDir(), Model: "m"})
	if err != nil {
		t.Fatal(err)
	}
	defer sess.Stop() //nolint:errcheck

	ch, err := sess.Send(ctx, "hello")
	if err != nil {
		t.Fatal(err)
	}

	var gotErrorOrDone bool
	for ev := range ch {
		if ev.Type == agent.EventError || ev.Type == agent.EventDone {
			gotErrorOrDone = true
		}
	}
	if !gotErrorOrDone {
		t.Error("expected EventError or EventDone after process death")
	}
}

func TestSession_SendAfterStop(t *testing.T) {
	if !hasPTY() {
		t.Skip("PTY not available")
	}

	binDir := fakeClaudeScript(t, `echo "ready"; cat`)
	t.Setenv("PATH", binDir+":"+os.Getenv("PATH"))

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	sess, err := New().Spawn(ctx, agent.SpawnConfig{Workspace: t.TempDir(), Model: "m"})
	if err != nil {
		t.Fatal(err)
	}
	sess.Stop() //nolint:errcheck

	_, err = sess.Send(ctx, "hello")
	if err == nil {
		t.Error("expected error when sending to stopped session")
	}
}

func TestSession_Workspace(t *testing.T) {
	if !hasPTY() {
		t.Skip("PTY not available")
	}

	binDir := fakeClaudeScript(t, `echo "ready"; cat`)
	t.Setenv("PATH", binDir+":"+os.Getenv("PATH"))

	workspace := t.TempDir()
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	sess, err := New().Spawn(ctx, agent.SpawnConfig{Workspace: workspace, Model: "m"})
	if err != nil {
		t.Fatal(err)
	}
	defer sess.Stop() //nolint:errcheck

	if sess.Workspace() != workspace {
		t.Errorf("Workspace() = %q, want %q", sess.Workspace(), workspace)
	}
	if sess.AgentName() != AgentName {
		t.Errorf("AgentName() = %q, want %q", sess.AgentName(), AgentName)
	}
}
