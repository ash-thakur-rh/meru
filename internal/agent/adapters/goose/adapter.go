// Package goose implements the meru agent adapter for Goose (by Block).
//
// Goose is started once in interactive mode:
//
//	goose [--model <model>]
//
// The process lives for the session lifetime under a PTY so the full terminal
// output streams to the client in real time.  Callers can subscribe to raw PTY
// bytes via SubscribeRaw (for the bidirectional terminal WebSocket) or write
// text prompts via Send (for the programmatic API).
package goose

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"log/slog"
	"os"
	"os/exec"
	"regexp"
	"sync"
	"time"

	"github.com/creack/pty"
	"github.com/google/uuid"

	"github.com/ash-thakur-rh/meru/internal/agent"
	"github.com/ash-thakur-rh/meru/internal/notify"
)

const AgentName = "goose"

const inactivityTimeout = 2 * time.Second
const idleTimeout = 1500 * time.Millisecond
const startupQuiet = 1 * time.Second
const startupTimeout = 15 * time.Second

var ansiEscape = regexp.MustCompile(`\x1b(?:\[[0-9;:<=>?]*[ -/]*[@-~]|\][^\x07\x1b]*(?:\x07|\x1b\\)|[^[\]])`)

var controlChar = regexp.MustCompile(`[\x00-\x08\x0b-\x1f\x7f]`)

func looksLikeWaitingForInput(tail []byte) bool {
	stripped := ansiEscape.ReplaceAll(tail, nil)
	stripped = controlChar.ReplaceAll(stripped, []byte(" "))
	lower := bytes.ToLower(stripped)
	for _, p := range [][]byte{
		[]byte("do you want to proceed"),
		[]byte("do you want to"),
		[]byte("esc to cancel"),
		[]byte("would you like to"),
		[]byte("shall i"),
		[]byte("please confirm"),
		[]byte("(y/n)"),
		[]byte("[y/n]"),
		[]byte("(yes/no)"),
		[]byte("[yes/no]"),
		[]byte("press enter"),
		[]byte("continue? "),
		[]byte("proceed? "),
	} {
		if bytes.Contains(lower, p) {
			return true
		}
	}
	return false
}

type Adapter struct{}

func New() *Adapter { return &Adapter{} }

func (a *Adapter) Name() string { return AgentName }

func (a *Adapter) Capabilities() agent.Capabilities {
	return agent.Capabilities{
		Streaming: true,
		MultiTurn: true,
		ToolUse:   true,
	}
}

func (a *Adapter) Spawn(ctx context.Context, cfg agent.SpawnConfig) (agent.Session, error) {
	var args []string
	if cfg.Model != "" {
		args = append(args, "--model", cfg.Model)
	}

	cmd := exec.Command("goose", args...)
	cmd.Dir = cfg.Workspace
	if len(cfg.Env) > 0 {
		cmd.Env = append(cmd.Environ(), envSlice(cfg.Env)...)
	}

	slog.Debug("goose starting interactive session", "workspace", cfg.Workspace, "model", cfg.Model)

	ptmx, err := pty.Start(cmd)
	if err != nil {
		slog.Error("failed to start goose", "error", err)
		return nil, fmt.Errorf("start goose: %w", err)
	}

	s := &session{
		id:        uuid.New().String(),
		name:      cfg.Name,
		workspace: cfg.Workspace,
		model:     cfg.Model,
		env:       cfg.Env,
		cmd:       cmd,
		ptmx:      ptmx,
		status:    agent.StatusStarting,
		logBuf:    &safeBuffer{},
		subs:      make(map[int]chan []byte),
	}

	go s.readLoop()

	if err := s.waitStartup(ctx); err != nil {
		s.Stop() //nolint:errcheck
		return nil, fmt.Errorf("goose startup: %w", err)
	}

	s.setStatus(agent.StatusIdle)
	slog.Debug("goose session ready", "session", s.id, "workspace", cfg.Workspace)
	return s, nil
}

type session struct {
	id        string
	name      string
	workspace string
	model     string
	env       map[string]string

	cmd  *exec.Cmd
	ptmx *os.File

	mu     sync.Mutex
	status agent.Status
	logBuf *safeBuffer

	subsMu  sync.Mutex
	subs    map[int]chan []byte
	nextSub int
}

func (s *session) ID() string        { return s.id }
func (s *session) Name() string      { return s.name }
func (s *session) AgentName() string { return AgentName }
func (s *session) Workspace() string { return s.workspace }
func (s *session) Logs() io.Reader   { return s.logBuf.Reader() }

func (s *session) Status() agent.Status {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.status
}

func (s *session) setStatus(st agent.Status) {
	s.mu.Lock()
	s.status = st
	s.mu.Unlock()
}

func (s *session) Stop() error {
	s.setStatus(agent.StatusStopped)
	s.ptmx.Close()
	if s.cmd != nil && s.cmd.Process != nil {
		return s.cmd.Process.Kill()
	}
	return nil
}

// WriteInput writes raw bytes to the PTY stdin.
// Implements agent.PTYSession.
func (s *session) WriteInput(p []byte) error {
	if s.Status() == agent.StatusWaiting {
		s.setStatus(agent.StatusBusy)
	}
	_, err := s.ptmx.Write(p)
	return err
}

// SubscribeRaw registers a subscriber to receive raw PTY output.
// Implements agent.PTYSession.
func (s *session) SubscribeRaw(bufSize int) (<-chan []byte, func()) {
	s.subsMu.Lock()
	id := s.nextSub
	s.nextSub++
	ch := make(chan []byte, bufSize)
	s.subs[id] = ch
	s.subsMu.Unlock()

	cancel := func() {
		s.subsMu.Lock()
		delete(s.subs, id)
		s.subsMu.Unlock()
	}
	return ch, cancel
}

