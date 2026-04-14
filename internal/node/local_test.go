package node_test

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	gogit "github.com/go-git/go-git/v6"
	"github.com/go-git/go-git/v6/plumbing/object"

	"github.com/ash-thakur-rh/meru/internal/node"
)

// initLocalRepo creates a temporary git repository with one initial commit,
// suitable for use as a clone source in tests.
func initLocalRepo(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()

	repo, err := gogit.PlainInit(dir, false)
	if err != nil {
		t.Fatalf("git init: %v", err)
	}

	wt, err := repo.Worktree()
	if err != nil {
		t.Fatalf("worktree: %v", err)
	}

	if err := os.WriteFile(filepath.Join(dir, "hello.txt"), []byte("hello"), 0o644); err != nil {
		t.Fatalf("write file: %v", err)
	}
	if _, err := wt.Add("hello.txt"); err != nil {
		t.Fatalf("git add: %v", err)
	}
	if _, err := wt.Commit("init", &gogit.CommitOptions{
		Author: &object.Signature{
			Name:  "Test",
			Email: "test@example.com",
			When:  time.Now(),
		},
	}); err != nil {
		t.Fatalf("git commit: %v", err)
	}

	return dir
}

// --- ListDir ---

func TestListDir_DefaultsToHome(t *testing.T) {
	n := node.NewLocalNode()
	listing, err := n.ListDir(context.Background(), "")
	if err != nil {
		t.Fatalf("ListDir: %v", err)
	}
	home, _ := os.UserHomeDir()
	if listing.Path != home {
		t.Errorf("path = %q, want home dir %q", listing.Path, home)
	}
}

func TestListDir_KnownDirectory(t *testing.T) {
	dir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dir, "subdir"), 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "file.txt"), []byte("x"), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}

	n := node.NewLocalNode()
	listing, err := n.ListDir(context.Background(), dir)
	if err != nil {
		t.Fatalf("ListDir: %v", err)
	}
	if listing.Path != dir {
		t.Errorf("path = %q, want %q", listing.Path, dir)
	}
	if len(listing.Entries) != 2 {
		t.Errorf("entries = %d, want 2", len(listing.Entries))
	}
	// Directories sorted before files
	if !listing.Entries[0].IsDir {
		t.Error("expected first entry to be a directory")
	}
	if listing.Entries[0].Name != "subdir" {
		t.Errorf("first entry name = %q, want subdir", listing.Entries[0].Name)
	}
}

func TestListDir_SkipsHiddenEntries(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, ".hidden"), []byte("x"), 0o644); err != nil {
		t.Fatalf("write hidden: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "visible.txt"), []byte("x"), 0o644); err != nil {
		t.Fatalf("write visible: %v", err)
	}

	n := node.NewLocalNode()
	listing, err := n.ListDir(context.Background(), dir)
	if err != nil {
		t.Fatalf("ListDir: %v", err)
	}
	if len(listing.Entries) != 1 {
		t.Errorf("entries = %d, want 1 (hidden file must be excluded)", len(listing.Entries))
	}
	if listing.Entries[0].Name == ".hidden" {
		t.Error("hidden file should not appear in listing")
	}
}

func TestListDir_ParentPath(t *testing.T) {
	dir := t.TempDir()
	n := node.NewLocalNode()
	listing, err := n.ListDir(context.Background(), dir)
	if err != nil {
		t.Fatalf("ListDir: %v", err)
	}
	if listing.Parent != filepath.Dir(dir) {
		t.Errorf("parent = %q, want %q", listing.Parent, filepath.Dir(dir))
	}
}

func TestListDir_InvalidPath(t *testing.T) {
	n := node.NewLocalNode()
	_, err := n.ListDir(context.Background(), "/no/such/path/conductor-xyz")
	if err == nil {
		t.Error("expected error for non-existent path")
	}
}

// --- GitClone ---

func TestGitClone_LocalSource(t *testing.T) {
	src := initLocalRepo(t)
	dest := filepath.Join(t.TempDir(), "clone")

	n := node.NewLocalNode()
	path, err := n.GitClone(context.Background(), src, dest, "", "")
	if err != nil {
		t.Fatalf("GitClone: %v", err)
	}
	if path != dest {
		t.Errorf("returned path = %q, want %q", path, dest)
	}
	if _, err := os.Stat(filepath.Join(dest, "hello.txt")); err != nil {
		t.Errorf("hello.txt not found in clone: %v", err)
	}
}

func TestGitClone_ContentsMatch(t *testing.T) {
	src := initLocalRepo(t)
	dest := filepath.Join(t.TempDir(), "clone")

	n := node.NewLocalNode()
	if _, err := n.GitClone(context.Background(), src, dest, "", ""); err != nil {
		t.Fatalf("GitClone: %v", err)
	}

	got, err := os.ReadFile(filepath.Join(dest, "hello.txt"))
	if err != nil {
		t.Fatalf("read cloned file: %v", err)
	}
	if string(got) != "hello" {
		t.Errorf("file content = %q, want hello", got)
	}
}

func TestGitClone_InvalidSource(t *testing.T) {
	n := node.NewLocalNode()
	dest := filepath.Join(t.TempDir(), "clone")
	_, err := n.GitClone(context.Background(), "/no/such/repo/conductor-xyz", dest, "", "")
	if err == nil {
		t.Error("expected error for non-existent source repository")
	}
}
