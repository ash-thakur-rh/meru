---
layout: default
title: Upgrading
nav_order: 14
---

# Upgrading
{: .no_toc }

## Table of contents
{: .no_toc .text-delta }

1. TOC
{:toc}

---

## General upgrade procedure

### Step 1 — Stop the daemon

```bash
# Send SIGTERM for a graceful shutdown
pkill -TERM meru

# Or if running in the foreground, press Ctrl+C
```

If you're running `meru-node` on remote machines, stop those too:

```bash
# On each remote machine
pkill -TERM meru-node
```

### Step 2 — Pull the latest source

```bash
cd /path/to/meru
git pull origin main
```

### Step 3 — Rebuild

```bash
# Rebuild UI + both binaries
make all
```

### Step 4 — (If installed to PATH) Reinstall

```bash
make install
```

### Step 5 — Restart the daemon

```bash
meru serve
```

### Step 6 — Restart remote nodes

On each remote machine, restart `meru-node` with the same flags as before.

---

## Database migrations

The SQLite schema is managed automatically. When you start a new version of `meru serve`, it runs any pending migrations on `~/.meru/meru.db` before accepting requests. No manual migration steps are needed.

**Back up your database before upgrading** if you have important session history:

```bash
cp ~/.meru/meru.db ~/.meru/meru.db.bak
```

---

## Upgrading agent CLIs

Meru adapters call agent CLIs at runtime. Upgrade them independently:

```bash
# Claude Code
npm update -g @anthropic-ai/claude-code

# Aider
pip install --upgrade aider-chat
```

No Meru restart is needed after upgrading agent CLIs — they are invoked fresh for each `Send` call.

---

## Upgrading remote nodes

Remote `meru-node` binaries must be upgraded separately on each machine. The control plane and nodes do not need to be on the exact same version, but it is recommended to keep them in sync to avoid protocol mismatches.

```bash
# On each remote machine:
cd /path/to/meru
git pull origin main
make build
pkill -TERM meru-node
./meru-node serve --token <token> [--tls-cert ... --tls-key ...]
```

---

## Checking the current version

```bash
meru --version
meru-node --version
```

---

## Rolling back

If you need to roll back to a previous version:

```bash
git checkout <previous-tag-or-commit>
make all
make install   # or copy binaries manually
```

Restore your database backup if the migration introduced incompatible schema changes:

```bash
cp ~/.meru/meru.db.bak ~/.meru/meru.db
```
