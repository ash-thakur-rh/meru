package node

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	gogit "github.com/go-git/go-git/v6"
	githttp "github.com/go-git/go-git/v6/plumbing/transport/http"
	gitssh "github.com/go-git/go-git/v6/plumbing/transport/ssh"

	"github.com/ash-thakur-rh/meru/internal/agent"
)

// LocalNode implements Node by spawning agents directly on this machine.
type LocalNode struct{}

func NewLocalNode() *LocalNode { return &LocalNode{} }

func (l *LocalNode) Name() string { return LocalNodeName }

func (l *LocalNode) Ping(ctx context.Context) (*Info, error) {
	hostname, _ := os.Hostname()
	return &Info{
		Name:    LocalNodeName,
		Agents:  agent.List(),
		Version: "local",
		Addr:    "",
		TLS:     false,
		// hostname reported so the UI can display it
		Token: hostname,
	}, nil
}

// Spawn delegates directly to the registered agent adapter.
// sessionID is passed in cfg.Name if no name was set; the adapter uses it.
func (l *LocalNode) Spawn(ctx context.Context, sessionID string, agentName string, cfg agent.SpawnConfig) (agent.Session, error) {
	a, err := agent.Get(agentName)
	if err != nil {
		return nil, err
	}
	return a.Spawn(ctx, cfg)
}

func (l *LocalNode) ListDir(_ context.Context, path string) (*DirListing, error) {
	if path == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			path = "/"
		} else {
			path = home
		}
	}

	abs, err := filepath.Abs(path)
	if err != nil {
		return nil, err
	}

	rawEntries, err := os.ReadDir(abs)
	if err != nil {
		return nil, err
	}

	entries := make([]DirEntry, 0, len(rawEntries))
	for _, e := range rawEntries {
		// Skip hidden files/dirs (dot-prefixed)
		if len(e.Name()) > 0 && e.Name()[0] == '.' {
			continue
		}
		entries = append(entries, DirEntry{
			Name:  e.Name(),
			Path:  filepath.Join(abs, e.Name()),
			IsDir: e.IsDir(),
		})
	}

	// Directories first, then files; both groups sorted alphabetically
	sort.Slice(entries, func(i, j int) bool {
		if entries[i].IsDir != entries[j].IsDir {
			return entries[i].IsDir
		}
		return entries[i].Name < entries[j].Name
	})

	parent := ""
	if abs != filepath.Dir(abs) {
		parent = filepath.Dir(abs)
	}

	return &DirListing{Path: abs, Parent: parent, Entries: entries}, nil
}

func (l *LocalNode) GitClone(ctx context.Context, rawURL, dest, username, password string) (string, error) {
	if dest == "" {
		home, _ := os.UserHomeDir()
		dest = filepath.Join(home, "meru-workspaces", repoName(rawURL))
	}

	opts := &gogit.CloneOptions{
		URL: rawURL,
	}

	switch {
	case username != "" || password != "":
		opts.Auth = &githttp.BasicAuth{Username: username, Password: password}
	case isSSHURL(rawURL):
		if auth, err := gitssh.NewSSHAgentAuth("git"); err == nil {
			opts.Auth = auth
		}
	}

	if _, err := gogit.PlainCloneContext(ctx, dest, opts); err != nil {
		return "", fmt.Errorf("git clone: %w", err)
	}
	return dest, nil
}

// isSSHURL returns true for SCP-style (git@host:org/repo) and ssh:// URLs.
func isSSHURL(rawURL string) bool {
	return strings.HasPrefix(rawURL, "git@") || strings.HasPrefix(rawURL, "ssh://")
}

// repoName extracts the repository name from a git URL.
// e.g. "https://github.com/org/repo.git" → "repo"
func repoName(rawURL string) string {
	base := strings.TrimSuffix(rawURL, ".git")
	for _, sep := range []string{"/", ":"} {
		if i := strings.LastIndex(base, sep); i >= 0 {
			base = base[i+1:]
		}
	}
	if base == "" {
		return "repo"
	}
	return base
}

func (l *LocalNode) Close() error { return nil }
