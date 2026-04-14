// Package notify sends desktop notifications when agent tasks complete.
//
// Platform support:
//   - macOS   : osascript (built-in)
//   - Windows  : PowerShell toast via Windows.UI.Notifications (Windows 10+, built-in)
//   - Linux    : notify-send → kdialog → zenity  (first available wins)
//   - WSL      : falls through to Windows PowerShell via /mnt/c/Windows/...
package notify

import (
	"fmt"
	"os"
	"os/exec"
	"runtime"
	"strings"
)

// TaskDone fires a desktop notification when a session finishes a task.
func TaskDone(sessionName, agentName string) {
	send(
		fmt.Sprintf("Conductor — %s", sessionName),
		fmt.Sprintf("%s finished a task", agentName),
		levelNormal,
	)
}

// WaitingForInput fires a desktop notification when an agent is waiting for
// user approval or input (e.g. a y/n prompt or a "Do you want to proceed?" question).
func WaitingForInput(sessionName, agentName string) {
	send(
		fmt.Sprintf("Conductor — %s", sessionName),
		fmt.Sprintf("%s is waiting for your input", agentName),
		levelCritical,
	)
}

// Error fires a desktop notification for a session error.
func Error(sessionName, msg string) {
	send(
		fmt.Sprintf("Conductor — %s", sessionName),
		fmt.Sprintf("Error: %s", msg),
		levelCritical,
	)
}

type urgency int

const (
	levelNormal   urgency = iota
	levelCritical urgency = iota
)

func send(title, body string, level urgency) {
	switch runtime.GOOS {
	case "darwin":
		sendDarwin(title, body)
	case "windows":
		sendWindows(title, body, level)
	case "linux":
		if isWSL() {
			sendWindowsFromWSL(title, body, level)
		} else {
			sendLinux(title, body, level)
		}
	}
}

// --- macOS ---

func sendDarwin(title, body string) {
	script := fmt.Sprintf(`display notification %q with title %q`, body, title)
	exec.Command("osascript", "-e", script).Start() //nolint:errcheck
}

// --- Windows ---

// sendWindows uses PowerShell to display a Windows 10/11 toast notification.
// No third-party tools required — uses the WinRT APIs available in every
// modern Windows install.
func sendWindows(title, body string, level urgency) {
	scenario := "default"
	if level == levelCritical {
		scenario = "alarm" // gives the notification more visual prominence
	}

	// Build the toast XML; escape angle brackets in user-supplied strings.
	xml := fmt.Sprintf(
		`<toast scenario="%s"><visual><binding template="ToastGeneric"><text>%s</text><text>%s</text></binding></visual></toast>`,
		scenario, xmlEscape(title), xmlEscape(body),
	)

	script := fmt.Sprintf(`
$xml = New-Object Windows.Data.Xml.Dom.XmlDocument
[Windows.UI.Notifications.ToastNotificationManager,Windows.UI.Notifications,ContentType=WindowsRuntime] | Out-Null
[Windows.Data.Xml.Dom.XmlDocument,Windows.Data.Xml.Dom.XmlDocument,ContentType=WindowsRuntime] | Out-Null
$xml.LoadXml('%s')
$toast = [Windows.UI.Notifications.ToastNotification]::new($xml)
[Windows.UI.Notifications.ToastNotificationManager]::CreateToastNotifier('Conductor').Show($toast)
`, strings.ReplaceAll(xml, "'", "''")) // escape single quotes for PowerShell

	exec.Command("powershell", "-NoProfile", "-NonInteractive", "-Command", script).Start() //nolint:errcheck
}

// sendWindowsFromWSL calls the Windows PowerShell binary directly from WSL.
func sendWindowsFromWSL(title, body string, level urgency) {
	ps := "/mnt/c/Windows/System32/WindowsPowerShell/v1.0/powershell.exe"
	if _, err := os.Stat(ps); err != nil {
		return // PowerShell not accessible from this WSL mount
	}

	scenario := "default"
	if level == levelCritical {
		scenario = "alarm"
	}
	xml := fmt.Sprintf(
		`<toast scenario="%s"><visual><binding template="ToastGeneric"><text>%s</text><text>%s</text></binding></visual></toast>`,
		scenario, xmlEscape(title), xmlEscape(body),
	)
	script := fmt.Sprintf(`
$xml = New-Object Windows.Data.Xml.Dom.XmlDocument
[Windows.UI.Notifications.ToastNotificationManager,Windows.UI.Notifications,ContentType=WindowsRuntime] | Out-Null
[Windows.Data.Xml.Dom.XmlDocument,Windows.Data.Xml.Dom.XmlDocument,ContentType=WindowsRuntime] | Out-Null
$xml.LoadXml('%s')
$toast = [Windows.UI.Notifications.ToastNotification]::new($xml)
[Windows.UI.Notifications.ToastNotificationManager]::CreateToastNotifier('Conductor').Show($toast)
`, strings.ReplaceAll(xml, "'", "''"))

	exec.Command(ps, "-NoProfile", "-NonInteractive", "-Command", script).Start() //nolint:errcheck
}

// --- Linux ---

// sendLinux tries notify-send, then kdialog (KDE), then zenity (GNOME/GTK).
func sendLinux(title, body string, level urgency) {
	urgencyStr := "normal"
	if level == levelCritical {
		urgencyStr = "critical"
	}

	// notify-send (libnotify) — works on GNOME, KDE, XFCE, most DEs
	if path, err := exec.LookPath("notify-send"); err == nil {
		exec.Command(path,
			"--urgency", urgencyStr,
			"--expire-time", "6000", // ms
			"--app-name", "Conductor",
			title, body,
		).Start() //nolint:errcheck
		return
	}

	// kdialog — KDE fallback
	if path, err := exec.LookPath("kdialog"); err == nil {
		exec.Command(path, "--passivepopup",
			fmt.Sprintf("%s\n%s", title, body),
			"6",
		).Start() //nolint:errcheck
		return
	}

	// zenity — GNOME/GTK fallback
	if path, err := exec.LookPath("zenity"); err == nil {
		notifType := "--info"
		if level == levelCritical {
			notifType = "--error"
		}
		exec.Command(path, notifType,
			"--title", title,
			"--text", body,
			"--timeout", "6",
			"--no-wrap",
		).Start() //nolint:errcheck
	}
}

// --- helpers ---

// isWSL reports whether the process is running inside Windows Subsystem for Linux.
func isWSL() bool {
	data, err := os.ReadFile("/proc/version")
	if err != nil {
		return false
	}
	lower := strings.ToLower(string(data))
	return strings.Contains(lower, "microsoft") || strings.Contains(lower, "wsl")
}

// xmlEscape replaces characters that are special in XML.
func xmlEscape(s string) string {
	s = strings.ReplaceAll(s, "&", "&amp;")
	s = strings.ReplaceAll(s, "<", "&lt;")
	s = strings.ReplaceAll(s, ">", "&gt;")
	s = strings.ReplaceAll(s, `"`, "&quot;")
	s = strings.ReplaceAll(s, "'", "&apos;")
	return s
}
