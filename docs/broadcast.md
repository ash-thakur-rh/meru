---
layout: default
title: Broadcast
nav_order: 7
---

# Broadcast
{: .no_toc }

Send a single prompt to multiple sessions simultaneously.
{: .fs-6 .fw-300 }

## Table of contents
{: .no_toc .text-delta }

1. TOC
{:toc}

---

## Overview

Broadcast fans out a prompt to multiple sessions concurrently and collects all results. It's useful for:

- Running the same task across multiple codebases
- Comparing how different agents respond to the same instruction
- Applying a global instruction to all active sessions at once

---

## Broadcast to all active sessions

### CLI

```bash
meru broadcast "run the test suite and report any failures"
```

The command sends the prompt to every session that is currently `idle` and prints the results as they come back.

### API

```bash
curl -X POST http://localhost:8080/broadcast \
  -H 'Content-Type: application/json' \
  -d '{"prompt": "summarize what you have done so far"}'
```

---

## Broadcast to specific sessions

### CLI

```bash
meru broadcast "add error handling" \
  --sessions a1b2c3d4-...,b2c3d4e5-...
```

### API

```bash
curl -X POST http://localhost:8080/broadcast \
  -H 'Content-Type: application/json' \
  -d '{
    "prompt":   "add error handling",
    "sessions": ["a1b2c3d4-...", "b2c3d4e5-..."]
  }'
```

---

## Response format

The API returns a JSON array with one result per session:

```json
[
  {
    "SessionID":   "a1b2c3d4-...",
    "SessionName": "api-bot",
    "Events": [
      {"type":"text","text":"Starting...\n","timestamp":"..."},
      {"type":"done","timestamp":"..."}
    ],
    "Err": null
  },
  {
    "SessionID":   "b2c3d4e5-...",
    "SessionName": "test-bot",
    "Events": [...],
    "Err": null
  }
]
```

If a session errors, its `Err` field is set and `Events` may be partial.

---

## Concurrency

All targeted sessions receive the prompt simultaneously. The broadcast waits for all sessions to finish before returning — it does not timeout individual sessions. If you need a timeout, set one on the HTTP client side.

---

## Notes

- Only `idle` sessions are included in an "all active" broadcast. Sessions with status `busy`, `stopped`, or `error` are skipped.
- If you pass explicit session IDs, those sessions are targeted regardless of status (they may return errors if not ready).
- Events from each session are fully accumulated before the response is returned. For long-running agents, this can take a while.
