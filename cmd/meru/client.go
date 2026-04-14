// client.go provides a thin HTTP client for the CLI commands to talk to the daemon.
package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
)

var daemonURL string

func init() {
	rootCmd.PersistentFlags().StringVar(&daemonURL, "server", "http://localhost:8080", "Conductor daemon URL")
}

func apiURL(path string) string {
	return daemonURL + path
}

func doJSON(method, path string, body any, out any) error {
	var r io.Reader
	if body != nil {
		b, _ := json.Marshal(body)
		r = bytes.NewReader(b)
	}

	req, err := http.NewRequest(method, apiURL(path), r)
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return fmt.Errorf("cannot reach meru daemon at %s — is it running? (meru serve)", daemonURL)
	}
	defer resp.Body.Close()

	data, _ := io.ReadAll(resp.Body)

	if resp.StatusCode >= 400 {
		var e struct{ Error string }
		json.Unmarshal(data, &e) //nolint:errcheck
		if e.Error != "" {
			return fmt.Errorf("server error %d: %s", resp.StatusCode, e.Error)
		}
		return fmt.Errorf("server error %d", resp.StatusCode)
	}

	if out != nil {
		return json.Unmarshal(data, out)
	}
	return nil
}
