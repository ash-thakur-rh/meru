package main

import (
	"fmt"
	"os"
	"text/tabwriter"

	"github.com/spf13/cobra"
)

var listCmd = &cobra.Command{
	Use:   "list",
	Short: "List all sessions",
	RunE:  runList,
}

func init() {
	rootCmd.AddCommand(listCmd)
}

func runList(cmd *cobra.Command, args []string) error {
	var sessions []map[string]any
	if err := doJSON("GET", "/sessions", nil, &sessions); err != nil {
		return err
	}

	if len(sessions) == 0 {
		fmt.Println("No sessions. Use: meru spawn <agent>")
		return nil
	}

	w := tabwriter.NewWriter(os.Stdout, 0, 0, 3, ' ', 0)
	fmt.Fprintln(w, "ID\tNAME\tAGENT\tSTATUS\tNODE\tWORKSPACE")
	for _, s := range sessions {
		status := str(s["status"])
		// Add a marker for states that need attention.
		switch status {
		case "waiting":
			status = "waiting [!]" // agent is asking for input
		case "stopped":
			status = "stopped [-]"
		}
		fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%s\t%s\n",
			str(s["id"]),
			str(s["name"]),
			str(s["agent"]),
			status,
			str(s["node_name"]),
			str(s["workspace"]),
		)
	}
	return w.Flush()
}

func str(v any) string {
	if v == nil {
		return ""
	}
	return fmt.Sprintf("%v", v)
}
