package main

import (
	"fmt"
	"os"
)

func main() {
	if len(os.Args) < 2 {
		fmt.Println("Usage:")
		fmt.Println("  ./duel server")
		fmt.Println("  ./duel client <ws://server:port>")
		return
	}

	switch os.Args[1] {
	case "server":
		StartServer()
	case "client":
		if len(os.Args) < 3 {
			fmt.Println("Usage: ./duel client <ws://server:port>")
			return
		}
		StartClient(os.Args[2])
	default:
		fmt.Println("Unknown mode:", os.Args[1])
		fmt.Println("Usage:")
		fmt.Println("  ./duel server")
		fmt.Println("  ./duel client <ws://server:port>")
	}
}
