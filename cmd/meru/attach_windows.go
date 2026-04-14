//go:build windows

package main

import "github.com/gorilla/websocket"

func watchResize(_ *websocket.Conn, _ int) func() {
	return func() {}
}
