---
layout: default
title: Contributing
nav_order: 15
---

# Contributing
{: .no_toc }

## Table of contents
{: .no_toc .text-delta }

1. TOC
{:toc}

---

## Development setup

### Step 1 — Clone and install dependencies

```bash
git clone https://github.com/ash-thakur-rh/meru.git
cd meru

# Install Go dependencies
go mod download

# Install Node dependencies for the web UI
cd web && npm install && cd ..
```

### Step 2 — Build

```bash
make all        # build UI + both binaries
```

### Step 3 — Run tests

```bash
make test-unit   # unit + integration tests (fast, no real binaries needed)
make test-smoke  # full-stack smoke tests (builds and runs the real binary)
make test        # both
```

### Step 4 — Format and lint

```bash
make fmt         # format Go (go fmt) and web (prettier) files
make fmt-check   # check formatting without writing (used in CI)
make vet         # go vet
```

### Step 5 — Start in dev mode

```bash
# Terminal 1 — daemon with hot-reloading Go (use air or similar)
make run

# Terminal 2 — Vite dev server (hot-reloads frontend, proxies API to :8080)
make dev
```

---

## Project layout

```
meru/
├── cmd/
│   ├── meru/          # control-plane CLI + daemon entry point
│   └── meru-node/     # remote node daemon
├── internal/
│   ├── agent/              # Agent/Session interfaces + registry
│   │   └── adapters/       # claude, aider, opencode, goose
│   ├── api/                # REST + WebSocket HTTP server
│   ├── node/               # Node interface, local node, gRPC client
│   ├── notify/             # cross-platform desktop notifications
│   ├── proto/              # generated protobuf/gRPC code
│   ├── session/            # session manager (lifecycle + persistence)
│   ├── store/              # SQLite persistence layer
│   ├── testutil/           # shared test helpers (MockAgent, MockSession)
│   ├── ui/                 # embedded web UI (//go:embed dist)
│   └── workspace/          # git worktree management
├── proto/
│   └── meru.proto     # gRPC service definition
├── tests/
│   └── smoke_test.go       # full-stack smoke tests (-tags smoke)
├── web/                    # React + Vite + TypeScript frontend
│   └── src/
│       ├── api.ts
│       ├── components/
│       ├── hooks/
│       └── pages/
├── docs/                   # this documentation site
├── Makefile
├── go.mod
└── LICENSE
```

---

## Adding a custom agent

### Step 1 — Create the adapter package

```
internal/agent/adapters/<yourname>/adapter.go
```

Implement the `agent.Agent` interface:

```go
package youragent

import (
    "context"
    "github.com/ash-thakur-rh/meru/internal/agent"
)

type Adapter struct{}

func New() *Adapter { return &Adapter{} }

func (a *Adapter) Name() string { return "youragent" }

func (a *Adapter) Capabilities() agent.Capabilities {
    return agent.Capabilities{
        Streaming: false,
        MultiTurn: true,
    }
}

func (a *Adapter) Spawn(_ context.Context, cfg agent.SpawnConfig) (agent.Session, error) {
    return &Session{
        id:        uuid.New().String(),
        name:      cfg.Name,
        workspace: cfg.Workspace,
    }, nil
}
```

Implement the `agent.Session` interface:

```go
type Session struct {
    id, name, workspace string
    // your fields ...
}

func (s *Session) ID() string        { return s.id }
func (s *Session) Name() string      { return s.name }
func (s *Session) AgentName() string { return "youragent" }
func (s *Session) Workspace() string { return s.workspace }
func (s *Session) Status() agent.Status { ... }
func (s *Session) Logs() io.Reader   { ... }
func (s *Session) Stop() error       { ... }

func (s *Session) Send(ctx context.Context, prompt string) (<-chan agent.Event, error) {
    ch := make(chan agent.Event, 64)
    go func() {
        defer close(ch)
        // run your agent, emit events...
        ch <- agent.Event{Type: agent.EventText, Text: "hello"}
        ch <- agent.Event{Type: agent.EventDone}
    }()
    return ch, nil
}
```

### Step 2 — Register it in the daemon

In `cmd/meru/serve.go`, add your adapter to the startup sequence:

```go
import youragent "github.com/ash-thakur-rh/meru/internal/agent/adapters/youragent"

// in runServe():
agent.Register(youragent.New())
```

### Step 3 — Test it

```bash
meru spawn youragent --workspace /tmp/test
meru send <session-id> "hello"
```

---

## Regenerating protobuf code

If you modify `proto/meru.proto`:

```bash
protoc \
  --go_out=internal/proto --go_opt=paths=source_relative \
  --go-grpc_out=internal/proto --go-grpc_opt=paths=source_relative \
  -I proto \
  proto/meru.proto
```

Requires `protoc`, `protoc-gen-go`, and `protoc-gen-go-grpc`:

```bash
go install google.golang.org/protobuf/cmd/protoc-gen-go@latest
go install google.golang.org/grpc/cmd/protoc-gen-go-grpc@latest
```

---

## Pull request checklist

- [ ] `make test` passes
- [ ] `make fmt-check` passes (run `make fmt` if not)
- [ ] `make vet` passes
- [ ] New code has corresponding tests
- [ ] Docs updated if behaviour changed

---

## License

By contributing, you agree that your contributions will be licensed under the [Apache 2.0 License](../LICENSE).
