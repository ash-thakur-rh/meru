package main

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/gorilla/websocket"
	"github.com/spf13/cobra"
	"golang.org/x/term"
)

var attachCmd = &cobra.Command{
	Use:   "attach <session-id>",
	Short: "Attach to a session's interactive terminal",
	Long: `Attach to a running session's PTY — identical to running the agent locally.

All keystrokes are forwarded to the agent's stdin. Output streams in real time.
Press Ctrl+C or close the connection to detach (the session keeps running).

For stopped sessions the stored output is replayed then the connection closes.`,
	Args: cobra.ExactArgs(1),
	RunE: runAttach,
}

func init() {
	rootCmd.AddCommand(attachCmd)
}

func runAttach(cmd *cobra.Command, args []string) error {
	sessionID := args[0]

	// Convert HTTP URL to WebSocket URL.
	wsURL := strings.Replace(daemonURL, "http://", "ws://", 1)
	wsURL = strings.Replace(wsURL, "https://", "wss://", 1)
	wsURL += "/sessions/" + sessionID + "/terminal"

	conn, resp, err := websocket.DefaultDialer.Dial(wsURL, nil)
	if err != nil {
		return fmt.Errorf("cannot connect to session %s: %w", sessionID, err)
	}
	if resp != nil {
		resp.Body.Close()
	}
	defer conn.Close()

	// Put terminal into raw mode so every keystroke goes straight to the agent.
	fd := int(os.Stdin.Fd())
	if !term.IsTerminal(fd) {
		return fmt.Errorf("stdin is not a terminal — attach requires an interactive terminal")
	}
	oldState, err := term.MakeRaw(fd)
	if err != nil {
		return fmt.Errorf("failed to set raw terminal mode: %w", err)
	}
	defer term.Restore(fd, oldState) //nolint:errcheck

	// Send initial terminal size.
	if cols, rows, err := term.GetSize(fd); err == nil {
		sendResize(conn, cols, rows)
	}

	// Forward terminal resize events to the PTY (no-op on Windows).
	stopResize := watchResize(conn, fd)
	defer stopResize()

	done := make(chan struct{})

	// WebSocket → stdout goroutine.
	go func() {
		defer close(done)
		for {
			mt, msg, err := conn.ReadMessage()
			if err != nil {
				return
			}
			if mt == websocket.BinaryMessage {
				os.Stdout.Write(msg) //nolint:errcheck
			}
		}
	}()

	// stdin → WebSocket goroutine.
	stdinErr := make(chan error, 1)
	go func() {
		buf := make([]byte, 256)
		for {
			n, err := os.Stdin.Read(buf)
			if n > 0 {
				if werr := conn.WriteMessage(websocket.BinaryMessage, buf[:n]); werr != nil {
					stdinErr <- werr
					return
				}
			}
			if err != nil {
				stdinErr <- err
				return
			}
		}
	}()

	select {
	case <-done:
		// Server closed the connection (session stopped or stopped-replay finished).
	case <-stdinErr:
		// stdin closed (user pressed Ctrl+D etc.).
	}

	return nil
}

func sendResize(conn *websocket.Conn, cols, rows int) {
	data, _ := json.Marshal(map[string]any{
		"type": "resize",
		"cols": cols,
		"rows": rows,
	})
	conn.WriteMessage(websocket.TextMessage, data) //nolint:errcheck
}
