// Package api provides the REST + WebSocket HTTP server.
package api

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/gorilla/websocket"

	"github.com/ash-thakur-rh/meru/internal/agent"
	"github.com/ash-thakur-rh/meru/internal/gitclone"
	"github.com/ash-thakur-rh/meru/internal/node"
	"github.com/ash-thakur-rh/meru/internal/session"
	"github.com/ash-thakur-rh/meru/internal/store"
)

// Server wires HTTP routes to the session Manager.
type Server struct {
	mgr      *session.Manager
	store    *store.Store
	clones   *gitclone.Manager
	upgrader websocket.Upgrader
}

// New creates a Server backed by mgr and st.
func New(mgr *session.Manager, st *store.Store) *Server {
	return &Server{
		mgr:    mgr,
		store:  st,
		clones: gitclone.New(),
		upgrader: websocket.Upgrader{
			CheckOrigin: func(r *http.Request) bool { return true },
		},
	}
}

// Handler returns the root http.Handler.
// uiHandler serves the embedded web UI; if nil, the UI is disabled.
func (s *Server) Handler(uiHandler http.Handler) http.Handler {
	r := chi.NewRouter()
	r.Use(middleware.Logger)
	r.Use(middleware.Recoverer)

	r.Get("/healthz", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		s.handleHealthz(w, r)
	})
	r.With(jsonContentType).Post("/broadcast", s.handleBroadcast)

	r.With(jsonContentType).Route("/nodes", func(r chi.Router) {
		r.Get("/", s.handleListNodes)
		r.Post("/", s.handleAddNode)
		r.Delete("/{name}", s.handleRemoveNode)
		r.Post("/{name}/ping", s.handlePingNode)
	})

	r.With(jsonContentType).Get("/fs", s.handleListDir)
	r.Route("/git/clone", func(r chi.Router) {
		r.With(jsonContentType).Post("/", s.handleGitCloneStart)
		r.Get("/{id}/stream", s.handleGitCloneStream)
		r.Delete("/{id}", s.handleGitCloneCancel)
	})

	r.With(jsonContentType).Route("/sessions", func(r chi.Router) {
		r.Post("/", s.handleSpawn)
		r.Get("/", s.handleList)
		r.Route("/{id}", func(r chi.Router) {
			r.Get("/", s.handleGet)
			r.Delete("/", s.handleStop)
			r.Post("/send", s.handleSend)
			r.Get("/logs", s.handleLogs)
			r.Get("/stream", s.handleStream)     // WebSocket: structured event broadcast
			r.Get("/terminal", s.handleTerminal) // WebSocket: bidirectional raw PTY bridge
		})
	})

	// Mount the web UI at / (must be last so API routes take priority)
	if uiHandler != nil {
		r.Handle("/*", uiHandler)
	}

	return r
}

// --- handlers ---

func (s *Server) handleHealthz(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

// POST /sessions
type spawnRequest struct {
	Agent      string            `json:"agent"`
	Name       string            `json:"name"`
	Workspace  string            `json:"workspace"`
	Model      string            `json:"model"`
	Env        map[string]string `json:"env"`
	Worktree   bool              `json:"worktree"`
	Node       string            `json:"node"`
	BranchName string            `json:"branch_name"`
}

func (s *Server) handleSpawn(w http.ResponseWriter, r *http.Request) {
	var req spawnRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON: "+err.Error())
		return
	}
	if req.Agent == "" {
		writeError(w, http.StatusBadRequest, "agent is required")
		return
	}
	if req.Workspace == "" {
		req.Workspace = "."
	}

	sess, err := s.mgr.Spawn(r.Context(), req.Agent, agent.SpawnConfig{
		Name:       req.Name,
		Workspace:  req.Workspace,
		Model:      req.Model,
		Env:        req.Env,
		Worktree:   req.Worktree,
		NodeName:   req.Node,
		BranchName: req.BranchName,
	})
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusCreated, sessionView(sess))
}

// GET /sessions
func (s *Server) handleList(w http.ResponseWriter, r *http.Request) {
	sessions, err := s.mgr.ListFromStore()
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, sessions)
}

// GET /sessions/:id
func (s *Server) handleGet(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")

	// Live session — return real-time status.
	if sess, err := s.mgr.Get(id); err == nil {
		writeJSON(w, http.StatusOK, sessionView(sess))
		return
	}

	// Stopped session — serve from the persistent store.
	stored, err := s.store.GetSession(id)
	if err != nil {
		writeError(w, http.StatusNotFound, "session not found")
		return
	}
	writeJSON(w, http.StatusOK, stored)
}

