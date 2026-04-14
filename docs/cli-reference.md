---
layout: default
title: CLI Reference
nav_order: 13
---

# CLI Reference
{: .no_toc }

## Table of contents
{: .no_toc .text-delta }

1. TOC
{:toc}

---

## Global flags

These flags apply to every `meru` subcommand:

| Flag | Default | Description |
|------|---------|-------------|
| `--addr` | `http://localhost:8080` | Address of the Meru daemon |

```bash
meru --addr http://myserver:9000 list
```

---

## meru serve

Start the Meru daemon.

```bash
meru serve [flags]
```

| Flag | Default | Description |
|------|---------|-------------|
| `--addr`, `-a` | `:8080` | HTTP listen address |

**Examples**

```bash
meru serve
meru serve --addr :9000
meru serve --addr 127.0.0.1:8080
```

---

## meru spawn

Spawn a new agent session.

```bash
meru spawn <agent> [flags]
```

**Arguments**

| Argument | Description |
|----------|-------------|
| `<agent>` | Agent name: `claude`, `aider`, `opencode`, `goose` |

**Flags**

| Flag | Default | Description |
|------|---------|-------------|
| `--workspace`, `-w` | Current directory | Working directory for the agent |
| `--name`, `-n` | Auto-generated | Human-readable session label |
| `--model`, `-m` | Agent default | Model to use |
| `--worktree` | `false` | Create an isolated git worktree |
| `--node` | `local` | Node to run the session on |

**Examples**

```bash
# Basic spawn
meru spawn claude --workspace ~/projects/myapp

# With all options
meru spawn claude \
  --workspace ~/projects/myapp \
  --name feature-bot \
  --model claude-opus-4-6 \
  --worktree \
  --node gpu-box
```

**Output**

```
Session spawned
  ID:        a1b2c3d4-e5f6-7890-abcd-ef1234567890
  Name:      feature-bot
  Agent:     claude
  Workspace: /home/user/projects/myapp
  Status:    idle
```

---

## meru attach

Attach to a session's interactive terminal — identical to running the agent directly in your local terminal.

```bash
meru attach <session-id>
```

All keystrokes are forwarded to the agent's PTY stdin. Output streams in real time including colours, TUI rendering, and approval prompts. The terminal resizes automatically when you resize your window.

Detach by closing the connection (Ctrl+C or closing the terminal). **The session keeps running.**

For stopped sessions the stored output is replayed then the connection closes.

**Examples**

```bash
# Attach to a running session
meru attach a1b2c3d4-...

# Spawn and immediately attach
meru spawn claude --workspace ~/projects/api
meru attach $(meru list | awk 'NR==2{print $1}')
```

> **Note:** `meru attach` requires stdin to be an interactive terminal. It cannot be used in scripts or piped commands — use `meru send` for non-interactive prompt delivery.

---

## meru send

Send a prompt to a session and stream the response.

```bash
meru send <session-id> <prompt>
```

**Arguments**

| Argument | Description |
|----------|-------------|
| `<session-id>` | ID from `meru list` or `meru spawn` |
| `<prompt>` | The prompt text to send |

**Examples**

```bash
meru send a1b2c3d4-... "add unit tests for the auth package"
meru send a1b2c3d4-... "now fix the failing test in user_test.go"
```

Events stream to stdout in real time. The command exits when the agent sends a `done` event.

---

## meru list

List all sessions — active and stopped.

```bash
meru list
```

**Output**

```
ID                                    NAME          AGENT   STATUS          NODE   WORKSPACE
a1b2c3d4-e5f6-7890-abcd-ef1234567890 feature-bot   claude  idle            local  /home/user/projects/myapp
b2c3d4e5-f6a7-8901-bcde-f01234567891 approval-bot  aider   waiting [!]     local  /home/user/projects/api
c3d4e5f6-a7b8-9012-cdef-012345678912 test-bot      goose   stopped [-]     local  /tmp/test
```