// ResizePTY sets the PTY window size.
// Implements agent.PTYSession.
func (s *session) ResizePTY(cols, rows uint16) error {
	return pty.Setsize(s.ptmx, &pty.Winsize{Cols: cols, Rows: rows})
}

func (s *session) readLoop() {
	buf := make([]byte, 4096)

	activity := make(chan struct{}, 1)
	done := make(chan struct{})
	defer close(done)

	go func() {
		inactivity := time.NewTimer(idleTimeout)
		defer inactivity.Stop()
		notifiedWaiting := false

		for {
			select {
			case <-done:
				return
			case <-activity:
				notifiedWaiting = false
				if !inactivity.Stop() {
					select {
					case <-inactivity.C:
					default:
					}
				}
				inactivity.Reset(idleTimeout)
			case <-inactivity.C:
				if st := s.Status(); st == agent.StatusBusy {
					if tail := s.logBuf.Tail(8192); looksLikeWaitingForInput(tail) {
						s.setStatus(agent.StatusWaiting)
						if !notifiedWaiting {
							notifiedWaiting = true
							notify.WaitingForInput(s.name, AgentName)
						}
					} else {
						s.setStatus(agent.StatusIdle)
					}
				}
				inactivity.Reset(idleTimeout)
			}
		}
	}()

	for {
		n, err := s.ptmx.Read(buf)
		if n > 0 {
			chunk := make([]byte, n)
			copy(chunk, buf[:n])
			s.logBuf.Write(chunk) //nolint:errcheck

			if st := s.Status(); st == agent.StatusIdle || st == agent.StatusWaiting {
				s.setStatus(agent.StatusBusy)
			}

			select {
			case activity <- struct{}{}:
			default:
			}

			s.subsMu.Lock()
			for _, ch := range s.subs {
				select {
				case ch <- chunk:
				default:
				}
			}
			s.subsMu.Unlock()
		}
		if err != nil {
			break
		}
	}

	s.setStatus(agent.StatusStopped)
	s.subsMu.Lock()
	for _, ch := range s.subs {
		close(ch)
	}
	s.subs = make(map[int]chan []byte)
	s.subsMu.Unlock()
}

func (s *session) waitStartup(ctx context.Context) error {
	raw, cancel := s.SubscribeRaw(64)
	defer cancel()

	deadline := time.NewTimer(startupTimeout)
	defer deadline.Stop()
	quiet := time.NewTimer(startupQuiet)
	defer quiet.Stop()
	gotData := false

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-deadline.C:
			return fmt.Errorf("timed out after %s", startupTimeout)
		case _, ok := <-raw:
			if !ok {
				return fmt.Errorf("process exited during startup")
			}
			gotData = true
			quiet.Reset(startupQuiet)
		case <-quiet.C:
			if gotData {
				return nil
			}
			quiet.Reset(startupQuiet)
		}
	}
}

func (s *session) Send(ctx context.Context, prompt string) (<-chan agent.Event, error) {
	s.mu.Lock()
	if s.status == agent.StatusStopped {
		s.mu.Unlock()
		return nil, fmt.Errorf("session %s is stopped", s.id)
	}
	if s.status == agent.StatusBusy {
		s.mu.Unlock()
		return nil, fmt.Errorf("session %s is busy", s.id)
	}
	s.status = agent.StatusBusy
	s.mu.Unlock()

	raw, cancel := s.SubscribeRaw(256)

	fmt.Fprintln(s.ptmx, prompt) //nolint:errcheck

	ch := make(chan agent.Event, 64)
	go func() {
		defer close(ch)
		defer cancel()
		defer func() {
			if s.Status() == agent.StatusBusy {
				s.setStatus(agent.StatusIdle)
			}
		}()

		inactivity := time.NewTimer(inactivityTimeout)
		defer inactivity.Stop()

		for {
			select {
			case <-ctx.Done():
				return
			case <-inactivity.C:
				ch <- agent.Event{Type: agent.EventDone, Timestamp: time.Now()}
				return
			case chunk, ok := <-raw:
				if !ok {
					ch <- agent.Event{Type: agent.EventError, Error: "agent process exited", Timestamp: time.Now()}
					return
				}
				inactivity.Reset(inactivityTimeout)
				ch <- agent.Event{Type: agent.EventText, Text: string(chunk), Timestamp: time.Now()}
			}
		}
	}()

	return ch, nil
}

func envSlice(m map[string]string) []string {
	out := make([]string, 0, len(m))
	for k, v := range m {
		out = append(out, k+"="+v)
	}
	return out
}

type safeBuffer struct {
	mu  sync.Mutex
	buf bytes.Buffer
}

func (b *safeBuffer) Write(p []byte) (int, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buf.Write(p)
}

func (b *safeBuffer) Reader() io.Reader {
	b.mu.Lock()
	defer b.mu.Unlock()
	cp := make([]byte, b.buf.Len())
	copy(cp, b.buf.Bytes())
	return bytes.NewReader(cp)
}

func (b *safeBuffer) Tail(n int) []byte {
	b.mu.Lock()
	defer b.mu.Unlock()
	data := b.buf.Bytes()
	if len(data) <= n {
		cp := make([]byte, len(data))
		copy(cp, data)
		return cp
	}
	cp := make([]byte, n)
	copy(cp, data[len(data)-n:])
	return cp
}
