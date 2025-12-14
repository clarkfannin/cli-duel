package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"strings"
)

const defaultServer = "wss://cli-duel.fly.dev/"
const defaultHTTPServer = "https://cli-duel.fly.dev/"

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
	case "-h", "--highscores", "highscores":
		// Show high scores leaderboard
		showHighScores()
	default:
		fmt.Println("Usage:")
		fmt.Println("  duel             - Join online match")
		fmt.Println("  duel host        - Host local server")
		fmt.Println("  duel join URL    - Join custom server")
		fmt.Println("  duel -h          - Show top 10 fastest takedowns")
	}
}

func showHighScores() {
	fmt.Println("\n=== FASTEST TAKEDOWNS ===\n")

	resp, err := http.Get(defaultHTTPServer + "highscores")
	if err != nil {
		fmt.Println("Error fetching high scores:", err)
		return
	}
	defer resp.Body.Close()

	var scores []HighScore
	if err := json.NewDecoder(resp.Body).Decode(&scores); err != nil {
		fmt.Println("Error parsing high scores:", err)
		return
	}

	if len(scores) == 0 {
		fmt.Println("No high scores yet. Be the first!")
		return
	}

	// Print header
	fmt.Printf(" %-4s %-15s %s\n", "#", "Player", "Time")
	fmt.Println(strings.Repeat("─", 30))

	// Print scores
	for _, score := range scores {
		seconds := float64(score.DurationMs) / 1000.0
		fmt.Printf(" %-4d %-15s %.2fs\n", score.Rank, score.PlayerName, seconds)
	}
	fmt.Println()
}
