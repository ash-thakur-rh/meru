# Workspace & Git Session Improvements Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Replace UUID-based worktree branch names with human-readable slugs derived from session name, and replace the blocking git clone call with an async job that streams live progress to the UI with cancel support.

**Architecture:** Part 1 threads a `BranchName` field from the spawn API through SpawnConfig → session manager (slug resolution) → worktree creation → proto → remote node handler. Part 2 introduces an in-process `gitclone.Manager` that runs `git clone --progress` as a subprocess, streams stderr line-by-line over SSE, and supports context cancellation.

**Tech Stack:** Go 1.22+, chi router, protoc/protoc-gen-go for proto, React/TypeScript with native `EventSource` for SSE

---

## File Map

| File | Action | Purpose |
|------|--------|---------|
| `internal/workspace/slug.go` | Create | `SlugifyBranch` helper |
| `internal/workspace/slug_test.go` | Create | Unit tests for slug helper |
| `internal/workspace/worktree.go` | Modify | Accept `worktreeID` + `branchSlug` separately |
| `internal/workspace/worktree_test.go` | Modify | Update calls to match new signature |
| `internal/agent/interface.go` | Modify | Add `BranchName string` to `SpawnConfig` |
| `internal/session/manager.go` | Modify | Branch resolution logic; fix worktreeID/sessionID bug in entry |
| `proto/meru.proto` | Modify | Add `branch_name` field to `SpawnRequest` |
| `internal/proto/meru.pb.go` | Regenerate | protoc output |
| `internal/proto/meru_grpc.pb.go` | Regenerate | protoc output |
| `cmd/meru-node/handler.go` | Modify | Use `req.BranchName` for worktree branch |
| `internal/api/server.go` | Modify | Add `branch_name` to `spawnRequest`; new clone routes; wire `cloneManager` |
| `internal/gitclone/manager.go` | Create | Async clone job manager |
| `internal/gitclone/manager_test.go` | Create | Unit tests for clone manager |
| `web/src/api.ts` | Modify | `SpawnParams.branchName`; `gitClone` → `{jobId}`; add `cancelClone` |
| `web/src/components/SpawnModal.tsx` | Modify | Branch name field; async clone SSE flow with progress UI |

---

## Task 1: SlugifyBranch helper

**Files:**
- Create: `internal/workspace/slug.go`
- Create: `internal/workspace/slug_test.go`

- [ ] **Step 1: Write the failing tests**

```go
// internal/workspace/slug_test.go
package workspace_test

import (
	"testing"

	"github.com/ash-thakur-rh/meru/internal/workspace"
)

func TestSlugifyBranch(t *testing.T) {
	cases := []struct {
		input string
		want  string
	}{
		{"Fix login bug", "fix-login-bug"},
		{"Refactor auth middleware", "refactor-auth-middleware"},
		{"Fix login bug #42", "fix-login-bug-42"},
		{"  leading and trailing  ", "leading-and-trailing"},
		{"multiple   spaces", "multiple-spaces"},
		{"UPPERCASE", "uppercase"},
		{"special!@#$%chars", "special-chars"},
		{"", ""},
		{"---hyphens---", "hyphens"},
		{
			"this-is-a-very-long-branch-name-that-exceeds-the-fifty-character-limit-by-a-lot",
			"this-is-a-very-long-branch-name-that-exceeds-the",
		},
	}

	for _, tc := range cases {
		got := workspace.SlugifyBranch(tc.input)
		if got != tc.want {
			t.Errorf("SlugifyBranch(%q) = %q, want %q", tc.input, got, tc.want)
		}
	}
}
```

- [ ] **Step 2: Run tests — verify they fail**

```
go test ./internal/workspace/... -run TestSlugifyBranch -v
```
Expected: FAIL — `SlugifyBranch` undefined

- [ ] **Step 3: Implement SlugifyBranch**

```go
// internal/workspace/slug.go
package workspace

import (
	"regexp"
	"strings"
)

var nonAlnum = regexp.MustCompile(`[^a-z0-9]+`)

// SlugifyBranch converts a human-readable name into a git-safe branch slug.
// Result is lowercased, non-alphanumeric runs replaced with a single hyphen,
// leading/trailing hyphens stripped, and truncated to 50 characters.
// Returns "" if the input contains no usable characters.
func SlugifyBranch(name string) string {
	s := strings.ToLower(name)
	s = nonAlnum.ReplaceAllString(s, "-")
	s = strings.Trim(s, "-")
	if len(s) > 50 {
		s = s[:50]
		s = strings.TrimRight(s, "-")
	}
	return s
}
```

- [ ] **Step 4: Run tests — verify they pass**

```
go test ./internal/workspace/... -run TestSlugifyBranch -v
```
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add internal/workspace/slug.go internal/workspace/slug_test.go
git commit -m "feat: add SlugifyBranch helper for readable git branch names"
```

---

## Task 2: Update worktree.go — separate worktreeID from branchSlug

Current: `CreateWorktree(repoPath, sessionID string)` uses `sessionID` for both path and branch name.
New: `CreateWorktree(repoPath, worktreeID, branchSlug string)` — `worktreeID` drives the path, `branchSlug` drives the branch.

**Files:**
- Modify: `internal/workspace/worktree.go`
- Modify: `internal/workspace/worktree_test.go`

- [ ] **Step 1: Update the tests to the new signature**

In `internal/workspace/worktree_test.go`, update all calls:

```go
// TestCreateWorktree — line 117
wtPath, err := m.CreateWorktree(dir, "test-session", "test-branch")

