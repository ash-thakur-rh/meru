package agent_test

import (
	"testing"

	"github.com/ash-thakur-rh/meru/internal/agent"
	"github.com/ash-thakur-rh/meru/internal/testutil"
)

func TestRegister_And_Get(t *testing.T) {
	a := testutil.NewMockAgent("test-agent-reg")
	agent.Register(a)
	t.Cleanup(func() { agent.Unregister("test-agent-reg") })

	got, err := agent.Get("test-agent-reg")
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if got.Name() != "test-agent-reg" {
		t.Errorf("Name = %q, want %q", got.Name(), "test-agent-reg")
	}
}

func TestGet_Unknown(t *testing.T) {
	_, err := agent.Get("no-such-agent-xyz")
	if err == nil {
		t.Error("expected error for unknown agent, got nil")
	}
}

func TestUnregister(t *testing.T) {
	agent.Register(testutil.NewMockAgent("temp-agent"))
	agent.Unregister("temp-agent")

	_, err := agent.Get("temp-agent")
	if err == nil {
		t.Error("expected error after unregister, got nil")
	}
}

func TestList_IncludesRegistered(t *testing.T) {
	agent.Register(testutil.NewMockAgent("list-agent-1"))
	agent.Register(testutil.NewMockAgent("list-agent-2"))
	t.Cleanup(func() {
		agent.Unregister("list-agent-1")
		agent.Unregister("list-agent-2")
	})

	names := agent.List()
	has := func(n string) bool {
		for _, name := range names {
			if name == n {
				return true
			}
		}
		return false
	}
	if !has("list-agent-1") || !has("list-agent-2") {
		t.Errorf("List() = %v, missing expected agents", names)
	}
}
