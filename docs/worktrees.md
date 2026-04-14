---
layout: default
title: Git Worktrees
nav_order: 9
---

# Git Worktrees
{: .no_toc }

Give each session its own isolated branch and working directory.
{: .fs-6 .fw-300 }

## Table of contents
{: .no_toc .text-delta }

1. TOC
{:toc}

---

## Overview

When you spawn a session with `--worktree`, Conductor creates a [git worktree](https://git-scm.com/docs/git-worktree) — a separate checkout of the repository on a new branch. The agent works in this isolated directory, so its changes don't affect your main working tree until you decide to merge them.

This is especially useful when:
- Running multiple agents on the same repo simultaneously
- Experimenting with a risky refactor without touching your current branch
- Reviewing what an agent did before merging its changes

---

## Using worktrees

```bash
meru spawn claude \
  --workspace ~/projects/myapp \
  --worktree
```

Conductor will:
1. Detect that `~/projects/myapp` is inside a git repository
2. Find the repository root
3. Create a new branch `meru/<session-id>`
4. Check it out into `.meru-worktrees/<session-id>/`
5. Set the agent's workspace to that directory

---

## Directory structure

```
~/projects/myapp/                        ← your working directory (unchanged)
├── .meru-worktrees/
│   ├── a1b2c3d4-e5f6-.../               ← session A's isolated checkout
│   └── b2c3d4e5-f6a7-.../               ← session B's isolated checkout
├── src/
└── ...
```

Each session gets its own branch:
- `meru/a1b2c3d4-e5f6-...`
- `meru/b2c3d4e5-f6a7-...`

---

## Reviewing changes

After the agent finishes, review its work before merging:

```bash
# See what the agent changed
git diff main meru/<session-id>

# Or check it out interactively
git log meru/<session-id> --oneline

# Merge if happy
git merge meru/<session-id>
```

---

## Cleanup

When you stop the session, Conductor automatically removes the worktree and its branch:

```bash
meru stop <session-id>
```

This runs:
```bash
git worktree remove --force .meru-worktrees/<session-id>
git branch -D meru/<session-id>
```

If the daemon crashes before cleaning up, you can manually remove stale worktrees:

```bash
git worktree list           # see all worktrees
git worktree prune          # remove references to deleted directories
git worktree remove --force .meru-worktrees/<session-id>
git branch -D meru/<session-id>
```

---

## Limitations

- The workspace must be inside a git repository. If it isn't, the `--worktree` flag is silently ignored.
- The repository must have at least one commit (worktrees cannot be created from an empty repo).
- Worktree creation uses the native `git` binary. Make sure `git` is installed on whichever node (local or remote) you spawn the session on.
