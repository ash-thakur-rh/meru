package main

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"

	"github.com/spf13/cobra"

	"github.com/ash-thakur-rh/meru/internal/api"
	"github.com/ash-thakur-rh/meru/internal/logwriter"
	"github.com/ash-thakur-rh/meru/internal/node"
	"github.com/ash-thakur-rh/meru/internal/session"
	"github.com/ash-thakur-rh/meru/internal/store"
	"github.com/ash-thakur-rh/meru/internal/ui"
)

var serveAddr string

var serveCmd = &cobra.Command{
	Use:   "serve",
	Short: "Start the Conductor daemon",
	RunE:  runServe,
}

func init() {
	serveCmd.Flags().StringVarP(&serveAddr, "addr", "a", ":8080", "Listen address")
	rootCmd.AddCommand(serveCmd)
}

func runServe(cmd *cobra.Command, args []string) error {
	dir := dataDir()
	dbPath := filepath.Join(dir, "meru.db")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("create data dir: %w", err)
	}

	// Set up structured logging to stderr + ~/.meru/meru.log
	logCloser, err := logwriter.Setup(dir)
	if err != nil {
		fmt.Fprintf(os.Stderr, "warn: could not open log file: %v (logging to stderr only)\n", err)
	} else {
		defer logCloser.Close()
		fmt.Printf("  Log: %s\n", logCloser.LogFile())
	}

	st, err := store.Open(dbPath)
	if err != nil {
		return fmt.Errorf("open store: %w", err)
	}
	defer st.Close()

	slog.Info("meru starting", "addr", serveAddr, "db", dbPath)

	// Always register the local node first.
	node.Register(node.NewLocalNode())

	// Re-register any persisted remote nodes from the store.
	if remoteNodes, err := st.ListNodes(); err == nil {
		for _, rec := range remoteNodes {
			n, err := node.NewGRPCNode(context.Background(), node.Info{
				Name:  rec.Name,
				Addr:  rec.Addr,
				Token: rec.Token,
				TLS:   rec.TLS,
			})
			if err != nil {
				slog.Warn("could not reconnect to remote node", "node", rec.Name, "addr", rec.Addr, "error", err)
				continue
			}
			node.Register(n)
			slog.Info("reconnected to remote node", "node", rec.Name, "addr", rec.Addr)
		}
	}

	mgr := session.New(st)
	srv := api.New(mgr, st)

	httpSrv := &http.Server{
		Addr:    serveAddr,
		Handler: srv.Handler(ui.Handler()),
	}

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		fmt.Printf("Conductor listening on %s\n  API: http://localhost%s\n  UI:  http://localhost%s\n  DB:  %s\n",
			serveAddr, serveAddr, serveAddr, dbPath)
		if err := httpSrv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			slog.Error("HTTP server error", "error", err)
			os.Exit(1)
		}
	}()

	<-quit
	slog.Info("meru shutting down")
	fmt.Println("\nShutting down...")
	return httpSrv.Close()
}

func dataDir() string {
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".meru")
}
