// Package store provides SQLite-backed persistence for sessions, events, and nodes.
package store

import (
	"database/sql"
	"fmt"
	"time"

	_ "modernc.org/sqlite"
)

const schema = `
CREATE TABLE IF NOT EXISTS nodes (
	name       TEXT PRIMARY KEY,
	addr       TEXT NOT NULL DEFAULT '',
	token      TEXT NOT NULL DEFAULT '',
	tls        INTEGER NOT NULL DEFAULT 0,
	last_seen  DATETIME
);

CREATE TABLE IF NOT EXISTS sessions (
	id         TEXT PRIMARY KEY,
	name       TEXT NOT NULL,
	agent      TEXT NOT NULL,
	workspace  TEXT NOT NULL,
	model      TEXT NOT NULL DEFAULT '',
	node_name  TEXT NOT NULL DEFAULT 'local',
	status     TEXT NOT NULL DEFAULT 'starting',
	created_at DATETIME NOT NULL,
	updated_at DATETIME NOT NULL
);

CREATE TABLE IF NOT EXISTS events (
	id         INTEGER PRIMARY KEY AUTOINCREMENT,
	session_id TEXT NOT NULL REFERENCES sessions(id),
	type       TEXT NOT NULL,
	text       TEXT NOT NULL DEFAULT '',
	tool_name  TEXT NOT NULL DEFAULT '',
	tool_input TEXT NOT NULL DEFAULT '',
	error      TEXT NOT NULL DEFAULT '',
	timestamp  DATETIME NOT NULL
);

CREATE TABLE IF NOT EXISTS tasks (
	id          INTEGER PRIMARY KEY AUTOINCREMENT,
	session_id  TEXT NOT NULL REFERENCES sessions(id),
	prompt      TEXT NOT NULL,
	status      TEXT NOT NULL DEFAULT 'pending',
	created_at  DATETIME NOT NULL,
	finished_at DATETIME
);
`

// NodeRecord mirrors the nodes table row.
type NodeRecord struct {
	Name     string     `json:"name"`
	Addr     string     `json:"addr"`
	Token    string     `json:"token,omitempty"` // omit from JSON responses
	TLS      bool       `json:"tls"`
	LastSeen *time.Time `json:"last_seen,omitempty"`
}

// Session mirrors the sessions table row.
type Session struct {
	ID        string    `json:"id"`
	Name      string    `json:"name"`
	Agent     string    `json:"agent"`
	Workspace string    `json:"workspace"`
	Model     string    `json:"model"`
	NodeName  string    `json:"node_name"`
	Status    string    `json:"status"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// Event mirrors the events table row.
type Event struct {
	ID        int64     `json:"id"`
	SessionID string    `json:"session_id"`
	Type      string    `json:"type"`
	Text      string    `json:"text"`
	ToolName  string    `json:"tool_name"`
	ToolInput string    `json:"tool_input"`
	Error     string    `json:"error"`
	Timestamp time.Time `json:"timestamp"`
}

// Store is the SQLite-backed persistence layer.
type Store struct {
	db *sql.DB
}

// Open opens (or creates) the SQLite database at path.
func Open(path string) (*Store, error) {
	db, err := sql.Open("sqlite", path)
	if err != nil {
		return nil, fmt.Errorf("open db: %w", err)
	}
	db.SetMaxOpenConns(1) // SQLite is single-writer
	if _, err := db.Exec(schema); err != nil {
		return nil, fmt.Errorf("apply schema: %w", err)
	}
	return &Store{db: db}, nil
}

func (s *Store) Close() error { return s.db.Close() }

// --- Sessions ---

func (s *Store) CreateSession(sess Session) error {
	if sess.NodeName == "" {
		sess.NodeName = "local"
	}
	_, err := s.db.Exec(
		`INSERT INTO sessions (id, name, agent, workspace, model, node_name, status, created_at, updated_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		sess.ID, sess.Name, sess.Agent, sess.Workspace, sess.Model,
		sess.NodeName, sess.Status, sess.CreatedAt, sess.UpdatedAt,
	)
	return err
}

