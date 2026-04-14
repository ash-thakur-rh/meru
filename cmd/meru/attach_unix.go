//go:build !windows

package main

import (
	"os"
	"os/signal"
	"syscall"

	"github.com/gorilla/websocket"
	"golang.org/x/term"
)

func watchResize(conn *websocket.Conn, fd int) func() {
	winch := make(chan os.Signal, 1)
	signal.Notify(winch, syscall.SIGWINCH)
	go func() {
		for range winch {
			if cols, rows, err := term.GetSize(fd); err == nil {
				sendResize(conn, cols, rows)
			}
		}
	}()
	return func() {
		signal.Stop(winch)
		close(winch)
	}
}
