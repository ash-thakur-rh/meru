# Meru

Orchestrate multiple AI coding agents — Claude Code, Aider, OpenCode, Goose, and more — from a single binary.

**No setup. No config. No extra processes.** Download the binary, run `meru serve`, and start orchestrating.

```bash
# macOS / Linux — download, extract, run
curl -fsSL https://github.com/ash-thakur-rh/meru/releases/latest/download/meru_darwin_arm64.tar.gz | tar -xz
./meru serve
# → open http://localhost:8080
```

That's it. Sessions, streaming, broadcast, remote nodes, git clone, web dashboard — all included, zero config required. Add more when you need it.

---

## Features

- **Multiple agent adapters** — Claude Code, Aider, OpenCode, Goose
- **Bidirectional terminal streaming** — each session runs the agent CLI under a PTY; the web UI gives you a live xterm.js terminal identical to running the agent locally — keystrokes, approvals, TUI rendering, all of it
- **Waiting-for-input detection** — when an agent pauses to ask for approval (y/n prompts etc.) its card turns orange and sorts to the top of the dashboard
- **Session history & re-spawn** — stopped sessions are kept in history and can be re-spawned in the same workspace with one click
- **Session management** — spawn, send prompts, stop, and stream events
- **Broadcast** — fan out a prompt to multiple sessions concurrently
- **Remote nodes** — spawn agents on remote machines over gRPC with Bearer token auth and optional TLS
- **Git clone** — clone any repository (public or private) directly from the UI using the built-in [go-git](https://github.com/go-git/go-git) library — no native `git` install required
- **Git worktrees** — isolated branch + working directory per session; requires native `git`
- **Event streaming** — nd-JSON over HTTP and WebSocket
- **Web UI** — built-in dashboard served from the daemon
- **Desktop notifications** — task-done and error alerts on macOS, Linux, and Windows/WSL
- **Persistent state** — SQLite-backed session and event history

## Architecture

```
┌──────────────────────────────────────┐
│             meru (daemon)         │
│                                      │
│  CLI ──► REST API ──► Session Mgr   │
│               │            │         │
│           Web UI       SQLite DB     │
│                            │         │
│                       Node Registry  │
│                       /           \  │
│              LocalNode          GRPCNode
│                 │                   │
│           Agent adapters     meru-node
│    (claude/aider/opencode/goose)   (remote)
└──────────────────────────────────────┘
```

The control plane (`meru`) manages sessions and persists state. Agent work runs either locally via the registered adapters or on remote machines via the `meru-node` daemon, which the control plane talks to over gRPC.

## Prerequisites

**To run pre-built binaries:**

| Tool | Purpose |
|------|---------|
| `git` | Required only for the `--worktree` feature. Git clone uses the built-in go-git library. |
| Agent CLIs | Install only the ones you intend to use (`claude`, `aider`, `opencode`, `goose`) |

**To build from source:**

| Tool | Purpose |
|------|---------|
| Go 1.21+ | Build both binaries |
| Node.js 18+ / npm | Build the embedded web UI |

## Getting Started

### Option A — Download binary (recommended)

```bash
# macOS (Apple Silicon)
curl -fsSL https://github.com/ash-thakur-rh/meru/releases/latest/download/meru_darwin_arm64.tar.gz | tar -xz
sudo mv meru /usr/local/bin/

# Start the daemon
meru serve
```

Open [http://localhost:8080](http://localhost:8080) — the dashboard is ready.

See [Installation](https://ash-thakur-rh.github.io/meru/installation) for Linux, Intel Mac, and Windows instructions.

### Option B — Build from source

```bash
git clone https://github.com/ash-thakur-rh/meru.git
cd meru
make all          # builds UI + both binaries
./meru serve
```

The daemon stores its database at `~/.meru/meru.db` and creates it automatically on first run.

## CLI

All commands talk to the daemon over HTTP (`--addr` defaults to `http://localhost:8080`).

### Sessions

```bash
# Spawn a new session
meru spawn claude --name my-bot --workspace ~/projects/myapp

# List sessions (active + stopped, with waiting [!] marker)
meru list

# Attach to a session's interactive terminal — type directly, see output live
meru attach <session-id>

# Send a single prompt and stream the response (non-interactive)
meru send <session-id> "refactor the auth module"

# View event history
meru logs <session-id>

# Stop a live session (keeps record)
meru stop <session-id>

# Permanently delete a stopped session record
meru delete <session-id>
```

### Broadcast

```bash
# Send a prompt to all active sessions
meru broadcast "summarize what you've done so far"

# Target specific sessions
meru broadcast --sessions <id1>,<id2> "run the test suite"
```

### Agents

```bash
# List registered agents
meru agents
```

### Nodes

```bash
# Add a remote node
meru nodes add gpu-box --addr 10.0.0.5:9090 --token <secret>

# List nodes
meru nodes list

# Ping a node
meru nodes ping gpu-box

# Remove a node
meru nodes remove gpu-box
```

## Remote Nodes

Run `meru-node` on any machine where you want to spawn agents:

```bash
meru-node serve --addr :9090 --token <secret>

# With TLS
meru-node serve --addr :9090 --token <secret> \
  --tls-cert cert.pem --tls-key key.pem
```

Then register it from the control plane:

```bash
meru nodes add gpu-box --addr 10.0.0.5:9090 --token <secret>
```

Once added, pass `--node gpu-box` to `meru spawn` to run agents there.

## REST API

The daemon exposes a JSON API on the same port as the web UI.

| Method | Path | Description |
|--------|------|-------------|
| `GET` | `/healthz` | Health check |
| `POST` | `/sessions` | Spawn a session |
| `GET` | `/sessions` | List all sessions (including stopped) |
| `GET` | `/sessions/:id` | Get a session (live or stopped) |
| `DELETE` | `/sessions/:id` | Stop live session (keeps record) or purge stopped session |
| `POST` | `/sessions/:id/send` | Send a prompt (nd-JSON stream) |
| `GET` | `/sessions/:id/logs` | Event history |
| `GET` | `/sessions/:id/stream` | WebSocket: structured event stream |
| `GET` | `/sessions/:id/terminal` | WebSocket: bidirectional raw PTY bridge |
| `POST` | `/broadcast` | Fan out to sessions |
| `GET` | `/nodes` | List remote nodes |
| `POST` | `/nodes` | Add a remote node |
| `DELETE` | `/nodes/:name` | Remove a remote node |
| `POST` | `/nodes/:name/ping` | Ping a node |
| `GET` | `/fs?path=&node=` | Browse a node's filesystem |
| `POST` | `/git/clone` | Clone a repository on a node |

### Example: spawn and send

```bash
# Spawn
curl -X POST http://localhost:8080/sessions \
  -H 'Content-Type: application/json' \
  -d '{"agent":"claude","name":"bot","workspace":"/tmp/work"}'

# Send (streams nd-JSON events)
curl -X POST http://localhost:8080/sessions/<id>/send \
  -H 'Content-Type: application/json' \
  -d '{"prompt":"write a hello world in Go"}'
```

## Development

```bash
make build        # build both binaries (uses last built UI)
make ui           # build the web UI only
make all          # build UI + both binaries

make test-unit    # run unit and integration tests
make test-smoke   # build the binary and run full-stack smoke tests
make test         # run everything

make fmt          # format Go (go fmt) and web (prettier) files
make fmt-check    # check formatting without writing (CI)
make vet          # run go vet

make run          # go run ./cmd/meru serve
make dev          # start Vite dev server (proxies API to :8080)
```

### Adding an Agent Adapter

1. Create `internal/agent/adapters/<name>/adapter.go` implementing `agent.Agent` and `agent.Session`.
2. Register it in `cmd/meru/serve.go`:
   ```go
   agent.Register(youradapter.New())
   ```
3. That's it — the REST API and CLI pick it up automatically.

## License

Apache 2.0 — see [LICENSE](LICENSE).