// DELETE /sessions/:id
// If the session is live it is stopped and kept in the store as "stopped".
// If it is already stopped the record is purged from the store permanently.
func (s *Server) handleStop(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	if err := s.mgr.Stop(id); err != nil {
		// Not live — try to purge the stopped record instead.
		if err2 := s.mgr.Purge(id); err2 != nil {
			writeError(w, http.StatusNotFound, err.Error())
			return
		}
	}
	w.WriteHeader(http.StatusNoContent)
}

// POST /sessions/:id/send
type sendRequest struct {
	Prompt string `json:"prompt"`
}

func (s *Server) handleSend(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	var req sendRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON: "+err.Error())
		return
	}
	if req.Prompt == "" {
		writeError(w, http.StatusBadRequest, "prompt is required")
		return
	}

	ch, err := s.mgr.Send(r.Context(), id, req.Prompt)
	if err != nil {
		// A busy session is a client-side conflict, not a server error.
		if strings.Contains(err.Error(), "is busy") {
			writeError(w, http.StatusConflict, err.Error())
			return
		}
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	// Stream nd-JSON events as SSE-style response
	w.Header().Set("Content-Type", "application/x-ndjson")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	flusher, _ := w.(http.Flusher)
	enc := json.NewEncoder(w)
	for ev := range ch {
		enc.Encode(ev) //nolint:errcheck
		if flusher != nil {
			flusher.Flush()
		}
	}
}

// GET /sessions/:id/logs
func (s *Server) handleLogs(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	events, err := s.mgr.History(id)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, events)
}

// POST /broadcast
type broadcastRequest struct {
	Prompt   string   `json:"prompt"`
	Sessions []string `json:"sessions"` // empty = all active
}

func (s *Server) handleBroadcast(w http.ResponseWriter, r *http.Request) {
	var req broadcastRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON: "+err.Error())
		return
	}
	if req.Prompt == "" {
		writeError(w, http.StatusBadRequest, "prompt is required")
		return
	}
	results := s.mgr.Broadcast(r.Context(), req.Prompt, req.Sessions)
	writeJSON(w, http.StatusOK, results)
}

// GET /sessions/:id/stream  (WebSocket)
// Client receives SessionEvents in real-time.
func (s *Server) handleStream(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")

	conn, err := s.upgrader.Upgrade(w, r, nil)
	if err != nil {
		return
	}
	defer conn.Close()

	_, ch, cancel := s.mgr.Subscribe()
	defer cancel()

	for {
		select {
		case ev, ok := <-ch:
			if !ok {
				return
			}
			if ev.SessionID != id {
				continue
			}
			data, _ := json.Marshal(ev.Event)
			if err := conn.WriteMessage(websocket.TextMessage, data); err != nil {
				return
			}
		case <-r.Context().Done():
			return
		}
	}
}

