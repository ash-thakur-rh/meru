package main

import (
	"fmt"

	"github.com/spf13/cobra"
)

var deleteCmd = &cobra.Command{
	Use:   "delete <session-id>",
	Short: "Permanently delete a stopped session record",
	Long: `Permanently removes a stopped session and its event history from the database.

The session must already be stopped. To stop a live session first, use:
  meru stop <session-id>
  meru delete <session-id>`,
	Args: cobra.ExactArgs(1),
	RunE: runDelete,
}

func init() {
	rootCmd.AddCommand(deleteCmd)
}

func runDelete(cmd *cobra.Command, args []string) error {
	sessionID := args[0]
	if err := doJSON("DELETE", "/sessions/"+sessionID, nil, nil); err != nil {
		return err
	}
	fmt.Printf("Session %s deleted.\n", sessionID)
	return nil
}
