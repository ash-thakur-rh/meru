package main

import (
	"fmt"
	"log/slog"
	"net"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"

	"github.com/spf13/cobra"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/credentials/insecure"

	"github.com/ash-thakur-rh/meru/internal/logwriter"
	pb "github.com/ash-thakur-rh/meru/internal/proto"
)

var (
	serveAddr    string
	serveToken   string
	serveTLSCert string
	serveTLSKey  string
)

var serveCmd = &cobra.Command{
	Use:   "serve",
	Short: "Start the gRPC node server",
	RunE:  runServe,
}

func init() {
	serveCmd.Flags().StringVarP(&serveAddr, "addr", "a", ":9090", "gRPC listen address")
	serveCmd.Flags().StringVar(&serveToken, "token", "", "Bearer token for authentication (required)")
	serveCmd.Flags().StringVar(&serveTLSCert, "tls-cert", "", "TLS certificate file (enables TLS)")
	serveCmd.Flags().StringVar(&serveTLSKey, "tls-key", "", "TLS key file")
	serveCmd.MarkFlagRequired("token") //nolint:errcheck
	rootCmd.AddCommand(serveCmd)
}

func runServe(cmd *cobra.Command, args []string) error {
	// Log to ~/.meru/meru-node.log (best-effort; falls back to stderr)
	home, _ := os.UserHomeDir()
	logDir := filepath.Join(home, ".meru")
	logCloser, err := logwriter.Setup(logDir)
	if err != nil {
		fmt.Fprintf(os.Stderr, "warn: could not open log file: %v (logging to stderr only)\n", err)
	} else {
		defer logCloser.Close()
	}

	lis, err := net.Listen("tcp", serveAddr)
	if err != nil {
		return fmt.Errorf("listen %s: %w", serveAddr, err)
	}

	var creds credentials.TransportCredentials
	if serveTLSCert != "" {
		creds, err = credentials.NewServerTLSFromFile(serveTLSCert, serveTLSKey)
		if err != nil {
			return fmt.Errorf("load TLS: %w", err)
		}
	} else {
		creds = insecure.NewCredentials()
	}

	handler := newNodeHandler()
	srv := grpc.NewServer(
		grpc.Creds(creds),
		grpc.UnaryInterceptor(authUnaryInterceptor(serveToken)),
		grpc.StreamInterceptor(authStreamInterceptor(serveToken)),
	)
	pb.RegisterMeruNodeServer(srv, handler)

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)

	tlsNote := ""
	if serveTLSCert != "" {
		tlsNote = " (TLS)"
	}
	hostname, _ := os.Hostname()
	slog.Info("meru-node starting", "addr", serveAddr, "tls", serveTLSCert != "", "host", hostname)
	fmt.Printf("meru-node listening on %s%s  (host: %s)\n", serveAddr, tlsNote, hostname)

	go func() {
		if err := srv.Serve(lis); err != nil {
			slog.Error("gRPC server error", "error", err)
			os.Exit(1)
		}
	}()

	<-quit
	slog.Info("meru-node shutting down")
	fmt.Println("\nShutting down...")
	srv.GracefulStop()
	return nil
}
