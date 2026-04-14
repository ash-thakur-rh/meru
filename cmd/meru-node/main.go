// meru-node is the lightweight remote agent daemon.
//
// Install this binary on any machine where you want to run coding agents.
// The local Conductor control plane connects to it over gRPC.
//
// Usage:
//
//	meru-node serve --addr :9090 --token <shared-secret>
//	meru-node serve --addr :9090 --token <secret> --tls-cert cert.pem --tls-key key.pem
package main

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"github.com/ash-thakur-rh/meru/internal/agent"
	aideradapter "github.com/ash-thakur-rh/meru/internal/agent/adapters/aider"
	claudeadapter "github.com/ash-thakur-rh/meru/internal/agent/adapters/claude"
	gooseadapter "github.com/ash-thakur-rh/meru/internal/agent/adapters/goose"
	opencodeadapter "github.com/ash-thakur-rh/meru/internal/agent/adapters/opencode"
)

func init() {
	agent.Register(claudeadapter.New())
	agent.Register(aideradapter.New())
	agent.Register(opencodeadapter.New())
	agent.Register(gooseadapter.New())
}

func main() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

var rootCmd = &cobra.Command{
	Use:   "meru-node",
	Short: "Conductor remote agent node",
	Long: `meru-node runs on a remote machine and exposes a gRPC service
that the local Conductor daemon can use to spawn and manage coding agents.

Quick start:
  # On the remote machine:
  meru-node serve --addr :9090 --token mysecret

  # On your local machine:
  meru nodes add my-server --addr remote-host:9090 --token mysecret
  meru spawn claude --node my-server --workspace /home/user/project`,
}