func (s *Store) UpdateSessionStatus(id, status string) error {
	_, err := s.db.Exec(
		`UPDATE sessions SET status = ?, updated_at = ? WHERE id = ?`,
		status, time.Now(), id,
	)
	return err
}

func (s *Store) GetSession(id string) (*Session, error) {
	row := s.db.QueryRow(
		`SELECT id, name, agent, workspace, model, node_name, status, created_at, updated_at FROM sessions WHERE id = ?`, id,
	)
	return scanSession(row)
}

func (s *Store) ListSessions() ([]Session, error) {
	rows, err := s.db.Query(
		`SELECT id, name, agent, workspace, model, node_name, status, created_at, updated_at
		 FROM sessions ORDER BY created_at DESC`,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Session
	for rows.Next() {
		sess, err := scanSession(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, *sess)
	}
	return out, rows.Err()
}

func (s *Store) DeleteSession(id string) error {
	res, err := s.db.Exec(`DELETE FROM sessions WHERE id = ?`, id)
	if err != nil {
		return err
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return fmt.Errorf("session %s not found", id)
	}
	return nil
}

type scanner interface {
	Scan(dest ...any) error
}

func scanSession(r scanner) (*Session, error) {
	var sess Session
	err := r.Scan(
		&sess.ID, &sess.Name, &sess.Agent, &sess.Workspace, &sess.Model,
		&sess.NodeName, &sess.Status, &sess.CreatedAt, &sess.UpdatedAt,
	)
	if err != nil {
		return nil, err
	}
	return &sess, nil
}

// --- Nodes ---

func (s *Store) UpsertNode(n NodeRecord) error {
	_, err := s.db.Exec(
		`INSERT INTO nodes (name, addr, token, tls) VALUES (?, ?, ?, ?)
		 ON CONFLICT(name) DO UPDATE SET addr=excluded.addr, token=excluded.token, tls=excluded.tls`,
		n.Name, n.Addr, n.Token, n.TLS,
	)
	return err
}

func (s *Store) TouchNode(name string) error {
	_, err := s.db.Exec(`UPDATE nodes SET last_seen = ? WHERE name = ?`, time.Now(), name)
	return err
}

func (s *Store) GetNode(name string) (*NodeRecord, error) {
	row := s.db.QueryRow(`SELECT name, addr, token, tls, last_seen FROM nodes WHERE name = ?`, name)
	return scanNode(row)
}

func (s *Store) ListNodes() ([]NodeRecord, error) {
	rows, err := s.db.Query(`SELECT name, addr, token, tls, last_seen FROM nodes ORDER BY name`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []NodeRecord
	for rows.Next() {
		n, err := scanNode(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, *n)
	}
	return out, rows.Err()
}

func (s *Store) DeleteNode(name string) error {
	_, err := s.db.Exec(`DELETE FROM nodes WHERE name = ?`, name)
	return err
}

func scanNode(r scanner) (*NodeRecord, error) {
	var n NodeRecord
	var tlsInt int
	err := r.Scan(&n.Name, &n.Addr, &n.Token, &tlsInt, &n.LastSeen)
	if err != nil {
		return nil, err
	}
	n.TLS = tlsInt != 0
	return &n, nil
}

// --- Events ---

func (s *Store) AppendEvent(e Event) error {
	_, err := s.db.Exec(
		`INSERT INTO events (session_id, type, text, tool_name, tool_input, error, timestamp)
		 VALUES (?, ?, ?, ?, ?, ?, ?)`,
		e.SessionID, e.Type, e.Text, e.ToolName, e.ToolInput, e.Error, e.Timestamp,
	)
	return err
}

func (s *Store) ListEvents(sessionID string) ([]Event, error) {
	rows, err := s.db.Query(
		`SELECT id, session_id, type, text, tool_name, tool_input, error, timestamp
		 FROM events WHERE session_id = ? ORDER BY id`,
		sessionID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Event
	for rows.Next() {
		var e Event
		if err := rows.Scan(&e.ID, &e.SessionID, &e.Type, &e.Text, &e.ToolName, &e.ToolInput, &e.Error, &e.Timestamp); err != nil {
			return nil, err
		}
		out = append(out, e)
	}
	return out, rows.Err()
}