// GET /sessions/:id/terminal  (WebSocket — bidirectional raw PTY bridge)
//
// For live sessions this is a full bidirectional PTY stream:
//
//   - PTY output → binary WebSocket frames → xterm.js
//   - xterm.js keystrokes → binary WebSocket frames → PTY stdin
//   - resize events → JSON text frames {"type":"resize","cols":N,"rows":N}
//
// For stopped sessions the handler upgrades the WebSocket, replays the
// stored event log as terminal output, then closes cleanly.  This lets
// the browser show the same terminal view regardless of session state.
//
// Closing the WebSocket does NOT stop the underlying session.
func (s *Server) handleTerminal(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")

	// ── Stopped session path ──────────────────────────────────────────────
	sess, liveErr := s.mgr.Get(id)
	if liveErr != nil {
		// Check the store so we can serve history for stopped sessions.
		stored, serr := s.store.GetSession(id)
		if serr != nil {
			writeError(w, http.StatusNotFound, "session not found")
			return
		}

		conn, uerr := s.upgrader.Upgrade(w, r, nil)
		if uerr != nil {
			return
		}
		defer conn.Close()

		// Replay stored text events as raw terminal bytes.
		events, _ := s.mgr.History(id)
		var history []byte
		for _, ev := range events {
			if ev.Type == "text" {
				history = append(history, ev.Text...)
			}
		}
		if len(history) > 0 {
			conn.WriteMessage(websocket.BinaryMessage, history) //nolint:errcheck
		}
		// Dim "session stopped" marker at the bottom.
		trailer := "\r\n\x1b[2m── " + stored.Status + " ──\x1b[0m\r\n"
		conn.WriteMessage(websocket.BinaryMessage, []byte(trailer)) //nolint:errcheck
		return
	}

	// ── Live session path ─────────────────────────────────────────────────
	ptySess, ok := sess.(agent.PTYSession)
	if !ok {
		http.Error(w, "session does not support terminal streaming", http.StatusNotImplemented)
		return
	}

	conn, err := s.upgrader.Upgrade(w, r, nil)
	if err != nil {
		return
	}
	defer conn.Close()

	// Send accumulated log history as a single binary frame so the browser
	// can replay what happened before this connection was opened.
	if history, rerr := io.ReadAll(sess.Logs()); rerr == nil && len(history) > 0 {
		conn.WriteMessage(websocket.BinaryMessage, history) //nolint:errcheck
	}

	// Subscribe to live PTY output.
	ch, cancelSub := ptySess.SubscribeRaw(4096)
	defer cancelSub()

	ctx, ctxCancel := context.WithCancel(r.Context())
	defer ctxCancel()

	// PTY → WebSocket goroutine.
	go func() {
		defer ctxCancel()
		for {
			select {
			case chunk, ok := <-ch:
				if !ok {
					return // PTY closed; let the read loop below also exit
				}
				if err := conn.WriteMessage(websocket.BinaryMessage, chunk); err != nil {
					return
				}
			case <-ctx.Done():
				return
			}
		}
	}()

	// WebSocket → PTY (main loop, runs until client disconnects).
	for {
		mt, msg, err := conn.ReadMessage()
		if err != nil {
			break
		}
		switch mt {
		case websocket.TextMessage:
			// Resize event sent by the frontend when the terminal window changes size.
			var ev struct {
				Type string `json:"type"`
				Cols uint16 `json:"cols"`
				Rows uint16 `json:"rows"`
			}
			if json.Unmarshal(msg, &ev) == nil && ev.Type == "resize" {
				ptySess.ResizePTY(ev.Cols, ev.Rows) //nolint:errcheck
			}
		case websocket.BinaryMessage:
			// Raw keystroke data from xterm.js — forward directly to PTY stdin.
			ptySess.WriteInput(msg) //nolint:errcheck
		}
	}
}

// --- helpers ---

type sessionJSON struct {
	ID        string `json:"id"`
	Name      string `json:"name"`
	Agent     string `json:"agent"`
	Workspace string `json:"workspace"`
	Status    string `json:"status"`
	NodeName  string `json:"node_name"`
}

func sessionView(sess agent.Session) sessionJSON {
	return sessionJSON{
		ID:        sess.ID(),
		Name:      sess.Name(),
		Agent:     sess.AgentName(),
		Workspace: sess.Workspace(),
		Status:    string(sess.Status()),
		NodeName:  "local", // live sessions served here are always local
	}
}

func writeJSON(w http.ResponseWriter, code int, v any) {
	w.WriteHeader(code)
	json.NewEncoder(w).Encode(v) //nolint:errcheck
}

func writeError(w http.ResponseWriter, code int, msg string) {
	writeJSON(w, code, map[string]string{"error": msg})
}

func jsonContentType(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		next.ServeHTTP(w, r)
	})
}

// --- Git ---

// POST /git/clone
type gitCloneRequest struct {
	URL      string `json:"url"`
	Dest     string `json:"dest"`
	NodeName string `json:"node"`
	Username string `json:"username"`
	Password string `json:"password"`
}

// POST /git/clone — starts an async clone and returns a job ID immediately.
func (s *Server) handleGitCloneStart(w http.ResponseWriter, r *http.Request) {
	var req gitCloneRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON: "+err.Error())
		return
	}
	if req.URL == "" {
		writeError(w, http.StatusBadRequest, "url is required")
		return
	}
	if req.NodeName == "" {
		req.NodeName = node.LocalNodeName
	}

	n, err := node.Get(req.NodeName)
	if err != nil {
		writeError(w, http.StatusNotFound, "node not found: "+err.Error())
		return
	}

	var jobID string
	if req.NodeName == node.LocalNodeName {
		jobID = s.clones.StartLocal(req.URL, req.Dest, req.Username, req.Password)
	} else {
		captured := req
		capturedNode := n
		jobID = s.clones.StartRemote(func(ctx context.Context) (string, error) {
			return capturedNode.GitClone(ctx, captured.URL, captured.Dest, captured.Username, captured.Password)
		})
	}

	writeJSON(w, http.StatusAccepted, map[string]string{"jobId": jobID})
}

