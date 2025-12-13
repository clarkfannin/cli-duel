package main

import (
	"fmt"
	"net/http"
	"sync"

	"github.com/gorilla/websocket"
)

type RemoteState struct {
	X            int  `json:"x"`
	Y            int  `json:"y"`
	HP           int  `json:"hp"`
	Attack       bool `json:"attack"`
	Facing       rune `json:"facing"`
	Player1      bool `json:"player1"`
	TotalPlayers int  `json:"total_players,omitempty"`
}

var upgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool { return true },
}

type Player struct {
	Conn  *websocket.Conn
	State RemoteState
	Lobby *Lobby
}

type Lobby struct {
	ID      int
	Players [2]*Player
	mu      sync.Mutex
}

var (
	lobbies      []*Lobby
	lobbyMu      sync.Mutex
	nextLobbyID  int
	totalPlayers int
)

// SERVER
func StartServer() {
	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		c, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			fmt.Println("Upgrade error:", err)
			return
		}
		player := &Player{Conn: c}

		// Find or create a lobby
		lobbyMu.Lock()
		var lobby *Lobby
		for _, l := range lobbies {
			l.mu.Lock()
			if l.Players[1] == nil {
				// Found a waiting lobby
				lobby = l
				player.State.Player1 = false
				lobby.Players[1] = player
				player.Lobby = lobby
				l.mu.Unlock()
				break
			}
			l.mu.Unlock()
		}

		if lobby == nil {
			// Create new lobby
			lobby = &Lobby{ID: nextLobbyID}
			nextLobbyID++
			player.State.Player1 = true
			lobby.Players[0] = player
			player.Lobby = lobby
			lobbies = append(lobbies, lobby)
		}
		totalPlayers++
		lobbyMu.Unlock()

		// Initialize player position before any broadcasts
		if player.State.Player1 {
			player.State.X = 10
			player.State.Y = 12
			player.State.HP = 100
		} else {
			player.State.X = 65
			player.State.Y = 12
			player.State.HP = 100
		}
		// Send initial state to the new player first
		player.Conn.WriteJSON(player.State)

		fmt.Printf("Player joined lobby %d (player1: %v) - %d online\n", lobby.ID, player.State.Player1, totalPlayers)
		broadcastPlayerCount()
		go handlePlayer(player)
	})

	fmt.Println("Server running on :8080")
	http.ListenAndServe(":8080", nil)
}

func handlePlayer(p *Player) {
	lobby := p.Lobby

	defer func() {
		p.Conn.Close()
		lobby.mu.Lock()
		// Remove player from lobby
		if lobby.Players[0] == p {
			lobby.Players[0] = nil
		} else if lobby.Players[1] == p {
			lobby.Players[1] = nil
		}
		// Clean up empty lobbies
		if lobby.Players[0] == nil && lobby.Players[1] == nil {
			lobbyMu.Lock()
			for i, l := range lobbies {
				if l == lobby {
					lobbies = append(lobbies[:i], lobbies[i+1:]...)
					break
				}
			}
			lobbyMu.Unlock()
		}
		lobby.mu.Unlock()

		lobbyMu.Lock()
		totalPlayers--
		fmt.Printf("Player left lobby %d - %d online\n", lobby.ID, totalPlayers)
		lobbyMu.Unlock()
		broadcastPlayerCount()
	}()

	// If both players are in the lobby, send each other's state
	lobby.mu.Lock()
	if lobby.Players[0] != nil && lobby.Players[1] != nil {
		lobby.Players[0].Conn.WriteJSON(lobby.Players[1].State)
		lobby.Players[1].Conn.WriteJSON(lobby.Players[0].State)
	}
	lobby.mu.Unlock()

	for {
		var st RemoteState
		err := p.Conn.ReadJSON(&st)
		if err != nil {
			return
		}
		p.State.X = st.X
		p.State.Y = st.Y
		p.State.HP = st.HP
		p.State.Attack = st.Attack
		p.State.Facing = st.Facing

		broadcastToLobby(lobby, p)

		// Clear one-time flags after broadcast
		p.State.Attack = false
	}
}

func broadcastToLobby(lobby *Lobby, sender *Player) {
	lobby.mu.Lock()
	defer lobby.mu.Unlock()
	for _, p := range lobby.Players {
		if p != nil && p != sender {
			p.Conn.WriteJSON(sender.State)
		}
	}
}

func broadcastPlayerCount() {
	lobbyMu.Lock()
	count := totalPlayers
	lobbyMu.Unlock()

	msg := RemoteState{TotalPlayers: count}
	lobbyMu.Lock()
	for _, lobby := range lobbies {
		lobby.mu.Lock()
		for _, p := range lobby.Players {
			if p != nil {
				p.Conn.WriteJSON(msg)
			}
		}
		lobby.mu.Unlock()
	}
	lobbyMu.Unlock()
}

// CLIENT
func StartClient(url string) {
	c, _, err := websocket.DefaultDialer.Dial(url, nil)
	if err != nil {
		fmt.Println("Connect error:", err)
		return
	}
	defer c.Close()

	netChan := make(chan RemoteState, 10)

	// Wait for initial role assignment
	var isLeft bool
	var st RemoteState
	err = c.ReadJSON(&st)
	if err != nil {
		fmt.Println("Failed to receive role:", err)
		return
	}
	isLeft = st.Player1

	game := NewGame(isLeft)
	// Set initial position from server
	game.PlayerX = st.X
	game.PlayerY = st.Y

	go func() {
		for {
			var st RemoteState
			err := c.ReadJSON(&st)
			if err != nil {
				return
			}
			netChan <- st
		}
	}()

	game.Run(netChan, func(st RemoteState) {
		c.WriteJSON(st)
	})
}
