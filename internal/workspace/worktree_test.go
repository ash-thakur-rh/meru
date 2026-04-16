package workspace_test

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"

	gogit "github.com/go-git/go-git/v6"
	"github.com/go-git/go-git/v6/plumbing/object"

	"github.com/ash-thakur-rh/meru/internal/workspace"
)

// initRepo creates a temporary git repository using go-git with one initial
// commit, so it can be used in worktree-related tests.
func initRepo(t *testing.T) string {
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

	// Write a file and stage it so the commit is non-empty
	if err := os.WriteFile(filepath.Join(dir, "README.md"), []byte("hello"), 0o644); err != nil {
		t.Fatalf("write file: %v", err)
	}
	if _, err := wt.Add("README.md"); err != nil {
		t.Fatalf("git add: %v", err)
	}
	if _, err := wt.Commit("initial commit", &gogit.CommitOptions{
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

// hasGit reports whether the native git binary is available.
func hasGit() bool {
	_, err := exec.LookPath("git")
	return err == nil
}

// --- IsGitRepo ---

func TestIsGitRepo_True(t *testing.T) {
	dir := initRepo(t)
	if !workspace.IsGitRepo(dir) {
		t.Error("expected true for a git repository")
	}
}

func TestIsGitRepo_False(t *testing.T) {
	dir := t.TempDir()
	if workspace.IsGitRepo(dir) {
		t.Error("expected false for a plain directory")
	}
}

func TestIsGitRepo_Subdirectory(t *testing.T) {
	dir := initRepo(t)
	sub := filepath.Join(dir, "subdir")
	if err := os.MkdirAll(sub, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if !workspace.IsGitRepo(sub) {
		t.Error("expected true when called from a subdirectory of a git repo")
	}
}

// --- RepoRoot ---

func TestRepoRoot_AtRoot(t *testing.T) {
	dir := initRepo(t)
	root, err := workspace.RepoRoot(dir)
	if err != nil {
		t.Fatalf("RepoRoot: %v", err)
	}
	if root != dir {
		t.Errorf("root = %q, want %q", root, dir)
	}
}

func TestRepoRoot_NotGitRepo(t *testing.T) {
	dir := t.TempDir()
	_, err := workspace.RepoRoot(dir)
	if err == nil {
		t.Error("expected error for a non-git directory")
	}
}

// --- CreateWorktree / RemoveWorktree ---
// These require the native git binary for linked-worktree support.

func TestCreateWorktree(t *testing.T) {
	if !hasGit() {
		t.Skip("native git not available")
	}

	dir := initRepo(t)
	m := workspace.New()

	wtPath, err := m.CreateWorktree(dir, "test-session", "test-branch")
	if err != nil {
		t.Fatalf("CreateWorktree: %v", err)
	}

	// The worktree directory must exist
	if _, err := os.Stat(wtPath); err != nil {
		t.Errorf("worktree path %q not found: %v", wtPath, err)
	}

	// It should be inside the expected subdirectory
	expected := filepath.Join(dir, ".meru-worktrees", "test-session")
	if wtPath != expected {
		t.Errorf("worktree path = %q, want %q", wtPath, expected)
	}
}

func TestCreateWorktree_NotGitRepo(t *testing.T) {
	dir := t.TempDir()
	m := workspace.New()

	_, err := m.CreateWorktree(dir, "sess", "my-branch")
	if err == nil {
		t.Error("expected error for non-git directory")
	}
}

func TestRemoveWorktree(t *testing.T) {
	if !hasGit() {
		t.Skip("native git not available")
	}

	dir := initRepo(t)
	m := workspace.New()

	wtPath, err := m.CreateWorktree(dir, "rm-session", "rm-branch")
	if err != nil {
		t.Fatalf("CreateWorktree: %v", err)
	}
	if _, err := os.Stat(wtPath); err != nil {
		t.Fatalf("worktree path not created: %v", err)
	}

	if err := m.RemoveWorktree(dir, "rm-session", "rm-branch"); err != nil {
		t.Fatalf("RemoveWorktree: %v", err)
	}

	// Directory should be gone (or empty) after removal
	if _, err := os.Stat(wtPath); !os.IsNotExist(err) {
		t.Errorf("worktree path %q still exists after removal", wtPath)
	}
}
