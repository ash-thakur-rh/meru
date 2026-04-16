package gitclone_test

import (
	"context"
	"os/exec"
	"strings"
	"testing"
	"time"

	"github.com/ash-thakur-rh/meru/internal/gitclone"
)

func hasGit() bool {
	_, err := exec.LookPath("git")
	return err == nil
}

func TestManager_StartLocal_InvalidURL(t *testing.T) {
	if !hasGit() {
		t.Skip("git not available")
	}

	m := gitclone.New()
	jobID := m.StartLocal("not-a-valid-url", t.TempDir(), "", "")

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	job, ok := m.Get(jobID)
	if !ok {
		t.Fatal("job not found")
	}

	for {
		lines, _, done, _, jobErr := job.Snapshot()
		if done {
			if jobErr == nil {
				t.Error("expected error for invalid URL, got nil")
			}
			_ = lines
			return
		}
		select {
		case <-job.Wait():
		case <-ctx.Done():
			t.Fatal("timed out waiting for clone to fail")
		}
	}
}

func TestManager_Cancel(t *testing.T) {
	m := gitclone.New()
	jobID := m.StartRemote(func(ctx context.Context) (string, error) {
		<-ctx.Done()
		return "", ctx.Err()
	})

	job, ok := m.Get(jobID)
	if !ok {
		t.Fatal("job not found")
	}

	m.Cancel(jobID)

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	for {
		_, _, done, _, _ := job.Snapshot()
		if done {
			return
		}
		select {
		case <-job.Wait():
		case <-ctx.Done():
			t.Fatal("timed out waiting for cancel to propagate")
		}
	}
}

func TestManager_LogLines(t *testing.T) {
	m := gitclone.New()
	jobID := m.StartRemote(func(ctx context.Context) (string, error) {
		return "/fake/path", nil
	})

	job, ok := m.Get(jobID)
	if !ok {
		t.Fatal("job not found")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	for {
		lines, _, done, path, err := job.Snapshot()
		if done {
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if path != "/fake/path" {
				t.Errorf("path = %q, want /fake/path", path)
			}
			_ = strings.Join(lines, "\n")
			return
		}
		select {
		case <-job.Wait():
		case <-ctx.Done():
			t.Fatal("timed out")
		}
	}
}
