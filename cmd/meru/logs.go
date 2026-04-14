package main

import (
	"fmt"

	"github.com/spf13/cobra"
)

var logsCmd = &cobra.Command{
	Use:   "logs <session-id>",
	Short: "Show event history for a session",
	Args:  cobra.ExactArgs(1),
	RunE:  runLogs,
}

func init() {
	rootCmd.AddCommand(logsCmd)
}

func runLogs(cmd *cobra.Command, args []string) error {
	sessionID := args[0]

	var events []map[string]any
	if err := doJSON("GET", "/sessions/"+sessionID+"/logs", nil, &events); err != nil {
		return err
	}

	if len(events) == 0 {
		fmt.Println("No events recorded for this session.")
		return nil
	}

	for _, e := range events {
		ts := str(e["timestamp"])
		typ := str(e["type"])
		switch typ {
		case "text":
			fmt.Printf("[%s] %s", ts, str(e["text"]))
		case "tool_use":
			fmt.Printf("[%s] [tool: %s] %s\n", ts, str(e["tool_name"]), str(e["tool_input"]))
		case "done":
			fmt.Printf("[%s] --- done ---\n", ts)
		case "error":
			fmt.Printf("[%s] ERROR: %s\n", ts, str(e["error"]))
		default:
			fmt.Printf("[%s] %s\n", ts, typ)
		}
	}
	return nil
}
