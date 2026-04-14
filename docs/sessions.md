---
layout: default
title: Sessions
nav_order: 6
---

# Sessions
{: .no_toc }

## Table of contents
{: .no_toc .text-delta }

1. TOC
{:toc}

---

## What is a session?

A **session** is a long-lived conversation with an agent. It has a workspace directory, a status, and a history of all events exchanged. Sessions persist across daemon restarts — you can stop the daemon, start it again, and still see all previous sessions in `meru list`.

---

## Session lifecycle

```
 spawn
   │
   ▼
 idle ──► busy ──► waiting ──► busy
   │         └──────────────────►idle
   ▼ (stop)
 stopped
```

| Status | Meaning |
|--------|---------|
| `idle` | Ready to receive a prompt |
| `busy` | Processing a prompt |
| `waiting` | Agent has paused and is awaiting user approval or input (e.g. a y/n prompt) |
| `stopped` | Terminated; no longer accepts prompts. Record is kept for history and re-spawn. |
| `error` | Entered an unrecoverable error state |

---

## Spawning a session

### CLI

```bash
meru spawn <agent> [flags]
```

| Flag | Default | Description |
|------|---------|-------------|
| `--workspace`, `-w` | Current directory | Working directory for the agent |
| `--name`, `-n` | Auto-generated | Human-readable label |
| `--model`, `-m` | Agent default | Model to use |
| `--worktree` | `false` | Create an isolated git worktree |
| `--node` | `local` | Node to run on |

```bash
# Local, auto-named
meru spawn claude --workspace ~/projects/api

# Custom name, specific model
meru spawn claude \
  --workspace ~/projects/api \
  --name api-refactor \
  --model claude-opus-4-6

# With isolated worktree
meru spawn claude \
  --workspace ~/projects/api \
  --worktree

# On a remote node
meru spawn claude \
  --workspace /home/user/projects/api \
  --node gpu-box
```

### API

```bash
curl -X POST http://localhost:8080/sessions \
  -H 'Content-Type: application/json' \
  -d '{
    "agent":     "claude",
    "name":      "api-refactor",
    "workspace": "/home/user/projects/api",
    "model":     "claude-opus-4-6",
    "worktree":  false,
    "node":      ""
  }'
```

Response (`201 Created`):

```json
{
  "id":        "a1b2c3d4-...",
  "name":      "api-refactor",
  "agent":     "claude",
  "workspace": "/home/user/projects/api",
  "status":    "idle"
}
```

---

## Listing sessions

### CLI

```bash
meru list
```

```
ID                                    NAME          AGENT   STATUS   WORKSPACE
a1b2c3d4-e5f6-...                    api-refactor  claude  idle     /home/user/projects/api
b2c3d4e5-f6a7-...                    test-bot      aider   stopped  /tmp/test
```

### API

```bash
curl http://localhost:8080/sessions
```

---

## Sending a prompt

### CLI

```bash
meru send <session-id> "your prompt here"
```

Events print to stdout in real time. The command exits when the agent finishes.

### API

```bash
curl -X POST http://localhost:8080/sessions/<id>/send \
  -H 'Content-Type: application/json' \
  -d '{"prompt": "add input validation to the registration endpoint"}'
```

The response is a newline-delimited JSON (nd-JSON) stream. Each line is an event:

```json
{"type":"text","text":"I'll start by reading the registration handler...\n","timestamp":"2026-04-13T10:00:00Z"}
{"type":"tool_use","tool_name":"read_file","tool_input":"{\"path\":\"src/auth/register.go\"}","timestamp":"2026-04-13T10:00:01Z"}
{"type":"text","text":"Here's what I'll add...\n","timestamp":"2026-04-13T10:00:02Z"}
{"type":"done","timestamp":"2026-04-13T10:00:05Z"}
```

---

## Viewing event history

### CLI

```bash
meru logs <session-id>
```

### API

```bash
curl http://localhost:8080/sessions/<id>/logs
```

Returns the full event history as a JSON array.

---

## Attaching to a session terminal

```bash
meru attach <session-id>
```

Opens a full bidirectional PTY session directly in your terminal — identical to running the agent locally. Every keystroke is forwarded to the agent's stdin and output streams in real time.

- **Detach** by pressing `Ctrl+C` or closing the terminal window. The session keeps running.
- **Resize** is handled automatically: resizing your terminal window sends the new dimensions to the agent's PTY.
- **Stopped sessions** replay their stored output then close — useful for reviewing what the agent did.
- **Approval prompts** can be answered interactively: when the agent is `waiting`, you can type `y`, `n`, or make a selection just as you would locally.

```bash
# Attach to a running session
meru attach a1b2c3d4-e5f6-...

# Useful pattern: spawn, then immediately attach
meru spawn claude --workspace ~/projects/api --name api-refactor
meru attach $(meru list --quiet | head -1)
```

---

## Real-time streaming via WebSocket

### Structured event stream

Receive JSON-encoded events as they happen:

```
ws://localhost:8080/sessions/<id>/stream
```

Each WebSocket message is a JSON-encoded event object (same shape as the nd-JSON events from `/send`).

### Bidirectional terminal (PTY bridge)

For full interactive terminal access — identical to running the agent CLI directly in your terminal:

```
ws://localhost:8080/sessions/<id>/terminal
```

- **Binary frames** carry raw PTY bytes in both directions (output → browser, keystrokes → agent)
- **Text frames** carry JSON resize events: `{"type":"resize","cols":N,"rows":N}`
- For stopped sessions the endpoint replays stored log output then closes — useful for reviewing history

This is what the web UI's xterm.js terminal uses. It lets you interact with agent approval prompts, TUI interfaces, and anything else the agent renders.

---

## Stopping a session

### CLI

```bash
meru stop <session-id>
```

### API

```bash
curl -X DELETE http://localhost:8080/sessions/<id>
```

**Behaviour depends on session state:**

| Session state | Action |
|---------------|--------|
| Live (idle/busy/waiting) | Terminates the process, marks status `stopped` in DB, cleans up git worktree |
| Already stopped | Permanently deletes the record from the database |

The session and its event history remain visible in `meru list` and `meru logs` until permanently deleted.

---

## Re-spawning a session

Stopped sessions can be re-spawned from the web UI (hover a stopped card → **re-spawn**) or by calling `POST /sessions` with the same parameters. The new session starts fresh in the same workspace.

---

## Auto-generated names

If you don't provide `--name`, Conductor generates one automatically in the format `<agent>-<8-char-uuid>` (e.g., `claude-a1b2c3d4`).

---

## Environment variables

You can inject extra environment variables into the agent process via the API:

```json
{
  "agent": "claude",
  "workspace": "/tmp/work",
  "env": {
    "DEBUG": "true",
    "CUSTOM_VAR": "value"
  }
}
```

The agent process inherits the daemon's environment plus these overrides.
