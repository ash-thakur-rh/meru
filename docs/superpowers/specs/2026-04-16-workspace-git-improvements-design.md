# Workspace & Git Session Improvements

**Date:** 2026-04-16
**Status:** Approved

---

## Problem Statement

Two pain points with git/isolated workspace sessions:

1. **Meaningless branch names** — worktree branches are named `meru/<full-uuid>` (e.g. `meru/550e8400-e29b-41d4-a716-446655440000`). The name is unreadable and tells the user nothing about what the session is doing.
2. **Opaque clone progress** — cloning a repo blocks synchronously, the UI shows only "Cloning…" with no progress, no live output, and no way to cancel.

---

## Part 1: Branch Naming

### Goal

Branch names should be human-readable and derived from the session name the user types, with an optional override field in the spawn dialog.

### Slug Helper

New function `SlugifyBranch(name string) string` in `internal/workspace`:

- Lowercase the input
- Replace spaces and non-alphanumeric characters (except hyphens) with hyphens
- Collapse consecutive hyphens to one
- Strip leading/trailing hyphens
- Truncate to 50 characters

Examples:
- `"Fix login bug #42"` → `"fix-login-bug-42"`
- `"Refactor auth middleware"` → `"refactor-auth-middleware"`

### SpawnConfig Change

Add `BranchName string` to `agent.SpawnConfig` (in `internal/agent/interface.go`).

### Branch Resolution (session manager)

In `internal/session/manager.go`, before calling `CreateWorktree`:

1. If `cfg.BranchName` is set, use it as-is (already validated/slugified by the UI)
2. Else if `cfg.Name` is non-empty, set `cfg.BranchName = SlugifyBranch(cfg.Name)`
3. Else fall back to `"sess-" + sessionID[:8]`

### Worktree Creation

Both local (`internal/workspace/worktree.go`) and remote (`cmd/meru-node/handler.go`) replace:

```go
branch := "meru/" + sessionID
```

with:

```go
branch := "meru/" + cfg.BranchName
```

The worktree **directory path** remains UUID-based (it is a temp path, not user-visible).

### Proto Change

Add `branch_name string` field to `SpawnRequest` in `proto/meru.proto` so the remote node receives the resolved branch name.

### UI Change

In `web/src/components/SpawnModal.tsx`:

- Add a "Branch name" text input, visible only when worktree mode is enabled
- Pre-fill it with a debounced slug of the session name as the user types
- Allow the user to override or clear it
- If cleared, the backend falls back to the session-name slug

---

## Part 2: Clone Progress (Async + SSE + Cancel)

### Goal

Replace the blocking clone call with an async job that streams live git output to the UI and supports cancellation.

### New Package: `internal/gitclone`

```
internal/gitclone/
  manager.go   — CloneJob, Manager
```

**`CloneJob`:**

```go
type CloneJob struct {
    ID     string
    done   bool
    err    error
    cancel context.CancelFunc
    lines  []string          // accumulated log lines
    pct    int               // latest parsed percentage (0-100)
    mu     sync.RWMutex
    notify chan struct{}      // closed/recreated to wake SSE subscribers
}
```

**`Manager`** wraps a `sync.RWMutex`-protected `map[string]*CloneJob`.

### Clone Subprocess

Each job runs `git clone --progress <url> <dest>` via `exec.CommandContext(ctx, "git", ...)`.

- Stderr is piped and read line by line
- Lines matching `Receiving objects:\s+(\d+)%` update `job.pct`
- Every line is appended to `job.lines` and the `notify` channel is signalled
- On exit: `job.done = true`, `job.err` set if non-zero exit

### API Changes

| Method | Path | Description |
|--------|------|-------------|
| `POST` | `/git/clone` | Start async clone, return `{"jobId": "..."}` immediately |
| `GET` | `/git/clone/{id}/stream` | SSE stream of log lines and completion event |
| `DELETE` | `/git/clone/{id}` | Cancel an in-progress clone |

**SSE event format:**

```
event: log
data: {"line": "Receiving objects:  45% (450/1000)", "pct": 45}

event: done
data: {"path": "/path/to/repo"}

event: error
data: {"message": "authentication required"}
```

**`POST /git/clone` request** (unchanged fields, same as current):

```json
{ "url": "...", "dest": "...", "node": "...", "username": "...", "password": "..." }
```

**`POST /git/clone` response:**

```json
{ "jobId": "abc123" }
```

### Frontend Changes (`web/src/components/SpawnModal.tsx`)

**Flow:**

1. User submits form with clone enabled
2. `POST /git/clone` → receive `jobId`
3. Open `EventSource("/git/clone/{jobId}/stream")`
4. Show clone progress UI:
   - Scrollable monospace log panel (auto-scroll to bottom)
   - Progress bar (driven by `pct` field in log events)
   - "Cancel" button
5. On `event: done`: close EventSource, store `path`, proceed to spawn
6. On `event: error`: show error message, allow retry
7. Cancel button: `DELETE /git/clone/{jobId}`, reset form to pre-clone state

**API client (`web/src/api.ts`):**

- `gitClone` now returns `{ jobId: string }` instead of `{ path: string }`
- Add `cancelClone(jobId: string)` calling `DELETE /git/clone/{id}`

### Cancellation

The subprocess is started with `exec.CommandContext(ctx, ...)`. `DELETE /git/clone/{id}` calls `job.cancel()`, which kills the process. The job is marked as cancelled with an appropriate error message.

### Node Interface

`GitClone` on the `Node` interface remains for the remote node path. For remote nodes, the async job runs on the **control plane** side (it shells out to `git` locally or proxies to the remote node). The SSE stream is served by the control plane API regardless.

> Note: For the remote node case, the control plane initiates the clone on behalf of the remote node via the existing gRPC `GitClone` RPC. Streaming progress from a remote node clone is out of scope for this iteration — the SSE endpoint will stream what it can from the control plane's perspective and show a spinner for remote clones.

---

## Files to Create / Modify

| File | Change |
|------|--------|
| `internal/workspace/slug.go` | New — `SlugifyBranch` helper |
| `internal/workspace/worktree.go` | Use `BranchName` instead of `sessionID` |
| `internal/agent/interface.go` | Add `BranchName` to `SpawnConfig` |
| `internal/session/manager.go` | Branch name resolution logic |
| `internal/gitclone/manager.go` | New — async clone job manager |
| `internal/api/server.go` | New routes: `POST`, `GET`, `DELETE` `/git/clone/{id}/...` |
| `proto/meru.proto` | Add `branch_name` to `SpawnRequest` |
| `cmd/meru-node/handler.go` | Use `branch_name` from proto |
| `web/src/api.ts` | Update `gitClone` return type, add `cancelClone` |
| `web/src/components/SpawnModal.tsx` | Async clone flow, SSE, progress UI |

---

## Out of Scope

- Streaming clone progress from a remote node (deferred)
- Auto-naming branch from the first agent message
- Persisting clone job state across server restarts
