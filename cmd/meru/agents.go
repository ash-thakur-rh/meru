package main

import (
	"fmt"
	"strings"

	"github.com/spf13/cobra"

	"github.com/ash-thakur-rh/meru/internal/agent"
)

var agentsCmd = &cobra.Command{
	Use:   "agents",
	Short: "List registered agent types",
	Run: func(cmd *cobra.Command, args []string) {
		names := agent.List()
		fmt.Printf("Registered agents: %s\n", strings.Join(names, ", "))
	},
}

func init() {
	rootCmd.AddCommand(agentsCmd)
}
