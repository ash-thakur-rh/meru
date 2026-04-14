---
layout: default
title: Claude Code
parent: Agents
nav_order: 1
---

# Claude Code
{: .no_toc }

## Table of contents
{: .no_toc .text-delta }

1. TOC
{:toc}

---

## Overview

The `claude` adapter wraps the [Claude Code CLI](https://docs.anthropic.com/en/docs/claude-code). It runs Claude under a PTY so the full terminal output — colours, progress indicators, tool-use animations — streams to the client in real time.

| Property | Value |
|----------|-------|
| Agent name | `claude` |
| Streaming | Yes (PTY) |
| Multi-turn | Yes |
| Tool use | Yes |
| Default model | `claude-sonnet-4-6` |

---

## Prerequisites

Install the Claude Code CLI:

```bash
npm install -g @anthropic-ai/claude-code
```

Authenticate with your Anthropic API key:

```bash
claude auth login
```

Verify the installation:

```bash
claude --version
```

---

## Spawning a session

```bash
meru spawn claude \
  --workspace ~/projects/myapp \
  --name feature-bot \
  --model claude-opus-4-6
```

| Flag | Description |
|------|-------------|
| `--workspace` | Directory where Claude Code runs. It reads your project files from here. |
| `--name` | Human-readable label for the session |
| `--model` | Claude model to use (see [models](#models)) |
| `--worktree` | Create an isolated git worktree for this session |
| `--node` | Run on a remote node instead of locally |

---

## Sending prompts

```bash
meru send <session-id> "refactor the database layer to use transactions"
```

Output streams back in real time via a PTY. You see text as Claude types it, tool calls as it reads files and runs commands, and a final `done` event when it finishes.

---

## Models

| Model ID | Description |
|----------|-------------|
| `claude-opus-4-6` | Most capable, slower |
| `claude-sonnet-4-6` | Balanced — **default** |
| `claude-haiku-4-5-20251001` | Fastest, lightest |

Pass the model with `--model`:

```bash
meru spawn claude --workspace . --model claude-opus-4-6
```

Or in the API:

```json
{
  "agent": "claude",
  "workspace": "/path/to/project",
  "model": "claude-opus-4-6"
}
```

---

## Interactive terminal

The web UI opens a bidirectional terminal that feels exactly like running `claude` in your own terminal:

- All output (colours, progress spinners, tool-use animations, approval prompts) streams live
- You can type directly in the browser terminal — keystrokes go straight to Claude's PTY stdin
- Resize the browser window and the PTY resizes accordingly
- If Claude asks for approval (`[y/n]` etc.) the session status changes to **waiting** (orange badge in the dashboard) — click through to the session and type your answer

---

## Multi-turn conversations

Claude Code runs once in interactive mode for the entire session lifetime. Every prompt typed in the terminal (or sent via `meru send`) is part of the same ongoing conversation — Claude retains context across turns exactly as it does in your local terminal.

```bash
# These two calls are part of the same Claude session:
meru send abc123 "add logging to all HTTP handlers"
meru send abc123 "now write tests for those handlers"
```

---

## Event types

When using the programmatic `send` API (`POST /sessions/:id/send`):

| Type | Description |
|------|-------------|
| `text` | A raw chunk of PTY output (may contain ANSI escape codes) |
| `done` | Inactivity timeout — Claude has gone quiet (≥ 2 s of silence) |
| `error` | Agent process exited unexpectedly |

For the terminal WebSocket (`GET /sessions/:id/terminal`), raw PTY bytes are streamed directly with no event envelope.

---

## How the adapter works

1. `Spawn` runs `claude [--model <model>]` in interactive mode under a PTY using [creack/pty](https://github.com/creack/pty). The process lives until `Stop()` is called.
2. A `readLoop` goroutine reads PTY output in 4 KB chunks, writes each to the log buffer, and fans it out to all active subscribers.
3. `waitStartup` subscribes to raw output and waits for 1 s of silence *after* receiving at least one byte — this prevents a false "ready" on slow process starts.
4. `WriteInput` writes raw bytes to PTY stdin (used by the terminal WebSocket to forward keystrokes). It also clears `waiting` status back to `busy` when the user types.
5. `SubscribeRaw` / `ResizePTY` implement the optional `PTYSession` interface consumed by `handleTerminal` in the API server.
