package main

import (
	"context"
	"net"
	"os"
	"path/filepath"
	"testing"
	"time"

	gogit "github.com/go-git/go-git/v6"
	"github.com/go-git/go-git/v6/plumbing/object"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/test/bufconn"

	"github.com/ash-thakur-rh/meru/internal/agent"
	pb "github.com/ash-thakur-rh/meru/internal/proto"
	"github.com/ash-thakur-rh/meru/internal/testutil"
)

const testToken = "test-secret"
const bufSize = 1024 * 1024

// startServer spins up a gRPC server over an in-memory bufconn listener.
// It registers the mock agent and returns a connected client.
func startServer(t *testing.T, events ...agent.Event) pb.MeruNodeClient {
	t.Helper()

	const mockName = "mock-grpc"
	agent.Register(testutil.NewMockAgent(mockName, events...))
	t.Cleanup(func() { agent.Unregister(mockName) })

	lis := bufconn.Listen(bufSize)
	t.Cleanup(func() { lis.Close() })

	h := newNodeHandler()
	srv := grpc.NewServer(
		grpc.UnaryInterceptor(authUnaryInterceptor(testToken)),
		grpc.StreamInterceptor(authStreamInterceptor(testToken)),
	)
	pb.RegisterMeruNodeServer(srv, h)

	go func() { _ = srv.Serve(lis) }()
	t.Cleanup(srv.GracefulStop)

	dialer := func(ctx context.Context, _ string) (net.Conn, error) {
		return lis.DialContext(ctx)
	}
	conn, err := grpc.NewClient("passthrough://bufnet",
		grpc.WithContextDialer(dialer),
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	)
	if err != nil {
		t.Fatalf("dial bufconn: %v", err)
	}
	t.Cleanup(func() { conn.Close() })

	return pb.NewMeruNodeClient(conn)
}

// authCtx attaches the Bearer token to the outgoing context.
func authCtx() context.Context {
	return metadata.AppendToOutgoingContext(
		context.Background(), "authorization", "Bearer "+testToken,
	)
}

// --- Auth ---

func TestAuth_MissingToken(t *testing.T) {
	client := startServer(t)
	// No auth metadata → should be rejected
	_, err := client.Ping(context.Background(), &pb.PingRequest{})
	if err == nil {
		t.Error("expected unauthenticated error, got nil")
	}
}

func TestAuth_WrongToken(t *testing.T) {
	client := startServer(t)
	ctx := metadata.AppendToOutgoingContext(context.Background(),
		"authorization", "Bearer wrong-token")
	_, err := client.Ping(ctx, &pb.PingRequest{})
	if err == nil {
		t.Error("expected permission denied error, got nil")
	}
}

// --- Ping ---

func TestPing(t *testing.T) {
	client := startServer(t)
	resp, err := client.Ping(authCtx(), &pb.PingRequest{})
	if err != nil {
		t.Fatalf("Ping: %v", err)
	}
	if resp.Version == "" {
		t.Error("expected non-empty version")
	}
	// mock-grpc agent is registered; it should appear in the list
	found := false
	for _, a := range resp.Agents {
		if a == "mock-grpc" {
			found = true
		}
	}
	if !found {
		t.Errorf("agents = %v, expected to include mock-grpc", resp.Agents)
	}
}

// --- Spawn ---

func TestSpawn_OK(t *testing.T) {
	client := startServer(t)
	resp, err := client.Spawn(authCtx(), &pb.SpawnRequest{
		Agent:     "mock-grpc",
		SessionId: "sess-1",
		Name:      "grpc-bot",
		Workspace: t.TempDir(),
	})
	if err != nil {
		t.Fatalf("Spawn: %v", err)
	}
	if resp.SessionId != "sess-1" {
		t.Errorf("SessionId = %q, want sess-1", resp.SessionId)
	}
	if resp.Name != "grpc-bot" {
		t.Errorf("Name = %q, want grpc-bot", resp.Name)
	}
}

func TestSpawn_UnknownAgent(t *testing.T) {
	client := startServer(t)
	_, err := client.Spawn(authCtx(), &pb.SpawnRequest{
		Agent:     "no-such-agent",
		SessionId: "x",
		Workspace: t.TempDir(),
	})
	if err == nil {
		t.Error("expected error for unknown agent, got nil")
	}
}

// --- GetSession ---

