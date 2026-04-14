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
	// Register all supported agents at startup.
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
	Use:   "meru",
	Short: "Conductor — local coding agent orchestrator",
	Long: `Conductor spins up, manages, and coordinates AI coding agents
(Claude Code, OpenCode, Goose, Aider, …) on your local machine.

Start the daemon first:
  meru serve

Then in another terminal:
  meru spawn claude --workspace ~/projects/myapp --name refactor-bot
  meru list
  meru send <session-id> "refactor the auth module"
  meru logs <session-id>
  meru stop <session-id>`,
}
