package main

import (
	"fmt"
	"os"
)

const defaultServer = "wss://cli-duel.fly.dev/"

func main() {
	// No args = connect to default server
	if len(os.Args) < 2 {
		fmt.Println("Connecting to server...")
		StartClient(defaultServer)
		return
	}

	switch os.Args[1] {
	case "host":
		// Run local server for LAN play
		StartServer()
	case "join":
		// Join custom server
		if len(os.Args) < 3 {
			fmt.Println("Usage: duel join <ws://server:port>")
			return
		}
		StartClient(os.Args[2])
	default:
		fmt.Println("Usage:")
		fmt.Println("  duel          - Join online match")
		fmt.Println("  duel host     - Host local server")
		fmt.Println("  duel join URL - Join custom server")
	}
}
