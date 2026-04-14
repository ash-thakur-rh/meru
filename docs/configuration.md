---
layout: default
title: Configuration
nav_order: 4
---

# Configuration
{: .no_toc }

## Table of contents
{: .no_toc .text-delta }

1. TOC
{:toc}

---

## Daemon flags

Start the daemon with `meru serve`. All flags are optional.

| Flag | Default | Description |
|------|---------|-------------|
| `--addr`, `-a` | `:8080` | HTTP listen address for the API and web UI |

### Examples

```bash
# Default — listens on all interfaces, port 8080
meru serve

# Local only
meru serve --addr 127.0.0.1:8080

# Custom port
meru serve --addr :9000
```

---

## Data directory

All persistent state lives in `~/.meru/`:

```
~/.meru/
└── meru.db    # SQLite database
```

The database holds:
- **Sessions** — ID, name, agent, workspace, node, status, timestamps
- **Events** — full event history per session
- **Nodes** — registered remote node addresses and tokens

There is currently no configuration file — all settings are passed via flags or the API.

---

## Environment variables

Meru itself does not read environment variables for its own configuration. However, you can pass extra environment variables **to the agent process** when spawning a session:

```bash
# Via CLI (not yet exposed as a flag — use the API directly)
curl -X POST http://localhost:8080/sessions \
  -H 'Content-Type: application/json' \
  -d '{
    "agent": "claude",
    "workspace": "/tmp/work",
    "env": {
      "ANTHROPIC_API_KEY": "sk-...",
      "MY_VAR": "value"
    }
  }'
```

The agent process inherits the daemon's environment, so any variables already set when you ran `meru serve` are automatically available to spawned agents.

---

## Remote node flags

The `meru-node` daemon has its own set of flags:

| Flag | Default | Required | Description |
|------|---------|----------|-------------|
| `--addr`, `-a` | `:9090` | No | gRPC listen address |
| `--token` | — | **Yes** | Bearer token clients must present |
| `--tls-cert` | — | No | Path to TLS certificate (enables TLS) |
| `--tls-key` | — | No | Path to TLS private key |

```bash
# Minimal — insecure (use only on trusted networks / localhost)
meru-node serve --token mysecrettoken

# Custom port
meru-node serve --addr :7070 --token mysecrettoken

# With TLS (recommended for remote machines)
meru-node serve \
  --addr :9090 \
  --token mysecrettoken \
  --tls-cert /etc/meru/cert.pem \
  --tls-key  /etc/meru/key.pem
```

---

## TLS for remote nodes

When `--tls-cert` and `--tls-key` are provided, `meru-node` serves over TLS. On the control-plane side, register the node with `--tls`:

```bash
meru nodes add gpu-box \
  --addr gpu-box.internal:9090 \
  --token mysecrettoken \
  --tls
```

For self-signed certificates, the control-plane trusts the system certificate store. Add your CA cert to the system store on the machine running `meru serve`.

---

## Running as a system service

### launchd (macOS)

Create `~/Library/LaunchAgents/com.meru.daemon.plist`:

```xml
<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN"
  "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0">
<dict>
  <key>Label</key>
  <string>com.meru.daemon</string>
  <key>ProgramArguments</key>
  <array>
    <string>/usr/local/bin/meru</string>
    <string>serve</string>
    <string>--addr</string>
    <string>127.0.0.1:8080</string>
  </array>
  <key>RunAtLoad</key>
  <true/>
  <key>KeepAlive</key>
  <true/>
  <key>StandardOutPath</key>
  <string>/tmp/meru.log</string>
  <key>StandardErrorPath</key>
  <string>/tmp/meru.err</string>
</dict>
</plist>
```

```bash
launchctl load ~/Library/LaunchAgents/com.meru.daemon.plist
```

### systemd (Linux)

Create `/etc/systemd/system/meru.service`:

```ini
[Unit]
Description=Meru AI Agent Orchestrator
After=network.target

[Service]
Type=simple
ExecStart=/usr/local/bin/meru serve --addr 127.0.0.1:8080
Restart=on-failure
User=youruser

[Install]
WantedBy=multi-user.target
```

```bash
sudo systemctl enable --now meru
```
