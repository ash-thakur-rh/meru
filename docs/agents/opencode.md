---
layout: default
title: OpenCode
parent: Agents
nav_order: 3
---

# OpenCode
{: .no_toc }

## Table of contents
{: .no_toc .text-delta }

1. TOC
{:toc}

---

## Overview

The `opencode` adapter wraps [OpenCode](https://opencode.ai). It runs `opencode run "<prompt>"` under a PTY so the full terminal output — tool use, progress, file edits — streams to the client in real time. Session continuity is maintained via OpenCode's built-in session naming.

| Property | Value |
|----------|-------|
| Agent name | `opencode` |
| Streaming | Yes (PTY) |
| Multi-turn | Yes |
| Tool use | Yes |
| Default model | OpenCode default |

---

## Prerequisites

Install OpenCode according to the [OpenCode documentation](https://opencode.ai).

Verify:

```bash
opencode --version
```

---

## Spawning a session

```bash
meru spawn opencode \
  --workspace ~/projects/myapp \
  --name opencode-bot
```

| Flag | Description |
|------|-------------|
| `--workspace` | Directory where OpenCode runs |
| `--name` | Human-readable label for the session |
| `--model` | Model identifier passed to OpenCode (`--model`) |

---

## Sending prompts

```bash
meru send <session-id> "add input validation to all API endpoints"
```

Output streams back in real time via a PTY, including OpenCode's tool-use display and progress output.

---

## Multi-turn conversations

The adapter passes the Meru session ID as OpenCode's `--session` flag on the first call:

```bash
opencode run "<prompt>" --session <meru-session-id> [--model <model>]
```

Subsequent calls use `--continue <session-id>` to resume the same OpenCode session, so the model remembers prior context. As long as the Meru session is alive, OpenCode continues from where it left off.

---

## How the adapter works

Each `Send` call:

1. First call: runs `opencode run "<prompt>" --session <id> [--model <model>]`
2. Subsequent calls: runs `opencode run "<prompt>" --continue <id> [--model <model>]`

Both run under a PTY. Raw terminal output is forwarded as `EventText` events as it arrives. On clean exit, `EventDone` is emitted; on non-zero exit, `EventError`.

All output is accumulated in an internal log buffer (accessible via `meru logs`).