// TestCreateWorktree_NotGitRepo — line 138
_, err := m.CreateWorktree(dir, "sess", "my-branch")

// TestRemoveWorktree — line 152
wtPath, err := m.CreateWorktree(dir, "rm-session", "rm-branch")
// ...
if err := m.RemoveWorktree(dir, "rm-session", "rm-branch"); err != nil {
```

- [ ] **Step 2: Run tests — verify they fail**

```
go test ./internal/workspace/... -run TestCreate -run TestRemove -v
```
Expected: compile error — wrong number of arguments

- [ ] **Step 3: Update CreateWorktree and RemoveWorktree signatures**

Replace `internal/workspace/worktree.go` lines 30–71:

```go
// CreateWorktree creates a new git worktree for the session and returns its path.
// worktreeID is used for the worktree directory name (must be unique).
// branchSlug is the human-readable part of the git branch name (meru/<branchSlug>).
func (m *Manager) CreateWorktree(repoPath, worktreeID, branchSlug string) (string, error) {
	if !IsGitRepo(repoPath) {
		return "", fmt.Errorf("%s is not a git repository", repoPath)
	}

	worktreesRoot := filepath.Join(repoPath, worktreeDir)
	if err := os.MkdirAll(worktreesRoot, 0o755); err != nil {
		return "", fmt.Errorf("create worktrees dir: %w", err)
	}

	worktreePath := filepath.Join(worktreesRoot, worktreeID)
	branch := "meru/" + branchSlug

	out, err := git(repoPath, "worktree", "add", "-b", branch, worktreePath, "HEAD")
	if err != nil {
		return "", fmt.Errorf("git worktree add: %w\n%s", err, out)
	}

	return worktreePath, nil
}

// RemoveWorktree deletes the worktree and its branch.
// worktreeID must match the ID used in CreateWorktree.
// branchSlug must match the slug used in CreateWorktree.
func (m *Manager) RemoveWorktree(repoPath, worktreeID, branchSlug string) error {
	worktreePath := filepath.Join(repoPath, worktreeDir, worktreeID)
	branch := "meru/" + branchSlug

	out, err := git(repoPath, "worktree", "remove", "--force", worktreePath)
	if err != nil {
		_ = out
	}

	out, err = git(repoPath, "branch", "-D", branch)
	if err != nil {
		_ = out
	}

	return nil
}
```

- [ ] **Step 4: Run tests — verify they pass**

```
go test ./internal/workspace/... -v
```
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add internal/workspace/worktree.go internal/workspace/worktree_test.go
git commit -m "feat: separate worktreeID from branchSlug in worktree manager"
```

---

## Task 3: Add BranchName to SpawnConfig and fix session manager

**Files:**
- Modify: `internal/agent/interface.go`
- Modify: `internal/session/manager.go`

- [ ] **Step 1: Add BranchName to SpawnConfig**

In `internal/agent/interface.go`, update `SpawnConfig` (lines 47–54):

```go
// SpawnConfig holds parameters for starting an agent session.
type SpawnConfig struct {
	Name       string            // human-readable session name
	Workspace  string            // working directory for the agent
	Model      string            // model to use (if agent supports selection)
	Env        map[string]string // extra env vars
	Worktree   bool              // create an isolated git worktree for this session (local node only)
	NodeName   string            // target node; empty means "local"
	BranchName string            // git branch slug for the worktree; auto-derived from Name if empty
}
```

- [ ] **Step 2: Update the entry struct in session manager**

In `internal/session/manager.go`, replace the `entry` struct (lines 32–37):

```go
// entry pairs a live Session with its last-known status.
type entry struct {
	sess           agent.Session
	repoRoot       string // non-empty if a worktree was created
	worktreeID     string // ID (UUID) used as the worktree directory name
	worktreeBranch string // branch slug used for the git branch (meru/<worktreeBranch>)
}
```

- [ ] **Step 3: Add branch resolution + fix worktreeID bug in Spawn**

In `internal/session/manager.go`, replace the worktree block (lines 76–134). The full updated `Spawn` method:

```go
func (m *Manager) Spawn(ctx context.Context, agentName string, cfg agent.SpawnConfig) (agent.Session, error) {
	nodeName := cfg.NodeName
	if nodeName == "" {
		nodeName = node.LocalNodeName
	}

	n, err := node.Get(nodeName)
	if err != nil {
		return nil, err
	}

	if cfg.Name == "" {
		cfg.Name = agentName + "-" + uuid.New().String()[:8]
	}

	// Resolve the branch name: explicit > slug of session name > short ID fallback.
	if cfg.BranchName == "" {
		cfg.BranchName = workspace.SlugifyBranch(cfg.Name)
	}
	if cfg.BranchName == "" {
		cfg.BranchName = "sess-" + uuid.New().String()[:8]
	}

	// For local nodes, create the worktree here so the manager can clean it up on stop.
	// Remote nodes handle worktree creation themselves (via the Worktree field in SpawnRequest).
	var repoRoot, worktreeID string
	if nodeName == node.LocalNodeName && cfg.Worktree && workspace.IsGitRepo(cfg.Workspace) {
		root, err := workspace.RepoRoot(cfg.Workspace)
		if err != nil {
			return nil, fmt.Errorf("find repo root: %w", err)
		}
		worktreeID = uuid.New().String()
		wtPath, err := m.wt.CreateWorktree(root, worktreeID, cfg.BranchName)
		if err != nil {
			return nil, fmt.Errorf("create worktree: %w", err)
		}
		cfg.Workspace = wtPath
		repoRoot = root
		defer func() {
			if repoRoot != "" {
				_ = m.wt.RemoveWorktree(root, worktreeID, cfg.BranchName)
			}
		}()
	}

	// Allocate the session ID on the control plane so we can reconnect later.
	sessionID := uuid.New().String()

	slog.Info("spawning session",
		"agent", agentName,
		"name", cfg.Name,
		"workspace", cfg.Workspace,
		"node", nodeName,
		"worktree", cfg.Worktree,
		"branch", cfg.BranchName,
	)

	sess, err := n.Spawn(ctx, sessionID, agentName, cfg)
	if err != nil {
		slog.Error("spawn failed", "agent", agentName, "node", nodeName, "error", err)
		return nil, fmt.Errorf("spawn %s on node %s: %w", agentName, nodeName, err)
	}

	slog.Info("session spawned", "session", sess.ID(), "name", sess.Name(), "agent", agentName)

	now := time.Now()
	if err := m.store.CreateSession(store.Session{
		ID:        sess.ID(),
		Name:      sess.Name(),
		Agent:     agentName,
		Workspace: sess.Workspace(),
		NodeName:  nodeName,
		Status:    string(agent.StatusIdle),
		CreatedAt: now,
		UpdatedAt: now,
	}); err != nil {
		return nil, fmt.Errorf("persist session: %w", err)
	}

	m.mu.Lock()
	m.sessions[sess.ID()] = &entry{
		sess:           sess,
		repoRoot:       repoRoot,
		worktreeID:     worktreeID,
		worktreeBranch: cfg.BranchName,
	}
	repoRoot = "" // prevent defer from cleaning up on success
	m.mu.Unlock()

	return sess, nil
}
```

- [ ] **Step 4: Fix Stop to use worktreeID and worktreeBranch**

In `internal/session/manager.go`, update `Stop` (lines 204–220). Replace the cleanup line:

```go
// old (line 217):
// _ = m.wt.RemoveWorktree(e.repoRoot, e.sessionID)

// new:
if e.repoRoot != "" {
    _ = m.wt.RemoveWorktree(e.repoRoot, e.worktreeID, e.worktreeBranch)
}
```

The full updated `Stop`:

```go
func (m *Manager) Stop(id string) error {
	m.mu.Lock()
	e, ok := m.sessions[id]
	if ok {
		delete(m.sessions, id)
	}
	m.mu.Unlock()
	if !ok {
		return fmt.Errorf("session %s not found", id)
	}
	slog.Info("stopping session", "session", id, "name", e.sess.Name())
	_ = m.store.UpdateSessionStatus(id, string(agent.StatusStopped))
	if e.repoRoot != "" {
		_ = m.wt.RemoveWorktree(e.repoRoot, e.worktreeID, e.worktreeBranch)
	}
	return e.sess.Stop()
}
```

- [ ] **Step 5: Build to check for compile errors**

```
go build ./...
```
Expected: success

- [ ] **Step 6: Run tests**

```
go test ./internal/workspace/... ./internal/session/... -v
```
Expected: PASS

- [ ] **Step 7: Commit**

```bash
git add internal/agent/interface.go internal/session/manager.go
git commit -m "feat: thread BranchName through SpawnConfig and fix worktree cleanup bug"
```

---

## Task 4: Proto — add branch_name to SpawnRequest

**Files:**
- Modify: `proto/meru.proto`
- Regenerate: `internal/proto/meru.pb.go`, `internal/proto/meru_grpc.pb.go`

- [ ] **Step 1: Add branch_name field to SpawnRequest**

In `proto/meru.proto`, update `SpawnRequest` (lines 53–61):

```protobuf
message SpawnRequest {
  string session_id = 1;  // assigned by control plane
  string agent      = 2;
  string name       = 3;
  string workspace  = 4;
  string model      = 5;
  map<string, string> env = 6;
  bool   worktree   = 7;  // create an isolated git worktree on the node before spawning
  string branch_name = 8; // branch slug for the worktree; meru/<branch_name> will be the branch
}
```

- [ ] **Step 2: Regenerate proto bindings**

```bash
protoc \
  --go_out=internal/proto --go_opt=paths=source_relative \
  --go-grpc_out=internal/proto --go-grpc_opt=paths=source_relative \
  -I proto proto/meru.proto
```
Expected: `internal/proto/meru.pb.go` and `internal/proto/meru_grpc.pb.go` updated

- [ ] **Step 3: Verify build**

```
go build ./...
```
Expected: success

- [ ] **Step 4: Commit**

```bash
git add proto/meru.proto internal/proto/meru.pb.go internal/proto/meru_grpc.pb.go
git commit -m "feat: add branch_name to SpawnRequest proto"
```

---

## Task 5: Remote node handler — use BranchName for worktree branch

**Files:**
- Modify: `cmd/meru-node/handler.go`

- [ ] **Step 1: Replace the hardcoded branch name in Spawn**

In `cmd/meru-node/handler.go`, update the worktree creation block (lines 66–82):

```go
// If a git worktree is requested, create one on this node before spawning.
if req.Worktree && workspace != "" {
    _, gitErr := gogit.PlainOpen(workspace)
    if gitErr == nil {
        worktreeDir := filepath.Join(workspace, ".meru-worktrees")
        if err := os.MkdirAll(worktreeDir, 0o755); err != nil {
            return nil, status.Errorf(codes.Internal, "create worktrees dir: %v", err)
        }
        // Use the session ID as the unique directory name; use BranchName for the branch.
        worktreePath := filepath.Join(worktreeDir, req.SessionId)
        branchSlug := req.BranchName
        if branchSlug == "" {
            branchSlug = req.SessionId[:8]
        }
        branch := "meru/" + branchSlug
        addCmd := exec.Command("git", "-C", workspace,
            "worktree", "add", "-b", branch, worktreePath, "HEAD")
        if out, err := addCmd.CombinedOutput(); err != nil {
            return nil, status.Errorf(codes.Internal, "git worktree add: %v\n%s", err, out)
        }
        workspace = worktreePath
    }
}
```

- [ ] **Step 2: Build**

```
go build ./cmd/meru-node/...
```
Expected: success

- [ ] **Step 3: Commit**

```bash
git add cmd/meru-node/handler.go
git commit -m "feat: use BranchName from proto for remote node worktree branch"
```

---

## Task 6: API server — pass BranchName through spawn endpoint

**Files:**
- Modify: `internal/api/server.go`

- [ ] **Step 1: Add branch_name to spawnRequest and pass it through**

In `internal/api/server.go`, update `spawnRequest` (lines 91–99) and `handleSpawn` (lines 115–122):

```go
// POST /sessions
type spawnRequest struct {
	Agent      string            `json:"agent"`
	Name       string            `json:"name"`
	Workspace  string            `json:"workspace"`
	Model      string            `json:"model"`
	Env        map[string]string `json:"env"`
	Worktree   bool              `json:"worktree"`
	Node       string            `json:"node"`
	BranchName string            `json:"branch_name"`
}
```

In `handleSpawn`, update the `SpawnConfig` construction:

```go
sess, err := s.mgr.Spawn(r.Context(), req.Agent, agent.SpawnConfig{
    Name:       req.Name,
    Workspace:  req.Workspace,
    Model:      req.Model,
    Env:        req.Env,
    Worktree:   req.Worktree,
    NodeName:   req.Node,
    BranchName: req.BranchName,
})
```

- [ ] **Step 2: Build and run tests**

```
go build ./... && go test ./internal/api/... -v
```
Expected: PASS

- [ ] **Step 3: Commit**

```bash
git add internal/api/server.go
git commit -m "feat: accept branch_name in spawn API and forward to session manager"
```

---

## Task 7: Frontend — branch name field in SpawnModal

**Files:**
- Modify: `web/src/api.ts`
- Modify: `web/src/components/SpawnModal.tsx`

- [ ] **Step 1: Add branchName to SpawnParams in api.ts**

In `web/src/api.ts`, update `SpawnParams` (lines 52–59):

```typescript
export interface SpawnParams {
  agent: string;
  name?: string;
  workspace?: string;
  model?: string;
  worktree?: boolean;
  node?: string;
  branch_name?: string;
}
```

- [ ] **Step 2: Add branch name state and auto-derive logic to SpawnModal**

In `web/src/components/SpawnModal.tsx`:

Add import at top (update existing import line):
```typescript
import { useState, useEffect, useRef, type FormEvent } from "react";
```

Add branch name state after the `worktree` state (after line 33):
```typescript
const [branchName, setBranchName] = useState("");
const [branchEdited, setBranchEdited] = useState(false);
```

Add a slug helper inside the component (before the `submit` function):
```typescript
function slugify(name: string): string {
  return name
    .toLowerCase()
    .replace(/[^a-z0-9]+/g, "-")
    .replace(/^-+|-+$/g, "")
    .slice(0, 50)
    .replace(/-+$/, "");
}
```

Update the session name `onChange` handler (line 160 area) to auto-derive the branch:
```typescript
onChange={(e) => {
  set("name", e.target.value);
  if (!branchEdited) {
    setBranchName(slugify(e.target.value));
  }
}}
```

- [ ] **Step 3: Add the branch name input field (visible only when worktree is on)**

In `SpawnModal.tsx`, add this block immediately after the worktree checkbox section (after line 334):

```tsx
{worktree && (
  <label className="block mb-4">
    <span className="text-xs text-slate-500 dark:text-slate-400 mb-1 block">
      Branch name{" "}
      <span className="text-slate-400 dark:text-slate-600">(optional)</span>
    </span>
    <input
      type="text"
      value={branchName}
      onChange={(e) => {
        setBranchName(e.target.value);
        setBranchEdited(e.target.value !== "");
      }}
      placeholder={form.name ? slugify(form.name) : "auto-derived from session name"}
      className={inputCls}
    />
    <p className="text-xs text-slate-400 dark:text-slate-500 mt-1">
      Branch will be created as{" "}
      <code className="font-mono">meru/{branchName || slugify(form.name ?? "") || "…"}</code>
    </p>
  </label>
)}
```

- [ ] **Step 4: Pass branch_name in the spawn call**

In `SpawnModal.tsx`, update the `spawnSession` call in `submit` (lines 84–89):

```typescript
await api.spawnSession({
  ...form,
  workspace,
  model: model || undefined,
  worktree,
  branch_name: worktree && branchName ? branchName : undefined,
});
```

- [ ] **Step 5: Verify UI builds**

```
cd web && npm run build
```
Expected: success with no TypeScript errors

- [ ] **Step 6: Commit**

```bash
git add web/src/api.ts web/src/components/SpawnModal.tsx
git commit -m "feat: branch name field in spawn modal, auto-derived from session name"
```

---

## Task 8: gitclone package — async clone job manager

**Files:**
- Create: `internal/gitclone/manager.go`
- Create: `internal/gitclone/manager_test.go`

- [ ] **Step 1: Write failing tests**

```go
// internal/gitclone/manager_test.go
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

	// Wait for job to finish (max 10s)
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
	// StartRemote wraps an arbitrary blocking function — cancel it immediately.
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
```

- [ ] **Step 2: Run tests — verify they fail**

```
go test ./internal/gitclone/... -v
```
Expected: FAIL — package not found

- [ ] **Step 3: Implement gitclone.Manager**

```go
// internal/gitclone/manager.go
package gitclone

import (
	"bufio"
	"context"
	"fmt"
	"os/exec"
	"regexp"
	"strconv"
	"strings"
	"sync"

	"github.com/google/uuid"
)

var pctRe = regexp.MustCompile(`(?i)(?:Receiving|Resolving) objects:\s+(\d+)%`)

// CloneJob tracks an in-progress (or finished) git clone operation.
type CloneJob struct {
	ID string

	mu     sync.RWMutex
	lines  []string
	pct    int
	done   bool
	path   string // set on success
	err    error
	cancel context.CancelFunc
	notify chan struct{} // closed when state changes; always replaced, never reused
}

// Snapshot returns a consistent read of the job's current state.
func (j *CloneJob) Snapshot() (lines []string, pct int, done bool, path string, err error) {
	j.mu.RLock()
	defer j.mu.RUnlock()
	cp := make([]string, len(j.lines))
	copy(cp, j.lines)
	return cp, j.pct, j.done, j.path, j.err
}

// Wait returns a channel that is closed whenever new lines arrive or the job
// finishes. Call Snapshot after receiving from this channel to get updated state.
func (j *CloneJob) Wait() <-chan struct{} {
	j.mu.RLock()
	ch := j.notify
	j.mu.RUnlock()
	return ch
}

func (j *CloneJob) appendLine(line string) {
	pct := 0
	if m := pctRe.FindStringSubmatch(line); m != nil {
		if v, err := strconv.Atoi(m[1]); err == nil {
			pct = v
		}
	}

	j.mu.Lock()
	j.lines = append(j.lines, line)
	if pct > j.pct {
		j.pct = pct
	}
	old := j.notify
	j.notify = make(chan struct{})
	j.mu.Unlock()

	close(old)
}

func (j *CloneJob) finish(path string, err error) {
	j.mu.Lock()
	j.done = true
	j.path = path
	j.err = err
	old := j.notify
	j.notify = make(chan struct{})
	j.mu.Unlock()

	close(old)
}

// Manager owns a set of CloneJobs.
type Manager struct {
	mu   sync.RWMutex
	jobs map[string]*CloneJob
}

// New returns a ready Manager.
func New() *Manager {
	return &Manager{jobs: make(map[string]*CloneJob)}
}

// Get returns the job with the given ID.
func (m *Manager) Get(id string) (*CloneJob, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	j, ok := m.jobs[id]
	return j, ok
}

// Cancel cancels a running job. Returns false if the job is not found.
func (m *Manager) Cancel(id string) bool {
	m.mu.RLock()
	j, ok := m.jobs[id]
	m.mu.RUnlock()
	if !ok {
		return false
	}
	j.cancel()
	return true
}

// StartLocal starts an async local git clone using the system git binary.
// Returns the job ID immediately; progress can be streamed via CloneJob.Wait.
func (m *Manager) StartLocal(url, dest, username, password string) string {
	ctx, cancel := context.WithCancel(context.Background())
	j := &CloneJob{
		ID:     uuid.New().String(),
		cancel: cancel,
		notify: make(chan struct{}),
	}
	m.mu.Lock()
	m.jobs[j.ID] = j
	m.mu.Unlock()

	go func() {
		args := []string{"clone", "--progress", url}
		if dest != "" {
			args = append(args, dest)
		}
		cmd := exec.CommandContext(ctx, "git", args...)
		if username != "" || password != "" {
			// Embed credentials in URL to avoid interactive prompts.
			// go-git used BasicAuth; git CLI needs them in the URL.
			cmd.Env = append(cmd.Environ(),
				fmt.Sprintf("GIT_TERMINAL_PROMPT=0"),
				fmt.Sprintf("GIT_ASKPASS=echo"),
			)
			// Rewrite URL to embed credentials.
			rewritten := embedCreds(url, username, password)
			args[len(args)-func() int {
				if dest != "" {
					return 2
				}
				return 1
			}()] = rewritten
			cmd = exec.CommandContext(ctx, "git", args...)
		}

		// git clone --progress writes progress to stderr, result to stdout.
		stderr, err := cmd.StderrPipe()
		if err != nil {
			j.finish("", fmt.Errorf("pipe: %w", err))
			return
		}

		if err := cmd.Start(); err != nil {
			j.finish("", fmt.Errorf("start: %w", err))
			return
		}

		scanner := bufio.NewScanner(stderr)
		// git progress lines end with \r, not \n; split on \r or \n.
		scanner.Split(scanCRLF)
		for scanner.Scan() {
			line := strings.TrimSpace(scanner.Text())
			if line != "" {
				j.appendLine(line)
			}
		}

		if err := cmd.Wait(); err != nil {
			if ctx.Err() != nil {
				j.finish("", fmt.Errorf("cancelled"))
			} else {
				j.finish("", fmt.Errorf("git clone: %w", err))
			}
			return
		}

		clonedPath := dest
		if clonedPath == "" {
			clonedPath = repoName(url)
		}
		j.finish(clonedPath, nil)
	}()

	return j.ID
}

// StartRemote wraps a blocking clone function (e.g. gRPC GitClone) as an async job.
// fn receives a cancellable context and returns the cloned path on success.
func (m *Manager) StartRemote(fn func(ctx context.Context) (string, error)) string {
	ctx, cancel := context.WithCancel(context.Background())
	j := &CloneJob{
		ID:     uuid.New().String(),
		cancel: cancel,
		notify: make(chan struct{}),
	}
	m.mu.Lock()
	m.jobs[j.ID] = j
	m.mu.Unlock()

	j.appendLine("Cloning on remote node…")

	go func() {
		path, err := fn(ctx)
		j.finish(path, err)
	}()

	return j.ID
}

// scanCRLF splits on \r or \n so we capture git's carriage-return progress lines.
func scanCRLF(data []byte, atEOF bool) (advance int, token []byte, err error) {
	for i, b := range data {
		if b == '\r' || b == '\n' {
			return i + 1, data[:i], nil
		}
	}
	if atEOF && len(data) > 0 {
		return len(data), data, nil
	}
	return 0, nil, nil
}

// embedCreds rewrites an https URL to include username:password.
func embedCreds(rawURL, username, password string) string {
	if !strings.HasPrefix(rawURL, "https://") {
		return rawURL
	}
	rest := strings.TrimPrefix(rawURL, "https://")
	return fmt.Sprintf("https://%s:%s@%s", username, password, rest)
}

// repoName extracts a repository directory name from a git URL.
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
```

- [ ] **Step 4: Run tests**

```
go test ./internal/gitclone/... -v -timeout 30s
```
Expected: PASS (the invalid-URL test may take a few seconds to time out on git)

- [ ] **Step 5: Commit**

```bash
git add internal/gitclone/manager.go internal/gitclone/manager_test.go
git commit -m "feat: async git clone manager with SSE-friendly progress notifications"
```

---

## Task 9: API — async clone routes

Wire the three new clone endpoints into the API server.

**Files:**
- Modify: `internal/api/server.go`

- [ ] **Step 1: Add cloneManager field to Server and update New()**

In `internal/api/server.go`, add the import and update the `Server` struct and `New`:

Add to imports:
```go
"github.com/ash-thakur-rh/meru/internal/gitclone"
```

Update `Server` struct (after line 25):
```go
type Server struct {
	mgr      *session.Manager
	store    *store.Store
	clones   *gitclone.Manager
	upgrader websocket.Upgrader
}
```

Update `New`:
```go
func New(mgr *session.Manager, st *store.Store) *Server {
	return &Server{
		mgr:   mgr,
		store: st,
		clones: gitclone.New(),
		upgrader: websocket.Upgrader{
			CheckOrigin: func(r *http.Request) bool { return true },
		},
	}
}
```

- [ ] **Step 2: Register the three new routes**

In `Handler()`, replace the single git/clone route (line 60):
```go
// old:
r.With(jsonContentType).Post("/git/clone", s.handleGitClone)

// new:
r.Route("/git/clone", func(r chi.Router) {
    r.With(jsonContentType).Post("/", s.handleGitCloneStart)
    r.Get("/{id}/stream", s.handleGitCloneStream)
    r.Delete("/{id}", s.handleGitCloneCancel)
})
```

- [ ] **Step 3: Implement handleGitCloneStart**

Replace the existing `handleGitClone` method (lines 445–471) with:

```go
// POST /git/clone — starts an async clone and returns a job ID immediately.
func (s *Server) handleGitCloneStart(w http.ResponseWriter, r *http.Request) {
	var req gitCloneRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON: "+err.Error())
		return
	}
	if req.URL == "" {
		writeError(w, http.StatusBadRequest, "url is required")
		return
	}
	if req.NodeName == "" {
		req.NodeName = node.LocalNodeName
	}

	n, err := node.Get(req.NodeName)
	if err != nil {
		writeError(w, http.StatusNotFound, "node not found: "+err.Error())
		return
	}

	var jobID string
	if req.NodeName == node.LocalNodeName {
		jobID = s.clones.StartLocal(req.URL, req.Dest, req.Username, req.Password)
	} else {
		// Remote node: wrap gRPC call as an async job (no live progress).
		captured := req
		capturedNode := n
		jobID = s.clones.StartRemote(func(ctx context.Context) (string, error) {
			return capturedNode.GitClone(ctx, captured.URL, captured.Dest, captured.Username, captured.Password)
		})
	}

	writeJSON(w, http.StatusAccepted, map[string]string{"jobId": jobID})
}
```

Add the `"context"` import if not already present.

- [ ] **Step 4: Implement handleGitCloneStream (SSE)**

```go
// GET /git/clone/{id}/stream — SSE stream of clone progress.
func (s *Server) handleGitCloneStream(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	job, ok := s.clones.Get(id)
	if !ok {
		writeError(w, http.StatusNotFound, "clone job not found")
		return
	}

	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("X-Accel-Buffering", "no")
	flusher, _ := w.(http.Flusher)

	sent := 0 // number of lines already sent to this client

	sendEvent := func(event, data string) {
		fmt.Fprintf(w, "event: %s\ndata: %s\n\n", event, data)
		if flusher != nil {
			flusher.Flush()
		}
	}

	for {
		lines, pct, done, path, jobErr := job.Snapshot()

		// Send any new lines since last flush.
		for ; sent < len(lines); sent++ {
			payload, _ := json.Marshal(map[string]any{"line": lines[sent], "pct": pct})
			sendEvent("log", string(payload))
		}

		if done {
			if jobErr != nil {
				payload, _ := json.Marshal(map[string]string{"message": jobErr.Error()})
				sendEvent("error", string(payload))
			} else {
				payload, _ := json.Marshal(map[string]string{"path": path})
				sendEvent("done", string(payload))
			}
			return
		}

		// Wait for next update or client disconnect.
		select {
		case <-job.Wait():
			// loop again to read new state
		case <-r.Context().Done():
			return
		}
	}
}
```

- [ ] **Step 5: Implement handleGitCloneCancel**

```go
// DELETE /git/clone/{id} — cancel an in-progress clone.
func (s *Server) handleGitCloneCancel(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	if !s.clones.Cancel(id) {
		writeError(w, http.StatusNotFound, "clone job not found")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
```

- [ ] **Step 6: Build and run API tests**

```
go build ./... && go test ./internal/api/... -v
```
Expected: PASS

- [ ] **Step 7: Commit**

```bash
git add internal/api/server.go
git commit -m "feat: async git clone API with SSE stream and cancel endpoints"
```

---

## Task 10: Frontend — async clone flow with SSE progress UI

**Files:**
- Modify: `web/src/api.ts`
- Modify: `web/src/components/SpawnModal.tsx`

- [ ] **Step 1: Update api.ts — gitClone returns jobId, add cancelClone**

In `web/src/api.ts`, replace the `gitClone` entry (lines 91–97):

```typescript
gitClone: (p: {
  url: string;
  dest?: string;
  node?: string;
  username?: string;
  password?: string;
}) => request<{ jobId: string }>("POST", "/git/clone/", p),

cancelClone: (jobId: string) =>
  request<void>("DELETE", `/git/clone/${jobId}`),

cloneStream: (jobId: string): EventSource =>
  new EventSource(`/git/clone/${jobId}/stream`),
```

- [ ] **Step 2: Replace the clone state and submit logic in SpawnModal**

Replace the clone-related state in `SpawnModal.tsx` (lines 24–38) with:

```typescript
// Git clone state
const [cloneEnabled, setCloneEnabled] = useState(false);
const [gitURL, setGitURL] = useState("");
const [gitDest, setGitDest] = useState("");
const [gitUsername, setGitUsername] = useState("");
const [gitPassword, setGitPassword] = useState("");
const [showCreds, setShowCreds] = useState(false);

// Async clone progress state
const [cloneJobId, setCloneJobId] = useState<string | null>(null);
const [cloneLines, setCloneLines] = useState<string[]>([]);
const [clonePct, setClonePct] = useState(0);
const [cloneError, setCloneError] = useState("");
const cloneESRef = useRef<EventSource | null>(null);

// Worktree state
const [worktree, setWorktree] = useState(false);
const [branchName, setBranchName] = useState("");
const [branchEdited, setBranchEdited] = useState(false);

// Submission state
const [loading, setLoading] = useState(false);
const [error, setError] = useState("");
```

Replace the `submit` function's Step 1 clone block:

```typescript
async function submit(e: FormEvent) {
  e.preventDefault();
  setError("");
  setCloneError("");

  let workspace = form.workspace ?? "";

  // Step 1: clone if requested
  if (cloneEnabled) {
    if (!gitURL.trim()) {
      setCloneError("Git URL is required when cloning is enabled.");
      return;
    }

    try {
      const { jobId } = await api.gitClone({
        url: gitURL.trim(),
        dest: gitDest.trim() || undefined,
        node: form.node || undefined,
        username: gitUsername || undefined,
        password: gitPassword || undefined,
      });

      setCloneJobId(jobId);
      setCloneLines([]);
      setClonePct(0);

      workspace = await new Promise<string>((resolve, reject) => {
        const es = api.cloneStream(jobId);
        cloneESRef.current = es;

        es.addEventListener("log", (ev) => {
          const data = JSON.parse(ev.data) as { line: string; pct: number };
          setCloneLines((prev) => [...prev, data.line]);
          if (data.pct > 0) setClonePct(data.pct);
        });

        es.addEventListener("done", (ev) => {
          const data = JSON.parse(ev.data) as { path: string };
          es.close();
          cloneESRef.current = null;
          setCloneJobId(null);
          resolve(data.path);
        });

        es.addEventListener("error", (ev) => {
          const data = JSON.parse((ev as MessageEvent).data ?? "{}") as { message?: string };
          es.close();
          cloneESRef.current = null;
          setCloneJobId(null);
          reject(new Error(data.message ?? "Clone failed"));
        });
      });
    } catch (err) {
      setCloneError((err as Error).message);
      return;
    }
  }

  // Step 2: spawn
  setLoading(true);
  try {
    await api.spawnSession({
      ...form,
      workspace,
      model: model || undefined,
      worktree,
      branch_name: worktree && branchName ? branchName : undefined,
    });
    onSpawned();
    onClose();
  } catch (err) {
    setError((err as Error).message);
  } finally {
    setLoading(false);
  }
}
```

Add a cancel handler:
```typescript
function cancelClone() {
  if (cloneJobId) {
    api.cancelClone(cloneJobId).catch(() => {});
    cloneESRef.current?.close();
    cloneESRef.current = null;
    setCloneJobId(null);
    setCloneLines([]);
    setClonePct(0);
  }
}
```

- [ ] **Step 3: Replace the clone progress UI in the render**

Update `isBusy` and `busyLabel` (lines 102–103):
```typescript
const cloning = cloneJobId !== null;
const isBusy = cloning || loading;
const busyLabel = loading ? "Starting agent…" : "Spawn";
```

Add the clone progress panel (rendered when `cloneJobId` is set), inserted just before the submit buttons block:

```tsx
{cloneJobId && (
  <div className="mb-4 border border-slate-200 dark:border-slate-700 rounded-lg p-3">
    <div className="flex items-center justify-between mb-2">
      <span className="text-xs font-medium text-slate-600 dark:text-slate-400">
        Cloning repository…
      </span>
      <button
        type="button"
        onClick={cancelClone}
        className="text-xs text-red-500 hover:text-red-700 dark:text-red-400"
      >
        Cancel
      </button>
    </div>
    {clonePct > 0 && (
      <div className="w-full bg-slate-200 dark:bg-slate-700 rounded-full h-1.5 mb-2">
        <div
          className="bg-purple-500 h-1.5 rounded-full transition-all duration-300"
          style={{ width: `${clonePct}%` }}
        />
      </div>
    )}
    <div className="bg-slate-950 rounded p-2 h-28 overflow-y-auto font-mono text-xs text-slate-300 space-y-0.5">
      {cloneLines.map((line, i) => (
        <div key={i}>{line}</div>
      ))}
    </div>
    {cloneError && (
      <p className="text-red-500 dark:text-red-400 text-xs mt-2">{cloneError}</p>
    )}
  </div>
)}
```

Remove the old `cloneError` display inside the cloneEnabled section and the old `cloning` state (already replaced above).

- [ ] **Step 4: Build**

```
cd web && npm run build
```
Expected: success, no TypeScript errors

- [ ] **Step 5: Commit**

```bash
git add web/src/api.ts web/src/components/SpawnModal.tsx
git commit -m "feat: async clone progress UI with SSE log stream, progress bar, and cancel"
```

---

## Task 11: Final integration check and push

- [ ] **Step 1: Run all Go tests**

```
go test ./... -timeout 60s
```
Expected: PASS

- [ ] **Step 2: Run UI build**

```
cd web && npm run build
```
Expected: success

- [ ] **Step 3: Push the feature branch**

```bash
git push -u origin feat/workspace-git-improvements
```

---

## Self-Review

**Spec coverage check:**

| Spec requirement | Task |
|-----------------|------|
| `SlugifyBranch` helper | Task 1 |
| `BranchName` in `SpawnConfig` | Task 3 |
| Branch resolution (explicit > slug > fallback) | Task 3 |
| `CreateWorktree` uses branchSlug, not sessionID | Task 2 |
| `RemoveWorktree` uses correct IDs | Task 2 + 3 |
| Proto `branch_name` field | Task 4 |
| Remote node uses `BranchName` | Task 5 |
| API `branch_name` field in spawn request | Task 6 |
| UI branch name field, auto-derive from session name | Task 7 |
| `POST /git/clone` → `{jobId}` | Task 9 |
| `GET /git/clone/{id}/stream` SSE | Task 9 |
| `DELETE /git/clone/{id}` cancel | Task 9 |
| Progress bar from parsed `%` | Task 10 |
| Live log panel | Task 10 |
| Cancel button in UI | Task 10 |
| Remote node clone wrapped as async job | Task 9 |
| Remote node streaming out of scope | ✓ noted in Task 9 |

**Placeholder scan:** No TBDs. All code blocks are complete.

**Type consistency check:**
- `CloneJob.Snapshot()` → `(lines []string, pct int, done bool, path string, err error)` — consistent across manager_test.go and server.go
- `Manager.StartLocal/StartRemote` return `string` (jobID) — consistent with API handler usage
- `branchName` state in SpawnModal flows correctly into `branch_name` in SpawnParams and API request body
- `worktreeID`/`worktreeBranch` in `entry` struct match usage in `Stop` and `Spawn`
