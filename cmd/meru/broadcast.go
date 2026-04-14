package main

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/spf13/cobra"
)

var broadcastSessions []string

var broadcastCmd = &cobra.Command{
	Use:   "broadcast <prompt>",
	Short: "Send a prompt to all active sessions in parallel",
	Example: `  meru broadcast "run the test suite and report failures"
  meru broadcast "summarize recent changes" --sessions id1,id2`,
	Args: cobra.ExactArgs(1),
	RunE: runBroadcast,
}

func init() {
	broadcastCmd.Flags().StringSliceVar(&broadcastSessions, "sessions", nil,
		"Comma-separated session IDs to target (default: all active)")
	rootCmd.AddCommand(broadcastCmd)
}

type broadcastResult struct {
	SessionID   string           `json:"SessionID"`
	SessionName string           `json:"SessionName"`
	Events      []map[string]any `json:"Events"`
	Err         *string          `json:"Err"`
}

func runBroadcast(cmd *cobra.Command, args []string) error {
	prompt := args[0]

	var results []broadcastResult
	if err := doJSON("POST", "/broadcast", map[string]any{
		"prompt":   prompt,
		"sessions": broadcastSessions,
	}, &results); err != nil {
		return err
	}

	if len(results) == 0 {
		fmt.Println("No active sessions to broadcast to. Use: meru spawn <agent>")
		return nil
	}

	for _, r := range results {
		sep := strings.Repeat("─", 60)
		fmt.Printf("\n%s\n[%s] %s\n%s\n", sep, r.SessionID[:8], r.SessionName, sep)
		if r.Err != nil {
			fmt.Printf("ERROR: %s\n", *r.Err)
			continue
		}
		for _, ev := range r.Events {
			switch ev["type"] {
			case "text":
				fmt.Print(ev["text"])
			case "tool_use":
				fmt.Printf("\n[tool: %v] %v\n", ev["tool_name"], ev["tool_input"])
			case "error":
				fmt.Printf("ERROR: %v\n", ev["error"])
			}
		}
		fmt.Println()
	}

	// Summary
	fmt.Printf("\nBroadcast complete: %d session(s)\n", len(results))
	errs := 0
	for _, r := range results {
		if r.Err != nil {
			errs++
		}
	}
	if errs > 0 {
		fmt.Printf("  %d error(s)\n", errs)
	}

	_ = json.Marshal // keep import
	return nil
}
