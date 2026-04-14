---
layout: default
title: REST API Reference
nav_order: 12
---

# REST API Reference
{: .no_toc }

All endpoints are served by `meru serve` on the configured address (default `:8080`).
{: .fs-6 .fw-300 }

## Table of contents
{: .no_toc .text-delta }

1. TOC
{:toc}

---

## Base URL

```
http://localhost:8080
```

## Content type

All request and response bodies are `application/json` unless otherwise noted. The `send` endpoint streams `application/x-ndjson`.

---

## Health

### GET /healthz

Returns daemon health status.

**Response** `200 OK`

```json
{ "status": "ok" }
```

---

## Sessions

### POST /sessions

Spawn a new agent session.

**Request body**

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `agent` | string | **Yes** | Agent name (`claude`, `aider`, `opencode`, `goose`) |
| `name` | string | No | Session label (auto-generated if omitted) |
| `workspace` | string | No | Working directory (defaults to `.`) |
| `model` | string | No | Model override (agent-specific) |
| `env` | object | No | Extra environment variables for the agent process |
| `worktree` | bool | No | Create an isolated git worktree (local node only) |
| `node` | string | No | Target node name (defaults to `local`) |

```json
{
  "agent":     "claude",
  "name":      "feature-bot",
  "workspace": "/home/user/projects/api",
  "model":     "claude-opus-4-6",
  "env":       { "DEBUG": "true" },
  "worktree":  false,
  "node":      ""
}
```

**Response** `201 Created`

```json
{
  "id":        "a1b2c3d4-e5f6-7890-abcd-ef1234567890",
  "name":      "feature-bot",
  "agent":     "claude",
  "workspace": "/home/user/projects/api",
  "status":    "idle"
}
```

**Errors**

| Code | Reason |
|------|--------|
| `400` | `agent` field is missing |
| `500` | Agent or node not found; spawn failed |

---

### GET /sessions

List all sessions (including stopped ones).

**Response** `200 OK`

```json
[
  {
    "id":        "a1b2c3d4-...",
    "name":      "feature-bot",
    "agent":     "claude",
    "workspace": "/home/user/projects/api",
    "status":    "idle"
  }
]
```

---

### GET /sessions/:id

Get a session. Returns real-time status for live sessions; falls back to the stored record for stopped sessions.

**Response** `200 OK`

```json
{
  "id":        "a1b2c3d4-e5f6-7890-abcd-ef1234567890",
  "name":      "feature-bot",
  "agent":     "claude",
  "workspace": "/home/user/projects/api",
  "status":    "idle",
  "node_name": "local",
  "model":     "claude-sonnet-4-6"
}
```

Status values: `starting`, `idle`, `busy`, `waiting`, `stopped`, `error`

**Errors**

| Code | Reason |
|------|--------|
| `404` | Session not found in live registry or store |

---

### DELETE /sessions/:id

Stop or purge a session. Behaviour depends on current state:

| Session state | Behaviour |
|---------------|-----------|
| Live | Terminates the process; marks status `stopped` in DB; cleans up worktree. Record kept for history. |
| Already stopped | Permanently deletes the record from the database. |

**Response** `204 No Content`

**Errors**

| Code | Reason |
|------|--------|
| `404` | Session not found (neither live nor in store) |

---

### POST /sessions/:id/send

Send a prompt to a session and stream events back.

**Request body**

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `prompt` | string | **Yes** | The prompt to send |

```json
{ "prompt": "add unit tests for the auth package" }
```

**Response** `200 OK`  
`Content-Type: application/x-ndjson`

Each line is a JSON-encoded event:

```json
{"type":"text","text":"I'll start by reading...","timestamp":"2026-04-13T10:00:00Z"}
{"type":"tool_use","tool_name":"read_file","tool_input":"{\"path\":\"auth.go\"}","timestamp":"2026-04-13T10:00:01Z"}
{"type":"text","text":"Here is the test file:","timestamp":"2026-04-13T10:00:02Z"}
{"type":"done","timestamp":"2026-04-13T10:00:05Z"}
```

**Event types**

| Type | Fields | Description |
|------|--------|-------------|
| `text` | `text` | A chunk of the agent's response |
| `tool_use` | `tool_name`, `tool_input` | Agent invoked a tool |
| `tool_result` | `text` | Result of a tool invocation |
| `done` | — | Agent finished responding |
| `error` | `error` | An error occurred |

