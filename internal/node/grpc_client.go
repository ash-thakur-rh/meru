package node

import (
	"context"
	"fmt"
	"io"
	"sync"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/metadata"

	"github.com/ash-thakur-rh/meru/internal/agent"
	pb "github.com/ash-thakur-rh/meru/internal/proto"
)

// GRPCNode implements Node by talking to a remote meru-node daemon over gRPC.
type GRPCNode struct {
	info   Info
	conn   *grpc.ClientConn
	client pb.MeruNodeClient
}

// NewGRPCNode dials the remote meru-node and returns a Node ready to use.
func NewGRPCNode(ctx context.Context, info Info) (*GRPCNode, error) {
	var creds credentials.TransportCredentials
	if info.TLS {
		creds = credentials.NewClientTLSFromCert(nil, "")
	} else {
		creds = insecure.NewCredentials()
	}

	conn, err := grpc.NewClient(info.Addr,
		grpc.WithTransportCredentials(creds),
		grpc.WithUnaryInterceptor(tokenInterceptor(info.Token)),
		grpc.WithStreamInterceptor(tokenStreamInterceptor(info.Token)),
	)
	if err != nil {
		return nil, fmt.Errorf("dial %s: %w", info.Addr, err)
	}

	return &GRPCNode{
		info:   info,
		conn:   conn,
		client: pb.NewMeruNodeClient(conn),
	}, nil
}

func (g *GRPCNode) Name() string { return g.info.Name }

func (g *GRPCNode) Ping(ctx context.Context) (*Info, error) {
	resp, err := g.client.Ping(ctx, &pb.PingRequest{})
	if err != nil {
		return nil, fmt.Errorf("ping %s: %w", g.info.Name, err)
	}
	return &Info{
		Name:    g.info.Name,
		Addr:    g.info.Addr,
		Token:   g.info.Token,
		TLS:     g.info.TLS,
		Version: resp.Version,
		Agents:  resp.Agents,
	}, nil
}

// Spawn creates a session on the remote node.
func (g *GRPCNode) Spawn(ctx context.Context, sessionID string, agentName string, cfg agent.SpawnConfig) (agent.Session, error) {
	req := &pb.SpawnRequest{
		SessionId:  sessionID,
		Agent:      agentName,
		Name:       cfg.Name,
		Workspace:  cfg.Workspace,
		Model:      cfg.Model,
		Env:        cfg.Env,
		Worktree:   cfg.Worktree,
		BranchName: cfg.BranchName,
	}

	resp, err := g.client.Spawn(ctx, req)
	if err != nil {
		return nil, fmt.Errorf("remote spawn on %s: %w", g.info.Name, err)
	}

	return &remoteSession{
		id:        resp.SessionId,
		name:      resp.Name,
		agentName: agentName,
		workspace: resp.Workspace,
		nodeName:  g.info.Name,
		client:    g.client,
		status:    agent.StatusIdle,
	}, nil
}

func (g *GRPCNode) GitClone(ctx context.Context, rawURL, dest, username, password string) (string, error) {
	resp, err := g.client.GitClone(ctx, &pb.GitCloneRequest{
		Url:      rawURL,
		Dest:     dest,
		Username: username,
		Password: password,
	})
	if err != nil {
		return "", fmt.Errorf("remote git clone on %s: %w", g.info.Name, err)
	}
	return resp.Path, nil
}

func (g *GRPCNode) ListDir(ctx context.Context, path string) (*DirListing, error) {
	resp, err := g.client.ListDir(ctx, &pb.ListDirRequest{Path: path})
	if err != nil {
		return nil, fmt.Errorf("remote listdir on %s: %w", g.info.Name, err)
	}
	entries := make([]DirEntry, len(resp.Entries))
	for i, e := range resp.Entries {
		entries[i] = DirEntry{Name: e.Name, Path: e.Path, IsDir: e.IsDir}
	}
	return &DirListing{Path: resp.Path, Parent: resp.Parent, Entries: entries}, nil
}

func (g *GRPCNode) Close() error {
	return g.conn.Close()
}

// --- remoteSession ---

// remoteSession implements agent.Session backed by gRPC calls to a remote node.
type remoteSession struct {
	id        string
	name      string
	agentName string
	workspace string
	nodeName  string
	client    pb.MeruNodeClient

	mu     sync.Mutex
	status agent.Status
}

func (s *remoteSession) ID() string        { return s.id }
func (s *remoteSession) Name() string      { return s.name }
func (s *remoteSession) AgentName() string { return s.agentName }
func (s *remoteSession) Workspace() string { return s.workspace }
func (s *remoteSession) Logs() io.Reader   { return io.NopCloser(nil) } // see GetLogs

func (s *remoteSession) Status() agent.Status {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.status
}

func (s *remoteSession) setStatus(st agent.Status) {
	s.mu.Lock()
	s.status = st
	s.mu.Unlock()
}

func (s *remoteSession) Stop() error {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	_, err := s.client.Stop(ctx, &pb.StopRequest{SessionId: s.id})
	if err == nil {
		s.setStatus(agent.StatusStopped)
	}
	return err
}

// Send opens a server-streaming gRPC call and translates events into the
// standard agent.Event channel used by the rest of Conductor.
func (s *remoteSession) Send(ctx context.Context, prompt string) (<-chan agent.Event, error) {
	s.mu.Lock()
	if s.status == agent.StatusStopped {
		s.mu.Unlock()
		return nil, fmt.Errorf("session %s is stopped", s.id)
	}
	s.status = agent.StatusBusy
	s.mu.Unlock()

	stream, err := s.client.Send(ctx, &pb.SendRequest{
		SessionId: s.id,
		Prompt:    prompt,
	})
	if err != nil {
		s.setStatus(agent.StatusIdle)
		return nil, fmt.Errorf("remote send: %w", err)
	}

	ch := make(chan agent.Event, 64)
	go func() {
		defer close(ch)
		defer s.setStatus(agent.StatusIdle)
		for {
			msg, err := stream.Recv()
			if err == io.EOF {
				return
			}
			if err != nil {
				if ctx.Err() == nil {
					ch <- agent.Event{
						Type:      agent.EventError,
						Error:     err.Error(),
						Timestamp: time.Now(),
					}
				}
				return
			}
			ch <- protoToEvent(msg)
		}
	}()

	return ch, nil
}

// --- auth interceptors ---

const authHeader = "authorization"

func tokenInterceptor(token string) grpc.UnaryClientInterceptor {
	return func(ctx context.Context, method string, req, reply any,
		cc *grpc.ClientConn, invoker grpc.UnaryInvoker, opts ...grpc.CallOption) error {
		return invoker(metadata.AppendToOutgoingContext(ctx, authHeader, "Bearer "+token),
			method, req, reply, cc, opts...)
	}
}

func tokenStreamInterceptor(token string) grpc.StreamClientInterceptor {
	return func(ctx context.Context, desc *grpc.StreamDesc, cc *grpc.ClientConn,
		method string, streamer grpc.Streamer, opts ...grpc.CallOption) (grpc.ClientStream, error) {
		return streamer(metadata.AppendToOutgoingContext(ctx, authHeader, "Bearer "+token),
			desc, cc, method, opts...)
	}
}

// --- proto translation ---

func protoToEvent(msg *pb.EventMessage) agent.Event {
	return agent.Event{
		Type:      agent.EventType(msg.Type),
		Text:      msg.Text,
		ToolName:  msg.ToolName,
		ToolInput: msg.ToolInput,
		Error:     msg.Error,
		Timestamp: time.UnixMilli(msg.TimestampUnixMs),
	}
}
