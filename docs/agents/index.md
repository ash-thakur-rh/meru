---
layout: default
title: Agents
nav_order: 5
has_children: true
---

# Agents
{: .no_toc }

Conductor supports multiple AI coding agent backends through a common adapter interface. Each adapter translates Conductor's session lifecycle into the commands and APIs that the underlying agent CLI expects.

All CLI-based adapters run the agent under a **PTY** (pseudo-terminal) so that the full terminal output — colours, progress spinners, diffs, tool-use animations — streams to the client in real time. xterm.js in the web UI renders this output natively, and keystrokes typed in the browser are forwarded directly to the agent's PTY stdin. The result is a fully interactive session indistinguishable from running the agent in your local terminal.

---

## Supported agents

| Agent | Name | Streaming | Multi-turn | Tool use |
|-------|------|-----------|-----------|----------|
| [Claude Code](./claude) | `claude` | Yes (PTY) | Yes | Yes |
| [Aider](./aider) | `aider` | Yes (PTY) | Yes | No |
| [OpenCode](./opencode) | `opencode` | Yes (PTY) | Yes | Yes |
| [Goose](./goose) | `goose` | Yes (PTY) | Yes | Yes |

---

## How adapters work

Each adapter implements two interfaces:

**`Agent`** — a factory that knows how to spawn sessions:
```
Name()          → string
Capabilities()  → Capabilities
Spawn(cfg)      → Session
```

**`Session`** — a live connection to a running agent:
```
ID(), Name(), AgentName(), Workspace(), Status()
Send(prompt)    → <-chan Event
Stop()          → error
Logs()          → io.Reader
```

When you call `meru spawn claude`, the daemon:
1. Looks up the `claude` adapter in the agent registry
2. Calls `Spawn(cfg)` to create a new `Session`
3. Stores the session in memory and in SQLite
4. Returns the session ID to the caller

---

## How PTY streaming works

All four adapters use the same pattern:

1. **Spawn once** — `pty.Start(cmd)` starts the agent CLI in interactive mode under a PTY. The process lives for the entire session lifetime.
2. **Read loop** — a goroutine reads raw bytes from the PTY master in 4 KB chunks and fans them out to all active subscribers (terminal WebSocket, `Send` listeners, startup waiter).
3. **Bidirectional** — keystrokes from the browser are written directly to PTY stdin via `WriteInput`. Resize events update the PTY window size via `ResizePTY`.
4. **Waiting detection** — after each chunk the adapter checks the last 300 bytes of output for approval-prompt patterns (`(y/n)`, `[y/n]`, `press enter`, etc.). If detected, the session status transitions to `waiting` so the dashboard can highlight it. Status clears back to `busy` when the user types.
5. **Log accumulation** — all PTY bytes are also written to an in-memory log buffer, accessible via `meru logs` and replayed to new terminal WebSocket connections.

---

## Multi-turn conversations

Because each adapter runs the agent in persistent **interactive mode** for the lifetime of the session, conversation context is maintained naturally by the agent itself — no special history-file or `--continue` flags are needed. Successive prompts typed in the terminal (or sent via `POST /sessions/:id/send`) build on the existing conversation just as they would if you were typing in your local terminal.

---

## Adding a custom agent

See the [Contributing](../contributing#adding-a-custom-agent) guide.
