package main

import (
	"fmt"

	"github.com/spf13/cobra"
)

var stopCmd = &cobra.Command{
	Use:   "stop <session-id>",
	Short: "Stop a live session (keeps record) or delete a stopped session",
	Long: `Stops a live session and keeps its record in history for re-inspection.

If the session is already stopped, the record is permanently deleted instead.
To explicitly delete a stopped session use: meru delete <session-id>`,
	Args: cobra.ExactArgs(1),
	RunE: runStop,
}

func init() {
	rootCmd.AddCommand(stopCmd)
}

func runStop(cmd *cobra.Command, args []string) error {
	sessionID := args[0]

	// Peek at current status to give a meaningful message after the call.
	var info map[string]any
	isAlreadyStopped := false
	if err := doJSON("GET", "/sessions/"+sessionID, nil, &info); err == nil {
		isAlreadyStopped = str(info["status"]) == "stopped"
	}

	if err := doJSON("DELETE", "/sessions/"+sessionID, nil, nil); err != nil {
		return err
	}

	if isAlreadyStopped {
		fmt.Printf("Session %s deleted.\n", sessionID)
	} else {
		fmt.Printf("Session %s stopped.\n", sessionID)
	}
	return nil
}