// GET /git/clone/{id}/stream — SSE stream of clone progress.
func (s *Server) handleGitCloneStream(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	job, ok := s.clones.Get(id)
	if !ok {
		writeError(w, http.StatusNotFound, "clone job not found")
		return
	}

	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("X-Accel-Buffering", "no")
	flusher, _ := w.(http.Flusher)

	sent := 0

	sendEvent := func(event, data string) {
		fmt.Fprintf(w, "event: %s\ndata: %s\n\n", event, data)
		if flusher != nil {
			flusher.Flush()
		}
	}

	for {
		lines, pct, done, path, jobErr := job.Snapshot()

		for ; sent < len(lines); sent++ {
			payload, _ := json.Marshal(map[string]any{"line": lines[sent], "pct": pct})
			sendEvent("log", string(payload))
		}

		if done {
			if jobErr != nil {
				payload, _ := json.Marshal(map[string]string{"message": jobErr.Error()})
				sendEvent("error", string(payload))
			} else {
				payload, _ := json.Marshal(map[string]string{"path": path})
				sendEvent("done", string(payload))
			}
			return
		}

		select {
		case <-job.Wait():
		case <-r.Context().Done():
			return
		}
	}
}

// DELETE /git/clone/{id} — cancel an in-progress clone.
func (s *Server) handleGitCloneCancel(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	if !s.clones.Cancel(id) {
		writeError(w, http.StatusNotFound, "clone job not found")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// --- Filesystem ---

// GET /fs?path=<dir>&node=<name>
func (s *Server) handleListDir(w http.ResponseWriter, r *http.Request) {
	path := r.URL.Query().Get("path")
	nodeName := r.URL.Query().Get("node")
	if nodeName == "" {
		nodeName = node.LocalNodeName
	}

	n, err := node.Get(nodeName)
	if err != nil {
		writeError(w, http.StatusNotFound, "node not found: "+err.Error())
		return
	}

	listing, err := n.ListDir(r.Context(), path)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, listing)
}

// --- Node handlers ---

// GET /nodes
func (s *Server) handleListNodes(w http.ResponseWriter, r *http.Request) {
	nodes, err := s.store.ListNodes()
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	// Scrub tokens from the response
	for i := range nodes {
		nodes[i].Token = ""
	}
	if nodes == nil {
		nodes = []store.NodeRecord{}
	}
	writeJSON(w, http.StatusOK, nodes)
}

// POST /nodes
type addNodeRequest struct {
	Name  string `json:"name"`
	Addr  string `json:"addr"`
	Token string `json:"token"`
	TLS   bool   `json:"tls"`
}

func (s *Server) handleAddNode(w http.ResponseWriter, r *http.Request) {
	var req addNodeRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON: "+err.Error())
		return
	}
	if req.Name == "" || req.Addr == "" || req.Token == "" {
		writeError(w, http.StatusBadRequest, "name, addr, and token are required")
		return
	}
	if req.Name == node.LocalNodeName {
		writeError(w, http.StatusBadRequest, `"local" is reserved`)
		return
	}

	rec := store.NodeRecord{Name: req.Name, Addr: req.Addr, Token: req.Token, TLS: req.TLS}
	if err := s.store.UpsertNode(rec); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	// Dial and register in the live registry
	n, err := node.NewGRPCNode(r.Context(), node.Info{
		Name:  req.Name,
		Addr:  req.Addr,
		Token: req.Token,
		TLS:   req.TLS,
	})
	if err != nil {
		writeError(w, http.StatusBadGateway, "node added to store but dial failed: "+err.Error())
		return
	}
	node.Register(n)

	rec.Token = "" // scrub before responding
	writeJSON(w, http.StatusCreated, rec)
}

// DELETE /nodes/:name
func (s *Server) handleRemoveNode(w http.ResponseWriter, r *http.Request) {
	name := chi.URLParam(r, "name")
	if name == node.LocalNodeName {
		writeError(w, http.StatusBadRequest, "cannot remove local node")
		return
	}
	if err := node.Unregister(name); err != nil {
		writeError(w, http.StatusNotFound, err.Error())
		return
	}
	_ = s.store.DeleteNode(name)
	w.WriteHeader(http.StatusNoContent)
}

// POST /nodes/:name/ping
func (s *Server) handlePingNode(w http.ResponseWriter, r *http.Request) {
	name := chi.URLParam(r, "name")
	n, err := node.Get(name)
	if err != nil {
		writeError(w, http.StatusNotFound, err.Error())
		return
	}
	info, err := n.Ping(context.Background())
	if err != nil {
		writeError(w, http.StatusBadGateway, err.Error())
		return
	}
	_ = s.store.TouchNode(name)
	writeJSON(w, http.StatusOK, info)
}
