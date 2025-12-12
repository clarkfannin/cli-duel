package main

import (
	"bufio"
	"fmt"
	"net/http"
	"os"
	"sync"

	"github.com/gorilla/websocket"
)

type RemoteState struct {
	X       int  `json:"x"`
	Y       int  `json:"y"`
	HP      int  `json:"hp"`
	Attack  bool `json:"attack"`
	Facing  rune `json:"facing"`
	Player1 bool `json:"player1"`
}

var upgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool { return true },
}

type Player struct {
	Conn  *websocket.Conn
	State RemoteState
}

var (
	players []*Player
	mu      sync.Mutex
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
		mu.Lock()
		player.State.Player1 = len(players) == 0
		players = append(players, player)
		mu.Unlock()

		go handlePlayer(player)
	})

	fmt.Println("Server running on :8080")
	http.ListenAndServe(":8080", nil)
}

func handlePlayer(p *Player) {
	defer func() {
		p.Conn.Close()
		mu.Lock()
		for i, pl := range players {
			if pl == p {
				players = append(players[:i], players[i+1:]...)
				break
			}
		}
		mu.Unlock()
	}()

	// Initialize player state with proper position and HP
	// Arena: left=1, top=3, right=78, bottom=23
	mu.Lock()
	if p.State.Player1 {
		p.State.X = 10       // left side of arena
		p.State.Y = 12       // vertically centered
		p.State.HP = 100
	} else {
		p.State.X = 65       // right side of arena
		p.State.Y = 12       // vertically centered
		p.State.HP = 100
	}

	// Send initial role assignment
	p.Conn.WriteJSON(p.State)

	// If this is the second player, broadcast both states to both players
	if len(players) == 2 {
		for _, player := range players {
			for _, other := range players {
				if other != player {
					player.Conn.WriteJSON(other.State)
				}
			}
		}
	}
	mu.Unlock()

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

		broadcastStates()

		// Clear one-time flags after broadcast
		p.State.Attack = false
	}
}

func broadcastStates() {
	mu.Lock()
	defer mu.Unlock()
	for _, p := range players {
		for _, other := range players {
			if other == p {
				continue
			}
			p.Conn.WriteJSON(other.State)
		}
	}
}

// CLIENT
func StartClient(url string) {
	c, _, err := websocket.DefaultDialer.Dial(url, nil)
	if err != nil {
		fmt.Println("Connect error:", err)
		return
	}
	defer c.Close()

	inputChan := make(chan rune, 10)
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

	go func() {
		reader := bufio.NewReader(os.Stdin)
		for {
			r, _, _ := reader.ReadRune()
			inputChan <- r
		}
	}()

	game.Run(inputChan, netChan, func(st RemoteState) {
		c.WriteJSON(st)
	})
}