**Errors**

| Code | Reason |
|------|--------|
| `400` | `prompt` field is missing |
| `500` | Session not found or send failed |

---

### GET /sessions/:id/logs

Retrieve the full event history for a session.

**Response** `200 OK`

```json
[
  {
    "session_id": "a1b2c3d4-...",
    "type":       "text",
    "text":       "Starting analysis...",
    "tool_name":  "",
    "tool_input": "",
    "error":      "",
    "timestamp":  "2026-04-13T10:00:00Z"
  }
]
```

---

### GET /sessions/:id/stream

Upgrade to WebSocket and receive real-time structured events.

**Upgrade:** `HTTP 101 Switching Protocols`

Each WebSocket message is a JSON-encoded event object (same shape as the nd-JSON events from `/send`).

---

### GET /sessions/:id/terminal

Upgrade to a bidirectional raw PTY WebSocket — identical to running the agent directly in your terminal.

**Upgrade:** `HTTP 101 Switching Protocols`

**Frame protocol:**

| Direction | Frame type | Content |
|-----------|------------|---------|
| Server → client | Binary | Raw PTY output bytes (ANSI, colour, cursor etc.) |
| Client → server | Binary | Raw keystroke bytes (forwarded to PTY stdin) |
| Client → server | Text | JSON resize event: `{"type":"resize","cols":N,"rows":N}` |

On connect, the server sends accumulated log history as a single binary frame so clients can replay what happened before connecting.

**Stopped sessions:** The endpoint upgrades the WebSocket, replays stored log output as binary frames, then closes cleanly. Closing the WebSocket does **not** stop the session.

**Errors**

| Code | Reason |
|------|--------|
| `404` | Session not found |
| `501` | Session is live but does not support PTY streaming |

---

## Broadcast

### POST /broadcast

Fan out a prompt to multiple sessions concurrently.

**Request body**

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `prompt` | string | **Yes** | The prompt to broadcast |
| `sessions` | array of strings | No | Session IDs to target (empty = all active idle sessions) |

```json
{
  "prompt":   "run the test suite",
  "sessions": ["a1b2c3d4-...", "b2c3d4e5-..."]
}
```

**Response** `200 OK`

```json
[
  {
    "SessionID":   "a1b2c3d4-...",
    "SessionName": "feature-bot",
    "Events":      [ ... ],
    "Err":         null
  },
  {
    "SessionID":   "b2c3d4e5-...",
    "SessionName": "test-bot",
    "Events":      [ ... ],
    "Err":         "session busy"
  }
]
```

**Errors**

| Code | Reason |
|------|--------|
| `400` | `prompt` field is missing |

---

## Nodes

### GET /nodes

List all registered remote nodes. The `token` field is always scrubbed from responses.

**Response** `200 OK`

```json
[
  {
    "name":      "gpu-box",
    "addr":      "gpu-box.internal:9090",
    "token":     "",
    "tls":       true,
    "last_seen": "2026-04-13T10:00:00Z"
  }
]
```

---

### POST /nodes

Register a new remote node.

**Request body**

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `name` | string | **Yes** | Unique name (cannot be `local`) |
| `addr` | string | **Yes** | `host:port` of the meru-node gRPC server |
| `token` | string | **Yes** | Bearer token for authentication |
| `tls` | bool | No | Connect with TLS |

```json
{
  "name":  "gpu-box",
  "addr":  "gpu-box.internal:9090",
  "token": "mysecrettoken",
  "tls":   true
}
```

**Response** `201 Created` — same shape as GET /nodes item, with `token` scrubbed.

**Errors**

| Code | Reason |
|------|--------|
| `400` | Missing required field or name is `local` |
| `502` | Node persisted but gRPC dial failed |

---

### DELETE /nodes/:name

Remove a node from the registry.

**Response** `204 No Content`

**Errors**

| Code | Reason |
|------|--------|
| `400` | Cannot remove the `local` node |
| `404` | Node not found |

---

### POST /nodes/:name/ping

Ping a node to verify connectivity and retrieve its capabilities.

**Response** `200 OK`

```json
{
  "name":    "gpu-box",
  "agents":  ["claude", "aider"],
  "version": "meru-node/1.0"
}
```

**Side effect:** Updates the node's `last_seen` timestamp.

**Errors**

| Code | Reason |
|------|--------|
| `404` | Node not found |
| `502` | Node unreachable |
