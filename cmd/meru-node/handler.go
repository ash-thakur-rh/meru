package main

import (
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	gogit "github.com/go-git/go-git/v6"
	githttp "github.com/go-git/go-git/v6/plumbing/transport/http"
	gitssh "github.com/go-git/go-git/v6/plumbing/transport/ssh"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"

	"github.com/ash-thakur-rh/meru/internal/agent"
	pb "github.com/ash-thakur-rh/meru/internal/proto"
)

// nodeHandler implements the MeruNode gRPC service.
// It maintains an in-process map of sessions spawned on this node.
type nodeHandler struct {
	pb.UnimplementedMeruNodeServer

	mu       sync.RWMutex
	sessions map[string]agent.Session // session ID → live session
	events   map[string][]agent.Event // session ID → persisted events
}

func newNodeHandler() *nodeHandler {
	return &nodeHandler{
		sessions: make(map[string]agent.Session),
		events:   make(map[string][]agent.Event),
	}
}

// Ping -------------------------------------------------------------------

func (h *nodeHandler) Ping(_ context.Context, _ *pb.PingRequest) (*pb.PingResponse, error) {
	hostname, _ := os.Hostname()
	return &pb.PingResponse{
		Version:  "meru-node/1.0",
		Agents:   agent.List(),
		Hostname: hostname,
	}, nil
}

// Spawn ------------------------------------------------------------------

func (h *nodeHandler) Spawn(_ context.Context, req *pb.SpawnRequest) (*pb.SpawnResponse, error) {
	a, err := agent.Get(req.Agent)
	if err != nil {
		return nil, status.Errorf(codes.NotFound, "agent %q not available on this node", req.Agent)
	}

	workspace := req.Workspace

	// If a git worktree is requested, create one on this node before spawning.
	if req.Worktree && workspace != "" {
		_, gitErr := gogit.PlainOpen(workspace)
		if gitErr == nil {
			worktreeDir := filepath.Join(workspace, ".meru-worktrees")
			if err := os.MkdirAll(worktreeDir, 0o755); err != nil {
				return nil, status.Errorf(codes.Internal, "create worktrees dir: %v", err)
			}
			// Use the session ID as the unique directory name; use BranchName for the branch.
			worktreePath := filepath.Join(worktreeDir, req.SessionId)
			branchSlug := req.BranchName
			if branchSlug == "" {
				branchSlug = req.SessionId[:8]
			}
			branch := "meru/" + branchSlug
			addCmd := exec.Command("git", "-C", workspace,
				"worktree", "add", "-b", branch, worktreePath, "HEAD")
			if out, err := addCmd.CombinedOutput(); err != nil {
				return nil, status.Errorf(codes.Internal, "git worktree add: %v\n%s", err, out)
			}
			workspace = worktreePath
		}
	}

	cfg := agent.SpawnConfig{
		Name:      req.Name,
		Workspace: workspace,
		Model:     req.Model,
		Env:       req.Env,
	}

	sess, err := a.Spawn(context.Background(), cfg)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "spawn: %v", err)
	}

	h.mu.Lock()
	h.sessions[req.SessionId] = sess
	h.events[req.SessionId] = nil
	h.mu.Unlock()

	return &pb.SpawnResponse{
		SessionId: req.SessionId,
		Name:      sess.Name(),
		Workspace: sess.Workspace(),
	}, nil
}

// Send -------------------------------------------------------------------

func (h *nodeHandler) Send(req *pb.SendRequest, stream pb.MeruNode_SendServer) error {
	h.mu.RLock()
	sess, ok := h.sessions[req.SessionId]
	h.mu.RUnlock()
	if !ok {
		return status.Errorf(codes.NotFound, "session %s not found", req.SessionId)
	}

	ch, err := sess.Send(stream.Context(), req.Prompt)
	if err != nil {
		return status.Errorf(codes.Internal, "send: %v", err)
	}

	for ev := range ch {
		// Persist locally
		h.mu.Lock()
		h.events[req.SessionId] = append(h.events[req.SessionId], ev)
		h.mu.Unlock()

		msg := eventToProto(ev)
		if err := stream.Send(msg); err != nil {
			// Client disconnected — drain the channel to avoid goroutine leak
			go func() {
				for range ch {
				}
			}()
			return err
		}
	}
	return nil
}

// Stop -------------------------------------------------------------------

func (h *nodeHandler) Stop(_ context.Context, req *pb.StopRequest) (*pb.StopResponse, error) {
	h.mu.Lock()
	sess, ok := h.sessions[req.SessionId]
	if ok {
		delete(h.sessions, req.SessionId)
	}
	h.mu.Unlock()
	if !ok {
		return nil, status.Errorf(codes.NotFound, "session %s not found", req.SessionId)
	}
	if err := sess.Stop(); err != nil {
		return nil, status.Errorf(codes.Internal, "stop: %v", err)
	}
	return &pb.StopResponse{}, nil
}

// GetSession -------------------------------------------------------------

func (h *nodeHandler) GetSession(_ context.Context, req *pb.GetSessionRequest) (*pb.SessionInfo, error) {
	h.mu.RLock()
	sess, ok := h.sessions[req.SessionId]
	h.mu.RUnlock()
	if !ok {
		return nil, status.Errorf(codes.NotFound, "session %s not found", req.SessionId)
	}
	return &pb.SessionInfo{
		SessionId: sess.ID(),
		Name:      sess.Name(),
		Agent:     sess.AgentName(),
		Workspace: sess.Workspace(),
		Status:    string(sess.Status()),
	}, nil
}

// ListSessions -----------------------------------------------------------

