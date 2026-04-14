package aider

import (
	"context"
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

func fakeAiderScript(t *testing.T, body string) string {
	t.Helper()
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "aider"), []byte("#!/bin/sh\n"+body), 0o755); err != nil {
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

func TestSpawn_PersistentProcess(t *testing.T) {
	if !hasPTY() {
		t.Skip("PTY not available")
	}
	if _, err := exec.LookPath("sh"); err != nil {
		t.Skip("sh not available")
	}

	binDir := fakeAiderScript(t, `echo "aider ready"; cat`)
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

func TestSend_WritesToStdinAndStreams(t *testing.T) {
	if !hasPTY() {
		t.Skip("PTY not available")
	}

	binDir := fakeAiderScript(t, `
echo "ready"
while IFS= read -r line; do
  echo "aider: $line"
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

	ch, err := sess.Send(ctx, "add docs")
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

	if !strings.Contains(output.String(), "aider: add docs") {
		t.Errorf("expected echoed prompt in output, got: %q", output.String())
	}
	if !gotDone {
		t.Error("expected EventDone after inactivity")
	}
}

func TestSend_BusyRejectsSecondCall(t *testing.T) {
	if !hasPTY() {
		t.Skip("PTY not available")
	}

	binDir := fakeAiderScript(t, `echo "ready"; cat`)
	t.Setenv("PATH", binDir+":"+os.Getenv("PATH"))

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	sess, err := New().Spawn(ctx, agent.SpawnConfig{Workspace: t.TempDir(), Model: "m"})
	if err != nil {
		t.Fatal(err)
	}
	defer sess.Stop() //nolint:errcheck

	_, err = sess.Send(ctx, "first")
	if err != nil {
		t.Fatal(err)
	}
	_, err = sess.Send(ctx, "second")
	if err == nil {
		t.Error("expected error on concurrent Send")
	}
}

func TestSession_SendAfterStop(t *testing.T) {
	if !hasPTY() {
		t.Skip("PTY not available")
	}

	binDir := fakeAiderScript(t, `echo "ready"; cat`)
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