Status markers:
- `waiting [!]` — agent has paused and is asking for input; use `meru attach` to respond
- `stopped [-]` — session ended; use `meru delete` to remove or re-spawn with `meru spawn`

---

## meru logs

View a session's event history.

```bash
meru logs <session-id>
```

**Arguments**

| Argument | Description |
|----------|-------------|
| `<session-id>` | Session ID |

**Examples**

```bash
meru logs a1b2c3d4-...
```

---

## meru stop

Stop a live session or delete a stopped session record.

```bash
meru stop <session-id>
```

| Session state | Behaviour |
|---------------|-----------|
| Live | Terminates the process; marks status `stopped`; keeps record in history |
| Already stopped | Permanently deletes the record and event history |

**Examples**

```bash
meru stop a1b2c3d4-...
```

---

## meru delete

Permanently delete a stopped session record and its event history.

```bash
meru delete <session-id>
```

The session must already be stopped. To stop a live session first:

```bash
meru stop a1b2c3d4-...   # stops it, keeps record
meru delete a1b2c3d4-... # removes the record permanently
```

---

## meru broadcast

Fan out a prompt to multiple sessions simultaneously.

```bash
meru broadcast <prompt> [flags]
```

**Arguments**

| Argument | Description |
|----------|-------------|
| `<prompt>` | The prompt to send |

**Flags**

| Flag | Default | Description |
|------|---------|-------------|
| `--sessions` | (empty) | Comma-separated session IDs to target. Empty = all active idle sessions |

**Examples**

```bash
# All active sessions
meru broadcast "run the test suite and fix any failures"

# Specific sessions
meru broadcast "summarize your changes" \
  --sessions a1b2c3d4-...,b2c3d4e5-...
```

---

## meru agents

List all registered agent adapters.

```bash
meru agents
```

**Output**

```
claude, aider, opencode, goose
```

---

## meru nodes

Manage remote nodes.

### meru nodes list

```bash
meru nodes list
```

**Output**

```
NAME     ADDR                   TLS   LAST SEEN
local    (built-in)             —     —
gpu-box  gpu-box.internal:9090  no    2026-04-13 10:00:00
```

---

### meru nodes add

Register a remote node.

```bash
meru nodes add <name> [flags]
```

**Arguments**

| Argument | Description |
|----------|-------------|
| `<name>` | Unique name for this node |

**Flags**

| Flag | Required | Description |
|------|----------|-------------|
| `--addr` | **Yes** | `host:port` of the meru-node gRPC server |
| `--token` | **Yes** | Bearer token for authentication |
| `--tls` | No | Connect with TLS |

**Examples**

```bash
meru nodes add gpu-box \
  --addr gpu-box.internal:9090 \
  --token mysecrettoken

meru nodes add secure-box \
  --addr secure-box.internal:9090 \
  --token mysecrettoken \
  --tls
```

---

### meru nodes ping

Check connectivity to a node and display its capabilities.

```bash
meru nodes ping <name>
```

**Examples**

```bash
meru nodes ping gpu-box
```

**Output**

```json
{
  "name":    "gpu-box",
  "agents":  ["claude", "aider"],
  "version": "meru-node/1.0"
}
```

---

### meru nodes remove

Remove a node from the registry.

```bash
meru nodes remove <name>
```

**Examples**

```bash
meru nodes remove gpu-box
```

---

## meru-node serve

Start the remote node daemon.

```bash
meru-node serve [flags]
```

| Flag | Default | Required | Description |
|------|---------|----------|-------------|
| `--addr`, `-a` | `:9090` | No | gRPC listen address |
| `--token` | — | **Yes** | Bearer token for authentication |
| `--tls-cert` | — | No | Path to TLS certificate file |
| `--tls-key` | — | No | Path to TLS private key file |

**Examples**

```bash
meru-node serve --token mysecrettoken
meru-node serve --addr :7070 --token mysecrettoken
meru-node serve \
  --token mysecrettoken \
  --tls-cert /etc/meru/cert.pem \
  --tls-key  /etc/meru/key.pem
```
