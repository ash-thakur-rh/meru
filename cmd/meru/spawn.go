package main

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

var (
	spawnWorkspace string
	spawnName      string
	spawnModel     string
	spawnWorktree  bool
	spawnNode      string
)

var spawnCmd = &cobra.Command{
	Use:   "spawn <agent>",
	Short: "Spawn a new agent session",
	Example: `  meru spawn claude
  meru spawn claude --workspace ~/projects/myapp --name refactor-bot
  meru spawn claude --model claude-opus-4-6`,
	Args: cobra.ExactArgs(1),
	RunE: runSpawn,
}

func init() {
	spawnCmd.Flags().StringVarP(&spawnWorkspace, "workspace", "w", mustGetwd(), "Working directory for the agent")
	spawnCmd.Flags().StringVarP(&spawnName, "name", "n", "", "Human-readable session name (default: auto-generated)")
	spawnCmd.Flags().StringVarP(&spawnModel, "model", "m", "", "Model to use (agent-specific)")
	spawnCmd.Flags().BoolVar(&spawnWorktree, "worktree", false, "Create an isolated git worktree for this session (local node only)")
	spawnCmd.Flags().StringVar(&spawnNode, "node", "", "Target node name (default: local)")
	rootCmd.AddCommand(spawnCmd)
}

func runSpawn(cmd *cobra.Command, args []string) error {
	agentName := args[0]

	var result map[string]any
	err := doJSON("POST", "/sessions", map[string]any{
		"agent":     agentName,
		"name":      spawnName,
		"workspace": spawnWorkspace,
		"model":     spawnModel,
		"worktree":  spawnWorktree,
		"node":      spawnNode,
	}, &result)
	if err != nil {
		return err
	}

	out, _ := json.MarshalIndent(result, "", "  ")
	fmt.Println(string(out))
	return nil
}

func mustGetwd() string {
	wd, err := os.Getwd()
	if err != nil {
		return "."
	}
	return wd
}