func TestGetSession_OK(t *testing.T) {
	client := startServer(t)
	client.Spawn(authCtx(), &pb.SpawnRequest{ //nolint:errcheck
		Agent: "mock-grpc", SessionId: "sess-g", Name: "getme", Workspace: t.TempDir(),
	})

	info, err := client.GetSession(authCtx(), &pb.GetSessionRequest{SessionId: "sess-g"})
	if err != nil {
		t.Fatalf("GetSession: %v", err)
	}
	if info.Name != "getme" {
		t.Errorf("Name = %q, want getme", info.Name)
	}
}

func TestGetSession_Unknown(t *testing.T) {
	client := startServer(t)
	_, err := client.GetSession(authCtx(), &pb.GetSessionRequest{SessionId: "no-such"})
	if err == nil {
		t.Error("expected error for unknown session, got nil")
	}
}

// --- ListSessions ---

func TestListSessions(t *testing.T) {
	client := startServer(t)
	client.Spawn(authCtx(), &pb.SpawnRequest{ //nolint:errcheck
		Agent: "mock-grpc", SessionId: "ls-1", Workspace: t.TempDir(),
	})
	client.Spawn(authCtx(), &pb.SpawnRequest{ //nolint:errcheck
		Agent: "mock-grpc", SessionId: "ls-2", Workspace: t.TempDir(),
	})

	resp, err := client.ListSessions(authCtx(), &pb.Empty{})
	if err != nil {
		t.Fatalf("ListSessions: %v", err)
	}
	if len(resp.Sessions) != 2 {
		t.Errorf("sessions = %d, want 2", len(resp.Sessions))
	}
}

// --- Send (streaming) ---

func TestSend_ReceivesEvents(t *testing.T) {
	evs := testutil.TextEvents("hello", "world")
	client := startServer(t, evs...)

	client.Spawn(authCtx(), &pb.SpawnRequest{ //nolint:errcheck
		Agent: "mock-grpc", SessionId: "send-1", Workspace: t.TempDir(),
	})

	stream, err := client.Send(authCtx(), &pb.SendRequest{
		SessionId: "send-1",
		Prompt:    "go",
	})
	if err != nil {
		t.Fatalf("Send: %v", err)
	}

	var got []*pb.EventMessage
	for {
		msg, err := stream.Recv()
		if err != nil {
			break
		}
		got = append(got, msg)
	}

	if len(got) < 3 {
		t.Fatalf("expected ≥3 events (hello, world, done), got %d", len(got))
	}
	if got[0].Text != "hello" {
		t.Errorf("event[0].Text = %q, want hello", got[0].Text)
	}
	if got[len(got)-1].Type != "done" {
		t.Errorf("last event type = %q, want done", got[len(got)-1].Type)
	}
}

func TestSend_UnknownSession(t *testing.T) {
	client := startServer(t)
	stream, err := client.Send(authCtx(), &pb.SendRequest{
		SessionId: "no-such",
		Prompt:    "hi",
	})
	if err != nil {
		t.Fatalf("Send: %v", err)
	}
	_, err = stream.Recv()
	if err == nil {
		t.Error("expected error for unknown session")
	}
}

// --- Stop ---

func TestStop_OK(t *testing.T) {
	client := startServer(t)
	client.Spawn(authCtx(), &pb.SpawnRequest{ //nolint:errcheck
		Agent: "mock-grpc", SessionId: "stop-1", Workspace: t.TempDir(),
	})

	_, err := client.Stop(authCtx(), &pb.StopRequest{SessionId: "stop-1"})
	if err != nil {
		t.Fatalf("Stop: %v", err)
	}

	// Should no longer be visible
	_, err = client.GetSession(authCtx(), &pb.GetSessionRequest{SessionId: "stop-1"})
	if err == nil {
		t.Error("expected error after stop, got nil")
	}
}

func TestStop_Unknown(t *testing.T) {
	client := startServer(t)
	_, err := client.Stop(authCtx(), &pb.StopRequest{SessionId: "no-such"})
	if err == nil {
		t.Error("expected error for unknown session")
	}
}

// --- GetLogs ---

func TestGetLogs_AfterSend(t *testing.T) {
	evs := testutil.TextEvents("logged!")
	client := startServer(t, evs...)

	client.Spawn(authCtx(), &pb.SpawnRequest{ //nolint:errcheck
		Agent: "mock-grpc", SessionId: "log-1", Workspace: t.TempDir(),
	})

	// Send a prompt to populate the event log
	stream, _ := client.Send(authCtx(), &pb.SendRequest{SessionId: "log-1", Prompt: "x"})
	for {
		if _, err := stream.Recv(); err != nil {
			break
		}
	}

	resp, err := client.GetLogs(authCtx(), &pb.GetLogsRequest{SessionId: "log-1"})
	if err != nil {
		t.Fatalf("GetLogs: %v", err)
	}
	if len(resp.Events) == 0 {
		t.Error("expected events in log, got none")
	}
	found := false
	for _, ev := range resp.Events {
		if ev.Text == "logged!" {
			found = true
		}
	}
	if !found {
		t.Errorf("events = %+v, expected text 'logged!'", resp.Events)
	}
}

