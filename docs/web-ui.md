---
layout: default
title: Web Dashboard
nav_order: 11
---

# Web Dashboard
{: .no_toc }

## Table of contents
{: .no_toc .text-delta }

1. TOC
{:toc}

---

## Overview

Conductor ships with a built-in web dashboard served directly from the daemon. No separate server or installation is required.

Access it at [http://localhost:8080](http://localhost:8080) after starting `meru serve`.

---

## Pages

### Dashboard

The main page lists all sessions with their status, agent, and workspace. Sessions are split into two sections:

**Active** — running sessions, sorted so `waiting` sessions appear first to draw attention.

**Recent** — stopped sessions kept for history. Hover a card to reveal:
- **Re-spawn** — opens the spawn modal pre-filled with the same agent, workspace, model, and node
- **Delete** — permanently removes the session record

From the header you can:

- **Spawn** a new session using the spawn modal
- **Click a session** to open its terminal view
- See live status badges: `idle` (green), `busy` (blue), `waiting` (orange pulse), `stopped` (grey), `error` (red)

**Waiting sessions** (orange border + ring) mean the agent has paused and is asking for your input — typically a y/n approval prompt. Click the card to open the terminal and respond.

### Session detail

Clicking a session opens a full bidirectional terminal powered by [xterm.js](https://xtermjs.org/) — identical to running the agent directly in your local terminal. It:

- Streams all raw PTY output live including colours, progress indicators, and TUI rendering
- Lets you type directly into the terminal (keystrokes forwarded to the agent's PTY stdin)
- Replays stored log output for stopped sessions so you can review history
- Resizes dynamically as you resize the browser window

For live sessions, the bottom of the page also has a **prompt bar** — type a prompt and press Enter (or click Send) to inject it into the terminal as if you typed it yourself. This is a shortcut; you can always type directly in the terminal above.

For stopped sessions, a banner is shown and a **Re-spawn** button appears in the header to start a fresh session in the same workspace.

### Nodes

The **Nodes** page (accessible from the top navigation) shows all registered nodes:

- The built-in `local` node is always present
- Remote nodes show their address, TLS status, and last-seen timestamp
- **Add** a new remote node with the "+ Add Node" button
- **Ping** a node to verify connectivity and see its agent list
- **Remove** a node from the registry

---

## Spawn modal

Click **New Session** on the dashboard to open the spawn modal. Fields:

| Field | Description |
|-------|-------------|
| Agent | Dropdown of all registered agents |
| Name | Optional — auto-generated if left blank |
| Workspace | Absolute path to the project directory. Click 📂 to browse the node's filesystem. |
| Model | Optional model override |
| Node | Dropdown of available nodes (local + remote) |
| Clone from git repository | Expand to clone a repo before spawning. Supports public and private (HTTPS) repos. Uses the built-in go-git library — no native `git` install required. |
| Git Worktree | Toggle — creates an isolated branch/worktree for the session |

### Cloning a repository

Expand the **Clone from git repository** section to clone a repo on the fly before spawning:

1. Enter the **Repository URL** (HTTPS or SSH).
2. Optionally set a **Clone to** destination path (defaults to `~/meru-workspaces/<repo>`). Click 📂 to browse.
3. For private HTTPS repos, expand **Private repository credentials** and enter your username and personal access token.
4. Click **Spawn** — Conductor clones the repo first, then spawns the agent in the cloned directory.

SSH repos use your local key agent automatically; no credentials are needed in the UI.

---

## Real-time updates

The dashboard polls for session status changes every 5 seconds. The session detail page connects a raw PTY WebSocket — output appears character-by-character exactly as the agent renders it, mirroring what you would see running the CLI locally.

---

## Embedding the UI

The web UI is compiled into the `meru` binary via Go's `//go:embed` directive. The built assets in `internal/ui/dist/` are embedded at build time. There are no runtime file dependencies.

To update the UI after making changes to the frontend:

```bash
make ui      # rebuild web assets
make build   # recompile the binary with the new assets
```

---

## Development mode

During frontend development, run Vite's dev server alongside the daemon:

```bash
# Terminal 1 — daemon (API on :8080)
make run

# Terminal 2 — Vite dev server with hot reload (on :5173, proxies API to :8080)
make dev
```

The Vite config proxies all `/api` requests to the meru daemon, so the UI works with live data during development.
