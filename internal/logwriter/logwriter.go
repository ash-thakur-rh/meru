// Package logwriter configures the global slog logger to write structured logs
// to both stderr and a rotating log file.
//
// Usage:
//
//	closer, err := logwriter.Setup(dataDir)
//	if err != nil { /* non-fatal, logs still go to stderr */ }
//	defer closer.Close()
//
// Every package can then call slog.Info / slog.Error / etc. directly — no
// logger instance needs to be threaded through the call stack.
//
// Set CONDUCTOR_LOG_LEVEL=debug to enable verbose logging.
package logwriter

import (
	"io"
	"log"
	"log/slog"
	"os"
	"path/filepath"
)

// Closer wraps the open log file so the caller can flush and close it on exit.
type Closer struct{ f *os.File }

func (c *Closer) Close() error { return c.f.Close() }

// LogFile returns the path of the log file that was opened.
func (c *Closer) LogFile() string { return c.f.Name() }

// Setup initialises the global slog logger with a text handler that writes to
// both os.Stderr and a file at <logDir>/meru.log.
// It also redirects the stdlib log package so chi's request logger ends up in
// the same file.
//
// Returns a Closer that must be closed when the process exits.
// If the file cannot be opened, Setup logs a warning to stderr and configures
// slog to write to stderr only — the daemon continues running.
func Setup(logDir string) (*Closer, error) {
	if err := os.MkdirAll(logDir, 0o755); err != nil {
		return nil, err
	}

	logPath := filepath.Join(logDir, "meru.log")
	f, err := os.OpenFile(logPath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
	if err != nil {
		// Fall back to stderr-only logging
		setupHandler(os.Stderr)
		return nil, err
	}

	w := io.MultiWriter(os.Stderr, f)
	setupHandler(w)

	return &Closer{f}, nil
}

func setupHandler(w io.Writer) {
	level := slog.LevelInfo
	if os.Getenv("CONDUCTOR_LOG_LEVEL") == "debug" {
		level = slog.LevelDebug
	}

	h := slog.NewTextHandler(w, &slog.HandlerOptions{Level: level})
	slog.SetDefault(slog.New(h))

	// Redirect the stdlib log package (used by chi middleware.Logger) to the
	// same destination so all log output ends up in one place.
	log.SetOutput(w)
	log.SetFlags(0) // slog already adds the timestamp
}
