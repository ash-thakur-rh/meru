---
layout: default
title: Aider
parent: Agents
nav_order: 2
---

# Aider
{: .no_toc }

## Table of contents
{: .no_toc .text-delta }

1. TOC
{:toc}

---

## Overview

The `aider` adapter wraps [Aider](https://aider.chat), a terminal-based AI pair programmer. It runs Aider under a PTY so the full terminal output — diffs, progress, file edits — streams to the client in real time. Conversation history is maintained via a markdown file in the workspace.

| Property | Value |
|----------|-------|
| Agent name | `aider` |
| Streaming | Yes (PTY) |
| Multi-turn | Yes |
| Tool use | No |
| Default model | `claude-sonnet-4-6` |

---

## Prerequisites

```bash
pip install aider-chat
```

Verify:

```bash
aider --version
```

Aider needs an API key in the environment depending on the model:

```bash
export ANTHROPIC_API_KEY=sk-...    # for Claude models
export OPENAI_API_KEY=sk-...       # for OpenAI models
```

---

## Spawning a session

```bash
meru spawn aider \
  --workspace ~/projects/myapp \
  --name aider-bot \
  --model claude-sonnet-4-6
```

| Flag | Description |
|------|-------------|
| `--workspace` | Project directory. Aider reads and edits files here. |
| `--name` | Human-readable label for the session |
| `--model` | Model to pass to Aider (`--model` flag) |

---

## Sending prompts

```bash
meru send <session-id> "add docstrings to all public functions in src/auth.py"
```

Output streams back in real time via a PTY, including Aider's diff display and edit confirmations.

---

## Multi-turn conversations

The adapter maintains a markdown chat history file at:

```
<workspace>/.meru-aider-history.md
```

Aider reads this file automatically on each invocation, so it remembers previous instructions and code changes across `Send` calls. You can inspect this file to see the full conversation history.

---

## How the adapter works

Each `Send` call runs:

```bash
aider \
  --message "<prompt>" \
  --yes-always \
  --no-pretty \
  --model <model> \
  --chat-history-file <workspace>/.meru-aider-history.md
```

under a PTY. Raw terminal output is forwarded as `EventText` events as it arrives. On clean exit, `EventDone` is emitted; on non-zero exit, `EventError`.

Because Aider modifies files directly, you can inspect the git diff in the workspace after each `Send`.
