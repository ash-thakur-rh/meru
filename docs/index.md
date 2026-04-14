---
layout: home
title: Home
nav_order: 1
---

# Meru

Orchestrate multiple AI coding agents — **Claude Code**, **Aider**, **OpenCode**, **Goose**, and more — from a **single binary**.
{: .fs-6 .fw-300 }

[Quick Start](./quick-start){: .btn .btn-primary .fs-5 .mb-4 .mb-md-0 .mr-2 }
[View on GitHub](https://github.com/ash-thakur-rh/meru){: .btn .fs-5 .mb-4 .mb-md-0 }

---

## Zero setup. Just run it.

Conductor is a **single self-contained binary**. There is no database server to start, no config file to write, no container to run. Download it, run one command, and your web dashboard and REST API are live.

```bash
meru serve
# → Listening on :8080  (API + Web UI + WebSocket)
# → Database: ~/.meru/meru.db  (created automatically)
```

Open [http://localhost:8080](http://localhost:8080) and start spawning agents. Everything else — remote nodes, git clone, worktrees, broadcast — is opt-in and only needed when you want it.

---

## What is Conductor?

Conductor is a lightweight orchestration layer that sits between you and your AI coding agents. Spawn sessions, stream responses, broadcast prompts to many agents at once, and manage everything through a REST API, a CLI, or the built-in web dashboard.

```
┌──────────────────────────────────────────┐
│              meru  (daemon)           │
│                                          │
│  CLI / Web UI ──► REST API ──► Session  │
│                                    │     │
│                               Node Reg  │
│                              /         \ │
│                        LocalNode   GRPCNode
│                            │              │
│                      Agent adapters  meru-node
│              (claude / aider / opencode / goose)  (remote)
└──────────────────────────────────────────┘
```

---

## Key Features

| Feature | Description |
|---------|-------------|
| **Single binary** | One `meru serve` command — no config files, no extra processes |
| **Multiple agents** | Claude Code, Aider, OpenCode, Goose — plug in more with a simple interface |
| **Session lifecycle** | Spawn, send prompts, stream events, stop |
| **Multi-turn conversations** | Each adapter maintains conversation context across sends |
| **Broadcast** | Fan out a single prompt to many sessions concurrently |
| **Git clone** | Clone any repo directly from the UI — no native git needed (uses go-git) |
| **Git worktrees** | Each session gets an isolated branch and working directory |
| **Remote nodes** | Spawn agents on remote machines over authenticated gRPC |
| **Event streaming** | nd-JSON over HTTP and WebSocket for real-time output |
| **Web dashboard** | Built-in UI served directly from the daemon — no separate server |
| **Desktop notifications** | Task-done and error alerts on macOS, Linux, and Windows/WSL |
| **Persistent state** | SQLite-backed session and event history — embedded, no server required |

---

## Quick Example

```bash
# 1. Download and run (macOS ARM example — see Installation for others)
curl -fsSL https://github.com/ash-thakur-rh/meru/releases/latest/download/meru_darwin_arm64.tar.gz \
  | tar -xz && ./meru serve

# 2. Spawn a Claude Code session on your project
meru spawn claude --workspace ~/projects/myapp --name feature-bot

# 3. Send it a prompt and stream the response
meru send <session-id> "add unit tests for the auth package"

# 4. See what it did
meru logs <session-id>
```
