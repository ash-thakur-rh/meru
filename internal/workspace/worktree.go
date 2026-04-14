// Package workspace manages isolated git worktrees for agent sessions.
//
// When a session is spawned with --worktree, meru:
//  1. Creates a new git worktree branch: meru/<session-id>
//  2. Checks it out into <base-repo>/.meru-worktrees/<session-id>
//  3. Returns that path as the agent's workspace
//
// On session stop, the worktree is removed.
package workspace

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	gogit "github.com/go-git/go-git/v6"
)

const worktreeDir = ".meru-worktrees"

// Manager handles worktree lifecycle.
type Manager struct{}

func New() *Manager { return &Manager{} }

// CreateWorktree creates a new git worktree for the session and returns its path.
// repoPath must be the root of a git repository.
func (m *Manager) CreateWorktree(repoPath, sessionID string) (string, error) {
	if !IsGitRepo(repoPath) {
		return "", fmt.Errorf("%s is not a git repository", repoPath)
	}

	worktreesRoot := filepath.Join(repoPath, worktreeDir)
	if err := os.MkdirAll(worktreesRoot, 0o755); err != nil {
		return "", fmt.Errorf("create worktrees dir: %w", err)
	}

	worktreePath := filepath.Join(worktreesRoot, sessionID)
	branch := "meru/" + sessionID

	// Create orphan branch from current HEAD, then add worktree
	out, err := git(repoPath, "worktree", "add", "-b", branch, worktreePath, "HEAD")
	if err != nil {
		return "", fmt.Errorf("git worktree add: %w\n%s", err, out)
	}

	return worktreePath, nil
}

// RemoveWorktree deletes the worktree and its branch.
func (m *Manager) RemoveWorktree(repoPath, sessionID string) error {
	worktreePath := filepath.Join(repoPath, worktreeDir, sessionID)
	branch := "meru/" + sessionID

	// Force-remove the worktree
	out, err := git(repoPath, "worktree", "remove", "--force", worktreePath)
	if err != nil {
		// Non-fatal: worktree might already be gone
		_ = out
	}

	// Delete the branch
	out, err = git(repoPath, "branch", "-D", branch)
	if err != nil {
		_ = out
	}

	return nil
}

// ListWorktrees returns paths of all meru-managed worktrees in a repo.
func (m *Manager) ListWorktrees(repoPath string) ([]string, error) {
	out, err := git(repoPath, "worktree", "list", "--porcelain")
	if err != nil {
		return nil, err
	}

	var paths []string
	worktreesRoot := filepath.Join(repoPath, worktreeDir)
	for _, line := range strings.Split(string(out), "\n") {
		if strings.HasPrefix(line, "worktree ") {
			p := strings.TrimPrefix(line, "worktree ")
			if strings.HasPrefix(p, worktreesRoot) {
				paths = append(paths, p)
			}
		}
	}
	return paths, nil
}

// IsGitRepo reports whether path is inside a git repository.
func IsGitRepo(path string) bool {
	_, err := gogit.PlainOpenWithOptions(path, &gogit.PlainOpenOptions{DetectDotGit: true})
	return err == nil
}

// RepoRoot returns the top-level directory of the git repo containing path.
func RepoRoot(path string) (string, error) {
	repo, err := gogit.PlainOpenWithOptions(path, &gogit.PlainOpenOptions{DetectDotGit: true})
	if err != nil {
		return "", fmt.Errorf("%s is not a git repository", path)
	}
	wt, err := repo.Worktree()
	if err != nil {
		return "", err
	}
	return wt.Filesystem.Root(), nil
}

func git(dir string, args ...string) ([]byte, error) {
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	out, err := cmd.CombinedOutput()
	if err != nil {
		return out, fmt.Errorf("git %s: %w", strings.Join(args, " "), err)
	}
	return out, nil
}
