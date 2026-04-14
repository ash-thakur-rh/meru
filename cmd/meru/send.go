package main

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/spf13/cobra"
)

var sendCmd = &cobra.Command{
	Use:   "send <session-id> <prompt>",
	Short: "Send a prompt to a session and stream the response",
	Example: `  meru send abc123 "refactor the auth module"
  meru send refactor-bot "add unit tests for user.go"`,
	Args: cobra.ExactArgs(2),
	RunE: runSend,
}

func init() {
	rootCmd.AddCommand(sendCmd)
}

// event is the JSON shape we receive from /sessions/:id/send (nd-JSON stream)
type event struct {
	Type      string `json:"type"`
	Text      string `json:"text"`
	ToolName  string `json:"tool_name"`
	ToolInput string `json:"tool_input"`
	Error     string `json:"error"`
}

func runSend(cmd *cobra.Command, args []string) error {
	sessionID, prompt := args[0], args[1]

	body, _ := json.Marshal(map[string]string{"prompt": prompt})
	req, err := http.NewRequest("POST", apiURL("/sessions/"+sessionID+"/send"), bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return fmt.Errorf("cannot reach meru daemon — is it running? (meru serve)")
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		data, _ := io.ReadAll(resp.Body)
		var e struct{ Error string }
		json.Unmarshal(data, &e) //nolint:errcheck
		return fmt.Errorf("error %d: %s", resp.StatusCode, e.Error)
	}

	scanner := bufio.NewScanner(resp.Body)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}
		var ev event
		if err := json.Unmarshal([]byte(line), &ev); err != nil {
			continue
		}
		switch ev.Type {
		case "text":
			fmt.Print(ev.Text)
		case "tool_use":
			fmt.Printf("\n[tool: %s] %s\n", ev.ToolName, ev.ToolInput)
		case "done":
			fmt.Println() // newline after streamed text
		case "error":
			return fmt.Errorf("agent error: %s", ev.Error)
		}
	}
	return scanner.Err()
}