func (h *nodeHandler) ListSessions(_ context.Context, _ *pb.Empty) (*pb.ListSessionsResponse, error) {
	h.mu.RLock()
	defer h.mu.RUnlock()
	var infos []*pb.SessionInfo
	for id, sess := range h.sessions {
		infos = append(infos, &pb.SessionInfo{
			SessionId: id,
			Name:      sess.Name(),
			Agent:     sess.AgentName(),
			Workspace: sess.Workspace(),
			Status:    string(sess.Status()),
		})
	}
	return &pb.ListSessionsResponse{Sessions: infos}, nil
}

// GetLogs ----------------------------------------------------------------

func (h *nodeHandler) GetLogs(_ context.Context, req *pb.GetLogsRequest) (*pb.GetLogsResponse, error) {
	h.mu.RLock()
	evs, ok := h.events[req.SessionId]
	h.mu.RUnlock()
	if !ok {
		return nil, status.Errorf(codes.NotFound, "session %s not found", req.SessionId)
	}
	msgs := make([]*pb.EventMessage, len(evs))
	for i, ev := range evs {
		msgs[i] = eventToProto(ev)
	}
	return &pb.GetLogsResponse{Events: msgs}, nil
}

// ListDir ----------------------------------------------------------------

func (h *nodeHandler) ListDir(_ context.Context, req *pb.ListDirRequest) (*pb.ListDirResponse, error) {
	path := req.Path
	if path == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			path = "/"
		} else {
			path = home
		}
	}

	abs, err := filepath.Abs(path)
	if err != nil {
		return nil, status.Errorf(codes.InvalidArgument, "invalid path: %v", err)
	}

	rawEntries, err := os.ReadDir(abs)
	if err != nil {
		return nil, status.Errorf(codes.NotFound, "read dir: %v", err)
	}

	entries := make([]*pb.DirEntry, 0, len(rawEntries))
	for _, e := range rawEntries {
		if len(e.Name()) > 0 && e.Name()[0] == '.' {
			continue
		}
		entries = append(entries, &pb.DirEntry{
			Name:  e.Name(),
			Path:  filepath.Join(abs, e.Name()),
			IsDir: e.IsDir(),
		})
	}

	sort.Slice(entries, func(i, j int) bool {
		if entries[i].IsDir != entries[j].IsDir {
			return entries[i].IsDir
		}
		return entries[i].Name < entries[j].Name
	})

	parent := ""
	if abs != filepath.Dir(abs) {
		parent = filepath.Dir(abs)
	}

	return &pb.ListDirResponse{Path: abs, Parent: parent, Entries: entries}, nil
}

// GitClone ---------------------------------------------------------------

func (h *nodeHandler) GitClone(ctx context.Context, req *pb.GitCloneRequest) (*pb.GitCloneResponse, error) {
	dest := req.Dest
	if dest == "" {
		home, _ := os.UserHomeDir()
		dest = filepath.Join(home, "meru-workspaces", repoNameFromURL(req.Url))
	}

	opts := &gogit.CloneOptions{
		URL: req.Url,
	}

	switch {
	case req.Username != "" || req.Password != "":
		opts.Auth = &githttp.BasicAuth{Username: req.Username, Password: req.Password}
	case isSSHURL(req.Url):
		if auth, err := gitssh.NewSSHAgentAuth("git"); err == nil {
			opts.Auth = auth
		}
	}

	if _, err := gogit.PlainCloneContext(ctx, dest, opts); err != nil {
		return nil, status.Errorf(codes.Internal, "git clone: %v", err)
	}
	return &pb.GitCloneResponse{Path: dest}, nil
}

func isSSHURL(rawURL string) bool {
	return strings.HasPrefix(rawURL, "git@") || strings.HasPrefix(rawURL, "ssh://")
}

func repoNameFromURL(rawURL string) string {
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

// --- auth interceptors --------------------------------------------------

const authHeader = "authorization"

func authUnaryInterceptor(token string) grpc.UnaryServerInterceptor {
	return func(ctx context.Context, req any, _ *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (any, error) {
		if err := checkToken(ctx, token); err != nil {
			return nil, err
		}
		return handler(ctx, req)
	}
}

func authStreamInterceptor(token string) grpc.StreamServerInterceptor {
	return func(srv any, ss grpc.ServerStream, _ *grpc.StreamServerInfo, handler grpc.StreamHandler) error {
		if err := checkToken(ss.Context(), token); err != nil {
			return err
		}
		return handler(srv, ss)
	}
}

func checkToken(ctx context.Context, expected string) error {
	md, ok := metadata.FromIncomingContext(ctx)
	if !ok {
		return status.Error(codes.Unauthenticated, "missing metadata")
	}
	vals := md.Get(authHeader)
	if len(vals) == 0 {
		return status.Error(codes.Unauthenticated, "missing authorization header")
	}
	got := strings.TrimPrefix(vals[0], "Bearer ")
	if got != expected {
		return status.Error(codes.PermissionDenied, "invalid token")
	}
	return nil
}

// --- proto helpers ------------------------------------------------------

func eventToProto(ev agent.Event) *pb.EventMessage {
	return &pb.EventMessage{
		Type:            string(ev.Type),
		Text:            ev.Text,
		ToolName:        ev.ToolName,
		ToolInput:       ev.ToolInput,
		Error:           ev.Error,
		TimestampUnixMs: ev.Timestamp.UnixMilli(),
	}
}

// Ensure io is used (Logs returns io.NopCloser; imported in grpc_client.go)
var _ = io.EOF
var _ = fmt.Sprintf
var _ = time.Now
