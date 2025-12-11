package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"net"
	"os"
)

func StartClient(addr string) {
	conn, err := net.Dial("tcp", addr)
	if err != nil {
		fmt.Printf("Failed to connect to server at %s: %v\n", addr, err)
		fmt.Println("Make sure the server is running with: ./cli-duel server")
		return
	}
	defer conn.Close()

	var isLeft bool
	sc := bufio.NewScanner(conn)
	// wait for role packet
	if sc.Scan() {
		var st RemoteState
		json.Unmarshal(sc.Bytes(), &st)
		isLeft = st.Player1
	}

	game := NewGame(isLeft)
	inputChan := make(chan rune, 10)
	netChan := make(chan RemoteState, 10)

	go func() {
		reader := bufio.NewReader(os.Stdin)
		for {
			r, _, _ := reader.ReadRune()
			inputChan <- r
		}
	}()

	go func() {
		for sc.Scan() {
			var st RemoteState
			json.Unmarshal(sc.Bytes(), &st)
			netChan <- st
		}
	}()

	sendState := func(st RemoteState) {
		b, _ := json.Marshal(st)
		fmt.Fprintln(conn, string(b))
	}

	game.Run(inputChan, netChan, sendState)
}

func main() {
	if len(os.Args) > 1 && os.Args[1] == "server" {
		StartServer()
	} else {
		addr := ":9999"
		if len(os.Args) > 1 {
			addr = os.Args[1]
		}
		StartClient(addr)
	}
}
