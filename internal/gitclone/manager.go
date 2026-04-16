package gitclone

import (
	"bufio"
	"context"
	"fmt"
	"os/exec"
	"regexp"
	"strconv"
	"strings"
	"sync"

	"github.com/google/uuid"
)

var pctRe = regexp.MustCompile(`(?i)(?:Receiving|Resolving) objects:\s+(\d+)%`)

// CloneJob tracks an in-progress (or finished) git clone operation.
type CloneJob struct {
	ID string

	mu     sync.RWMutex
	lines  []string
	pct    int
	done   bool
	path   string // set on success
	err    error
	cancel context.CancelFunc
	notify chan struct{} // closed when state changes; always replaced, never reused
}

// Snapshot returns a consistent read of the job's current state.
func (j *CloneJob) Snapshot() (lines []string, pct int, done bool, path string, err error) {
	j.mu.RLock()
	defer j.mu.RUnlock()
	cp := make([]string, len(j.lines))
	copy(cp, j.lines)
	return cp, j.pct, j.done, j.path, j.err
}

// Wait returns a channel that is closed whenever new lines arrive or the job
// finishes. Call Snapshot after receiving from this channel to get updated state.
func (j *CloneJob) Wait() <-chan struct{} {
	j.mu.RLock()
	ch := j.notify
	j.mu.RUnlock()
	return ch
}

func (j *CloneJob) appendLine(line string) {
	pct := 0
	if m := pctRe.FindStringSubmatch(line); m != nil {
		if v, err := strconv.Atoi(m[1]); err == nil {
			pct = v
		}
	}

	j.mu.Lock()
	j.lines = append(j.lines, line)
	if pct > j.pct {
		j.pct = pct
	}
	old := j.notify
	j.notify = make(chan struct{})
	j.mu.Unlock()

	close(old)
}

func (j *CloneJob) finish(path string, err error) {
	j.mu.Lock()
	j.done = true
	j.path = path
	j.err = err
	old := j.notify
	j.notify = make(chan struct{})
	j.mu.Unlock()

	close(old)
}

// Manager owns a set of CloneJobs.
type Manager struct {
	mu   sync.RWMutex
	jobs map[string]*CloneJob
}

// New returns a ready Manager.
func New() *Manager {
	return &Manager{jobs: make(map[string]*CloneJob)}
}

// Get returns the job with the given ID.
func (m *Manager) Get(id string) (*CloneJob, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	j, ok := m.jobs[id]
	return j, ok
}

// Cancel cancels a running job. Returns false if the job is not found.
func (m *Manager) Cancel(id string) bool {
	m.mu.RLock()
	j, ok := m.jobs[id]
	m.mu.RUnlock()
	if !ok {
		return false
	}
	j.cancel()
	return true
}

// StartLocal starts an async local git clone using the system git binary.
func (m *Manager) StartLocal(url, dest, username, password string) string {
	ctx, cancel := context.WithCancel(context.Background())
	j := &CloneJob{
		ID:     uuid.New().String(),
		cancel: cancel,
		notify: make(chan struct{}),
	}
	m.mu.Lock()
	m.jobs[j.ID] = j
	m.mu.Unlock()

	go func() {
		args := []string{"clone", "--progress", url}
		if dest != "" {
			args = append(args, dest)
		}
		cmd := exec.CommandContext(ctx, "git", args...)
		if username != "" || password != "" {
			cmd.Env = append(cmd.Environ(),
				"GIT_TERMINAL_PROMPT=0",
				"GIT_ASKPASS=echo",
			)
			rewritten := embedCreds(url, username, password)
			args[len(args)-func() int {
				if dest != "" {
					return 2
				}
				return 1
			}()] = rewritten
			cmd = exec.CommandContext(ctx, "git", args...)
		}

		stderr, err := cmd.StderrPipe()
		if err != nil {
			j.finish("", fmt.Errorf("pipe: %w", err))
			return
		}

		if err := cmd.Start(); err != nil {
			j.finish("", fmt.Errorf("start: %w", err))
			return
		}

		scanner := bufio.NewScanner(stderr)
		scanner.Split(scanCRLF)
		for scanner.Scan() {
			line := strings.TrimSpace(scanner.Text())
			if line != "" {
				j.appendLine(line)
			}
		}

		if err := cmd.Wait(); err != nil {
			if ctx.Err() != nil {
				j.finish("", fmt.Errorf("cancelled"))
			} else {
				j.finish("", fmt.Errorf("git clone: %w", err))
			}
			return
		}

		clonedPath := dest
		if clonedPath == "" {
			clonedPath = repoName(url)
		}
		j.finish(clonedPath, nil)
	}()

	return j.ID
}

// StartRemote wraps a blocking clone function as an async job.
func (m *Manager) StartRemote(fn func(ctx context.Context) (string, error)) string {
	ctx, cancel := context.WithCancel(context.Background())
	j := &CloneJob{
		ID:     uuid.New().String(),
		cancel: cancel,
		notify: make(chan struct{}),
	}
	m.mu.Lock()
	m.jobs[j.ID] = j
	m.mu.Unlock()

	j.appendLine("Cloning on remote node\u2026")

	go func() {
		path, err := fn(ctx)
		j.finish(path, err)
	}()

	return j.ID
}

// scanCRLF splits on \r or \n to capture git's carriage-return progress lines.
func scanCRLF(data []byte, atEOF bool) (advance int, token []byte, err error) {
	for i, b := range data {
		if b == '\r' || b == '\n' {
			return i + 1, data[:i], nil
		}
	}
	if atEOF && len(data) > 0 {
		return len(data), data, nil
	}
	return 0, nil, nil
}

// embedCreds rewrites an https URL to include username:password.
func embedCreds(rawURL, username, password string) string {
	if !strings.HasPrefix(rawURL, "https://") {
		return rawURL
	}
	rest := strings.TrimPrefix(rawURL, "https://")
	return fmt.Sprintf("https://%s:%s@%s", username, password, rest)
}

// repoName extracts a repository directory name from a git URL.
func repoName(rawURL string) string {
	base := strings.TrimSuffix(rawURL, ".git")
	for _, sep := range []string{"/", ":"} {
		if i := strings.LastIndex(base, sep); i >= 0 {
			base = base[i+1:]
		}
	}
	if base == "" {
		return "repo"
	}
	return base
}
