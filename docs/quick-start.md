---
layout: default
title: Quick Start
nav_order: 3
---

# Quick Start
{: .no_toc }

Up and running in under two minutes.
{: .fs-6 .fw-300 }

## Table of contents
{: .no_toc .text-delta }

1. TOC
{:toc}

---

## Step 1 — Download and run

Meru is a single binary. Download it, run it, and you're done with setup.

```bash
# macOS (Apple Silicon)
curl -fsSL https://github.com/ash-thakur-rh/meru/releases/latest/download/meru_darwin_arm64.tar.gz \
  | tar -xz && sudo mv meru meru-node /usr/local/bin/

# macOS (Intel)
curl -fsSL https://github.com/ash-thakur-rh/meru/releases/latest/download/meru_darwin_x86_64.tar.gz \
  | tar -xz && sudo mv meru meru-node /usr/local/bin/

# Linux (x86-64)
curl -fsSL https://github.com/ash-thakur-rh/meru/releases/latest/download/meru_linux_x86_64.tar.gz \
  | tar -xz && sudo mv meru meru-node /usr/local/bin/
```

Each archive contains both `meru` (control plane) and `meru-node` (remote node daemon).

> **Windows?** Download `meru_windows_x86_64.zip` from the [Releases page](https://github.com/ash-thakur-rh/meru/releases) and place both `.exe` files on your `%PATH%`.

See [Installation](./installation) for the full list of platforms and a build-from-source option.

---

## Step 2 — Start the daemon

```bash
meru serve
```

You'll see:

```
meru listening on :8080
  API: http://localhost:8080
  UI:  http://localhost:8080
  DB:  /Users/you/.meru/meru.db
```

**That's all the setup there is.** No config file needed, no database server to start — the SQLite database is created automatically on first run. Leave this terminal open; all other commands and the web UI talk to this daemon.

---

## Step 3 — Open the web dashboard

Navigate to [http://localhost:8080](http://localhost:8080). The Meru dashboard shows your sessions and lets you spawn new ones without touching the CLI.

---

## Step 4 — Install an agent CLI

Meru orchestrates agent CLIs you already have installed. Install at least one:

| Agent | Install |
|-------|---------|
| **Claude Code** (recommended) | `npm install -g @anthropic-ai/claude-code` |
| Aider | `pip install aider-chat` |
| OpenCode | [opencode.ai](https://opencode.ai) |
| Goose | [block.github.io/goose](https://block.github.io/goose) |

You only need the ones you plan to use — the daemon starts fine without any of them.

---

## Step 5 — Spawn your first session

**Via the web UI:** Click **New Session**, pick an agent, choose a workspace directory, and click **Spawn**.

**Via the CLI:**

```bash
meru spawn claude \
  --workspace ~/projects/myapp \
  --name my-first-bot
```

Output:

```
Session spawned
  ID:        a1b2c3d4-...
  Name:      my-first-bot
  Agent:     claude
  Workspace: /Users/you/projects/myapp
  Status:    idle
```

---

## Step 6 — Send a prompt

```bash
meru send a1b2c3d4-... "explain what this codebase does"
```

Events stream back to your terminal in real time as the agent reads the project and responds. The same output appears live in the web dashboard.

---

## Step 7 — Stop the session

```bash
meru stop a1b2c3d4-...
```

---

## What's next?

Everything above required zero configuration. When you're ready for more:

| Topic | What it unlocks |
|-------|-----------------|
| [Agents](./agents/) | Per-agent flags, models, and environment variables |
| [Sessions](./sessions) | Multi-turn conversations, git worktrees, event history |
| [Broadcast](./broadcast) | Fan out one prompt to all running agents at once |
| [Remote Nodes](./remote-nodes) | Spawn agents on remote machines over gRPC |
| [Web Dashboard](./web-ui) | Clone repos from the UI, browse filesystems, manage nodes |
| [REST API](./api-reference) | Integrate Meru into scripts and your own tools |
| [CLI Reference](./cli-reference) | Every command and flag |
