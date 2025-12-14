package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"

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

type MatchResult struct {
	Type       string `json:"type"` // "match_result"
	Won        bool   `json:"won"`
	DurationMs int64  `json:"duration_ms"`
}

type HighScoreSubmit struct {
	Type       string `json:"type"` // "highscore_submit"
	PlayerName string `json:"player_name"`
	DurationMs int64  `json:"duration_ms"`
}

type HighScore struct {
	Rank       int    `json:"rank"`
	PlayerName string `json:"player_name"`
	DurationMs int64  `json:"duration_ms"`
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
	ID         int
	Players    [2]*Player
	StartTime  time.Time
	MatchEnded bool
	mu         sync.Mutex
}

var (
	lobbies        []*Lobby
	lobbyMu        sync.Mutex
	nextLobbyID    int
	totalPlayers   int
	upstashURL     string
	upstashToken   string
	redisConnected bool
)

func initRedis() {
	upstashURL = os.Getenv("UPSTASH_REDIS_REST_URL")
	upstashToken = os.Getenv("UPSTASH_REDIS_REST_TOKEN")
	if upstashURL == "" || upstashToken == "" {
		fmt.Println("Warning: UPSTASH_REDIS_REST_URL or UPSTASH_REDIS_REST_TOKEN not set, high scores disabled")
		return
	}
	redisConnected = true
	fmt.Println("Connected to Upstash Redis")
}

func upstashRequest(command []interface{}) (interface{}, error) {
	if !redisConnected {
		return nil, fmt.Errorf("redis not connected")
	}
	body, _ := json.Marshal(command)
	req, _ := http.NewRequest("POST", upstashURL, bytes.NewBuffer(body))
	req.Header.Set("Authorization", "Bearer "+upstashToken)
	req.Header.Set("Content-Type", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	respBody, _ := io.ReadAll(resp.Body)
	var result struct {
		Result interface{} `json:"result"`
		Error  string      `json:"error"`
	}
	json.Unmarshal(respBody, &result)
	if result.Error != "" {
		return nil, fmt.Errorf(result.Error)
	}
	return result.Result, nil
}

func submitHighScore(playerName string, durationMs int64) error {
	if !redisConnected {
		return fmt.Errorf("redis not connected")
	}
	// Use sorted set with duration as score (lower is better)
	// Member format: "playerName:timestamp" for uniqueness
	member := fmt.Sprintf("%s:%d", playerName, time.Now().UnixNano())
	_, err := upstashRequest([]interface{}{"ZADD", "highscores", durationMs, member})
	return err
}

func getTopScores(limit int) ([]HighScore, error) {
	if !redisConnected {
		return nil, fmt.Errorf("redis not connected")
	}
	result, err := upstashRequest([]interface{}{"ZRANGE", "highscores", "0", strconv.Itoa(limit - 1), "WITHSCORES"})
	if err != nil {
		return nil, err
	}

	// Result is an array of [member, score, member, score, ...]
	arr, ok := result.([]interface{})
	if !ok {
		return nil, fmt.Errorf("unexpected response format")
	}

	scores := make([]HighScore, 0)
	for i := 0; i < len(arr); i += 2 {
		member := arr[i].(string)
		scoreStr := arr[i+1].(string)
		score, _ := strconv.ParseInt(scoreStr, 10, 64)

		// Parse "playerName:timestamp" format
		parts := strings.Split(member, ":")
		playerName := parts[0]
		if len(parts) > 2 {
			// Handle names with colons by rejoining all but last part
			playerName = strings.Join(parts[:len(parts)-1], ":")
		}
		scores = append(scores, HighScore{
			Rank:       len(scores) + 1,
			PlayerName: playerName,
			DurationMs: score,
		})
	}
	return scores, nil
}

// SERVER
func StartServer() {
	initRedis()

	// High scores API endpoint
	http.HandleFunc("/highscores", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("Access-Control-Allow-Origin", "*")
		scores, err := getTopScores(10)
		if err != nil {
			json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
			return
		}
		json.NewEncoder(w).Encode(scores)
	})

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
				lobby.StartTime = time.Now() // Match begins when both players join
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
		// Read raw message to determine type
		_, rawMsg, err := p.Conn.ReadMessage()
		if err != nil {
			return
		}

		// Check if it's a high score submission
		var msgType struct {
			Type string `json:"type"`
		}
		json.Unmarshal(rawMsg, &msgType)

		if msgType.Type == "highscore_submit" {
			var submit HighScoreSubmit
			if json.Unmarshal(rawMsg, &submit) == nil {
				handleHighScoreSubmit(submit.PlayerName, submit.DurationMs)
			}
			continue
		}

		// Otherwise treat as game state
		var st RemoteState
		if json.Unmarshal(rawMsg, &st) != nil {
			continue
		}

		p.State.X = st.X
		p.State.Y = st.Y
		p.State.HP = st.HP
		p.State.Attack = st.Attack
		p.State.Facing = st.Facing

		broadcastToLobby(lobby, p)

		// Detect win condition: this player's HP reached 0
		lobby.mu.Lock()
		if p.State.HP <= 0 && !lobby.MatchEnded && !lobby.StartTime.IsZero() {
			lobby.MatchEnded = true
			durationMs := time.Since(lobby.StartTime).Milliseconds()

			// Find opponent (the winner)
			var winner *Player
			for _, player := range lobby.Players {
				if player != nil && player != p {
					winner = player
					break
				}
			}

			// Send match results to both players
			if winner != nil {
				// Winner gets the duration for high score submission
				winner.Conn.WriteJSON(MatchResult{
					Type:       "match_result",
					Won:        true,
					DurationMs: durationMs,
				})
				fmt.Printf("Match ended in lobby %d - duration: %dms\n", lobby.ID, durationMs)
			}
			// Loser just gets notified they lost
			p.Conn.WriteJSON(MatchResult{
				Type:       "match_result",
				Won:        false,
				DurationMs: durationMs,
			})
		}
		lobby.mu.Unlock()

		// Clear one-time flags after broadcast
		p.State.Attack = false
	}
}

// Handle high score submission from winner
func handleHighScoreSubmit(playerName string, durationMs int64) {
	if len(playerName) < 1 || len(playerName) > 12 {
		return
	}
	err := submitHighScore(playerName, durationMs)
	if err != nil {
		fmt.Println("Failed to submit high score:", err)
	} else {
		fmt.Printf("High score submitted: %s - %dms\n", playerName, durationMs)
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
	matchResultChan := make(chan MatchResult, 1)

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
			_, rawMsg, err := c.ReadMessage()
			if err != nil {
				return
			}

			// Check message type
			var msgType struct {
				Type string `json:"type"`
			}
			json.Unmarshal(rawMsg, &msgType)

			if msgType.Type == "match_result" {
				var result MatchResult
				if json.Unmarshal(rawMsg, &result) == nil {
					matchResultChan <- result
				}
				continue
			}

			// Otherwise it's a game state
			var st RemoteState
			if json.Unmarshal(rawMsg, &st) == nil {
				netChan <- st
			}
		}
	}()

	game.Run(netChan, matchResultChan, func(msg interface{}) {
		c.WriteJSON(msg)
	})
}
