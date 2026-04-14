package node_test

import (
	"context"
	"testing"

	"github.com/ash-thakur-rh/meru/internal/node"
)

func TestRegister_LocalNode(t *testing.T) {
	n := node.NewLocalNode()
	node.Register(n)
	// local node is always present; no cleanup needed (idempotent)

	got, err := node.Get(node.LocalNodeName)
	if err != nil {
		t.Fatalf("Get local: %v", err)
	}
	if got.Name() != node.LocalNodeName {
		t.Errorf("Name = %q, want %q", got.Name(), node.LocalNodeName)
	}
}

func TestGet_Empty_DefaultsToLocal(t *testing.T) {
	node.Register(node.NewLocalNode())

	got, err := node.Get("") // empty string → "local"
	if err != nil {
		t.Fatalf("Get empty: %v", err)
	}
	if got.Name() != node.LocalNodeName {
		t.Errorf("Name = %q, want %q", got.Name(), node.LocalNodeName)
	}
}

func TestGet_Unknown(t *testing.T) {
	_, err := node.Get("no-such-node-xyz")
	if err == nil {
		t.Error("expected error for unknown node, got nil")
	}
}

func TestUnregister_RemotNode(t *testing.T) {
	// Can't create a real GRPCNode without a server, so verify Unregister
	// returns an error for an unknown node name.
	node.Register(node.NewLocalNode())
	err := node.Unregister("no-such-node-abc-xyz")
	if err == nil {
		t.Error("Unregister unknown should return error")
	}
}

func TestLocalNode_Ping(t *testing.T) {
	n := node.NewLocalNode()
	info, err := n.Ping(context.Background())
	if err != nil {
		t.Fatalf("Ping: %v", err)
	}
	if info == nil {
		t.Fatal("Ping returned nil info")
	}
	if info.Name != node.LocalNodeName {
		t.Errorf("Name = %q, want %q", info.Name, node.LocalNodeName)
	}
}

func TestList_IncludesRegistered(t *testing.T) {
	node.Register(node.NewLocalNode())
	names := node.List()
	found := false
	for _, n := range names {
		if n == node.LocalNodeName {
			found = true
		}
	}
	if !found {
		t.Errorf("List() = %v, missing %q", names, node.LocalNodeName)
	}
}
