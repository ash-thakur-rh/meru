---
layout: default
title: Goose
parent: Agents
nav_order: 4
---

# Goose
{: .no_toc }

## Table of contents
{: .no_toc .text-delta }

1. TOC
{:toc}

---

## Overview

The `goose` adapter wraps [Goose](https://block.github.io/goose) by Block. Goose is a local, extensible AI coding agent. The adapter runs Goose under a PTY so the full terminal output streams in real time. Session continuity is maintained via Goose's built-in named sessions.

| Property | Value |
|----------|-------|
| Agent name | `goose` |
| Streaming | Yes (PTY) |
| Multi-turn | Yes |
| Tool use | Yes |
| Default model | Goose default |

---

## Prerequisites

Install Goose according to the [Goose documentation](https://block.github.io/goose).

Verify:

```bash
goose --version
```

---

## Spawning a session

```bash
meru spawn goose \
  --workspace ~/projects/myapp \
  --name goose-bot \
  --model gpt-4o
```

| Flag | Description |
|------|-------------|
| `--workspace` | Directory where Goose runs |
| `--name` | Human-readable label for the session |
| `--model` | Model identifier passed to Goose (`--model`) |

---

## Sending prompts

```bash
meru send <session-id> "profile the application and identify bottlenecks"
```

Output streams back in real time via a PTY, including Goose's tool-use display and progress output.

---

## Multi-turn conversations

The adapter passes the Meru session ID as Goose's `--name` flag on the first invocation:

```bash
goose run --text "<prompt>" --name <meru-session-id> [--model <model>]
```

Subsequent calls reuse the same `--name`, so Goose resumes its session and remembers prior context. As long as the Meru session is alive, Goose continues from where it left off.

---

## How the adapter works

Each `Send` call:

1. Runs `goose run --text "<prompt>" --name <session-id> [--model <model>]` under a PTY
2. Reads raw bytes from the PTY master in a loop, forwarding each chunk as an `EventText` event
3. When the PTY closes (process exits): emits `EventDone` on clean exit, `EventError` on non-zero exit
4. All output is accumulated in an internal log buffer (accessible via `meru logs`)
