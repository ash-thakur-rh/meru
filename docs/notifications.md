---
layout: default
title: Desktop Notifications
nav_order: 10
---

# Desktop Notifications
{: .no_toc }

## Table of contents
{: .no_toc .text-delta }

1. TOC
{:toc}

---

## Overview

Meru sends desktop notifications when a session finishes a task or encounters an error, so you don't have to watch the terminal while long-running agents work.

---

## Notification types

| Event | Title | Body | Urgency |
|-------|-------|------|---------|
| Task completed | `Meru — <agent>` | `<session-name> finished` | Normal |
| Error | `Meru — Error` | `<session-name>: <error message>` | Critical |

---

## Platform support

### macOS

Uses `osascript` (AppleScript), which is built into macOS. No additional software needed.

Notifications appear in Notification Center. Allow notifications from `osascript` or `Script Editor` in **System Settings → Notifications**.

### Linux

Tries each of the following in order, using the first one found:

| Tool | Install |
|------|---------|
| `notify-send` | `sudo apt install libnotify-bin` / `sudo dnf install libnotify` |
| `kdialog` | Available in KDE environments |
| `zenity` | `sudo apt install zenity` / `sudo dnf install zenity` |

```bash
# Recommended (works on most desktops)
sudo apt install libnotify-bin
```

### Windows

Uses PowerShell with the WinRT `ToastNotificationManager` API — available on Windows 10 and later with no additional software.

Notifications appear in the Windows notification tray. If notifications are suppressed, check **Settings → System → Notifications → PowerShell**.

### WSL (Windows Subsystem for Linux)

Meru detects the WSL environment by reading `/proc/version` and, if it contains `microsoft` or `wsl`, uses the Windows PowerShell binary at:

```
/mnt/c/Windows/System32/WindowsPowerShell/v1.0/powershell.exe
```

This sends a native Windows toast notification even though the process runs inside Linux. No additional configuration needed.

---

## Notification urgency

| Level | Trigger | Effect |
|-------|---------|--------|
| Normal | `EventDone` — agent finished | Standard notification |
| Critical | `EventError` — agent errored | High-urgency / alarm notification |

On Linux with `notify-send`, critical notifications use `--urgency critical`. On Windows, critical notifications set `scenario="alarm"` in the toast XML.

---

## Disabling notifications

Notifications are always sent when events occur. To suppress them, ensure none of the notification backends are available on your system — though this is not recommended.

A per-session or global opt-out flag may be added in a future release.
