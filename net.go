package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"net"
)

type RemoteState struct {
	X           int  `json:"x"`
	Y           int  `json:"y"`
	HP          int  `json:"hp"`
	Attack      bool `json:"attack"`
	TookDamage  bool `json:"tookDamage"`  // when this player took damage
	EnemyDamage bool `json:"enemyDamage"` // when enemy took damage  
	Player1     bool `json:"player1"`     // role info
}

// SERVER
func StartServer() {
	ln, err := net.Listen("tcp", ":9999")
	if err != nil {
		panic(err)
	}
	fmt.Println("Server listening on :9999")
	defer ln.Close()

	clients := []net.Conn{}
	roles := []bool{} // true = left, false = right

	for len(clients) < 2 {
		conn, _ := ln.Accept()
		clients = append(clients, conn)
		roles = append(roles, len(clients) == 1) // first client = left
		go func(c net.Conn, role bool) {
			// send role immediately
			msg := RemoteState{Player1: role}
			b, _ := json.Marshal(msg)
			fmt.Fprintln(c, string(b))
		}(conn, roles[len(clients)-1])
		fmt.Println("Client connected")
	}

	// relay messages
	for _, c := range clients {
		go func(sender net.Conn) {
			sc := bufio.NewScanner(sender)
			for sc.Scan() {
				msg := sc.Bytes()
				for _, c2 := range clients {
					if c2 != sender {
						c2.Write(msg)
						c2.Write([]byte("\n"))
					}
				}
			}
		}(c)
	}

	select {}
}
