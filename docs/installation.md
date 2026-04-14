---
layout: default
title: Installation
nav_order: 2
---

# Installation
{: .no_toc }

Conductor is a **single self-contained binary** — no database server, no config file, no containers. Download it, run `meru serve`, and everything is live.
{: .fs-5 .fw-300 }

## Table of contents
{: .no_toc .text-delta }

1. TOC
{:toc}

---

## Download pre-built binaries (recommended)

Pre-built binaries for Linux, macOS, and Windows are attached to every [GitHub release](https://github.com/ash-thakur-rh/meru/releases).

### macOS / Linux — one-liner

Replace `VERSION` with the release you want (e.g. `v0.1.0`), and set `OS` / `ARCH` to match your machine:

| OS | `OS` value |
|----|-----------|
| macOS | `darwin` |
| Linux | `linux` |

| CPU | `ARCH` value |
|-----|-------------|
| Apple Silicon / ARM | `arm64` |
| Intel / AMD 64-bit | `amd64` |

```bash
VERSION=v0.1.0
OS=darwin      # or linux
ARCH=arm64     # or amd64

curl -fsSL "https://github.com/ash-thakur-rh/meru/releases/download/${VERSION}/conductor_${OS}_${ARCH}.tar.gz" \
  | tar -xz

curl -fsSL "https://github.com/ash-thakur-rh/meru/releases/download/${VERSION}/meru-node_${OS}_${ARCH}.tar.gz" \
  | tar -xz

# Move to a directory on your PATH
sudo mv meru meru-node /usr/local/bin/
```

### Windows

Download the `.zip` archives from the [Releases page](https://github.com/ash-thakur-rh/meru/releases):

```
meru_windows_amd64.zip
meru-node_windows_amd64.zip
```

Extract both archives and place the `.exe` files somewhere on your `%PATH%`.

### Verify the installation

```bash
meru --version
meru-node --version
```

---

## Prerequisites

Go and Node.js are **not** required when using pre-built binaries. You only need:

- **Git** — only if you use the git worktree feature (`--worktree` flag on spawn). Git clone operations are handled by the built-in go-git library and do **not** require a native git installation.
- **Agent CLIs** — install only the ones you intend to use.

| Agent | Install |
|-------|---------|
| Claude Code | `npm install -g @anthropic-ai/claude-code` |
| Aider | `pip install aider-chat` |
| OpenCode | [opencode.ai](https://opencode.ai) |
| Goose | [block.github.io/goose](https://block.github.io/goose) |

---

## Build from source

Use this approach if you want to build from a specific commit or contribute to Conductor.

### Requirements

| Requirement | Version | Purpose |
|-------------|---------|---------|
| [Go](https://go.dev/dl/) | 1.21+ | Build both binaries |
| [Node.js](https://nodejs.org/) + npm | 18+ | Build the embedded web UI |

### Steps

```bash
# 1. Clone
git clone https://github.com/ash-thakur-rh/meru.git
cd meru

# 2. Build web UI + binaries
make all

# 3. Install to GOPATH/bin (optional)
make install
```

`make all` runs `npm install`, `npm run build`, and `go build` in sequence, producing `meru` and `meru-node` in the current directory.

Add them to your PATH if you used `make install`:

```bash
export PATH="$PATH:$(go env GOPATH)/bin"
```

### Build targets

| Target | Description |
|--------|-------------|
| `make ui` | Build the web UI only |
| `make build` | Build both binaries (uses last built UI) |
| `make all` | Build UI + both binaries in one step |
| `make install` | Build UI + install both binaries to `GOPATH/bin` |

---

## Data directory

The meru daemon stores all persistent state in:

```
~/.meru/
└── meru.db    # SQLite database (sessions, events, nodes)
```

This directory is created automatically on first run.

---

## Upgrading

See the [Upgrading](./upgrading) page for step-by-step upgrade instructions.