func TestGetLogs_Unknown(t *testing.T) {
	client := startServer(t)
	_, err := client.GetLogs(authCtx(), &pb.GetLogsRequest{SessionId: "no-such"})
	if err == nil {
		t.Error("expected error for unknown session")
	}
}

// --- ListDir ---

func TestListDir_KnownPath(t *testing.T) {
	client := startServer(t)
	dir := t.TempDir()
	// Create a file and a subdirectory to verify listing
	if err := os.WriteFile(filepath.Join(dir, "file.txt"), []byte("x"), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}
	if err := os.MkdirAll(filepath.Join(dir, "sub"), 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}

	resp, err := client.ListDir(authCtx(), &pb.ListDirRequest{Path: dir})
	if err != nil {
		t.Fatalf("ListDir: %v", err)
	}
	if resp.Path != dir {
		t.Errorf("path = %q, want %q", resp.Path, dir)
	}
	if len(resp.Entries) != 2 {
		t.Errorf("entries = %d, want 2", len(resp.Entries))
	}
	// Directories are sorted before files
	if !resp.Entries[0].IsDir {
		t.Error("expected first entry to be a directory")
	}
}

func TestListDir_EmptyPathIsHome(t *testing.T) {
	client := startServer(t)
	resp, err := client.ListDir(authCtx(), &pb.ListDirRequest{Path: ""})
	if err != nil {
		t.Fatalf("ListDir: %v", err)
	}
	home, _ := os.UserHomeDir()
	if resp.Path != home {
		t.Errorf("path = %q, want home dir %q", resp.Path, home)
	}
}

func TestListDir_SkipsHiddenEntries(t *testing.T) {
	client := startServer(t)
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, ".hidden"), []byte("x"), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "visible.txt"), []byte("x"), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}

	resp, err := client.ListDir(authCtx(), &pb.ListDirRequest{Path: dir})
	if err != nil {
		t.Fatalf("ListDir: %v", err)
	}
	if len(resp.Entries) != 1 {
		t.Errorf("entries = %d, want 1 (hidden must be excluded)", len(resp.Entries))
	}
	if resp.Entries[0].Name == ".hidden" {
		t.Error("hidden file should not appear in listing")
	}
}

func TestListDir_InvalidPath(t *testing.T) {
	client := startServer(t)
	_, err := client.ListDir(authCtx(), &pb.ListDirRequest{Path: "/no/such/path/conductor-xyz"})
	if err == nil {
		t.Error("expected error for non-existent path")
	}
}

// --- GitClone ---

// initHandlerTestRepo creates a local git repository with one commit,
// for use as a clone source in handler tests.
func initHandlerTestRepo(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	repo, err := gogit.PlainInit(dir, false)
	if err != nil {
		t.Fatalf("git init: %v", err)
	}
	wt, _ := repo.Worktree()
	if err := os.WriteFile(filepath.Join(dir, "src.txt"), []byte("hello"), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}
	wt.Add("src.txt")                       //nolint:errcheck
	wt.Commit("init", &gogit.CommitOptions{ //nolint:errcheck
		Author: &object.Signature{Name: "t", Email: "t@t.com", When: time.Now()},
	})
	return dir
}

func TestGitClone_Success(t *testing.T) {
	client := startServer(t)
	src := initHandlerTestRepo(t)
	dest := filepath.Join(t.TempDir(), "clone")

	resp, err := client.GitClone(authCtx(), &pb.GitCloneRequest{
		Url:  src,
		Dest: dest,
	})
	if err != nil {
		t.Fatalf("GitClone: %v", err)
	}
	if resp.Path != dest {
		t.Errorf("path = %q, want %q", resp.Path, dest)
	}
	if _, err := os.Stat(filepath.Join(dest, "src.txt")); err != nil {
		t.Errorf("src.txt not found in clone: %v", err)
	}
}

func TestGitClone_InvalidSource(t *testing.T) {
	client := startServer(t)
	_, err := client.GitClone(authCtx(), &pb.GitCloneRequest{
		Url:  "/no/such/repo/conductor-xyz",
		Dest: filepath.Join(t.TempDir(), "clone"),
	})
	if err == nil {
		t.Error("expected error for non-existent source repository")
	}
}
