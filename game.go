package main

import (
	"fmt"
	"os"
	"time"

	"github.com/gdamore/tcell/v2"
)

// Arena dimensions
const (
	arenaLeft   = 1
	arenaTop    = 3
	arenaRight  = 78
	arenaBottom = 23
)

type Game struct {
	screen      tcell.Screen
	PlayerX     int
	PlayerY     int
	hp          int
	lastHit     time.Time
	hitFlash    time.Time
	playerColor tcell.Color
	facing     rune // last direction: 'w', 'a', 's', 'd'
	attacking  time.Time
	lastAttack time.Time // for attack cooldown
	isLeft     bool      // which side this player is on

	enemyX         int
	enemyY         int
	enemyHP        int
	enemyHitFlash  time.Time
	enemyColor     tcell.Color
	enemyFacing    rune
	enemyAttack    time.Time
	enemyConnected bool

	totalPlayers int
}

func NewGame(isLeft bool) *Game {
	s, _ := tcell.NewScreen()
	s.Init()
	s.Clear()
	
	playerCol := tcell.ColorBlue
	enemyCol := tcell.ColorRed
	if !isLeft {
		playerCol, enemyCol = enemyCol, playerCol
	}
	// Set initial enemy position and facing based on which side we're on
	enemyX := 65 // enemy on right if we're on left
	playerFacing := 'd' // face right if on left
	enemyFacing := 'a'  // enemy faces left if on right
	if !isLeft {
		enemyX = 10          // enemy on left if we're on right
		playerFacing = 'a'   // face left if on right
		enemyFacing = 'd'    // enemy faces right if on left
	}
	return &Game{
		screen:      s,
		PlayerX:     0,  // will be set by server
		PlayerY:     12, // temporary, will be set by server
		hp:          100,
		playerColor: playerCol,
		enemyColor:  enemyCol,
		facing:      rune(playerFacing),
		isLeft:      isLeft,
		enemyX:      enemyX,
		enemyY:      12,
		enemyHP:     100,
		enemyFacing: rune(enemyFacing),
	}
}

// Check if attacker at (ax, ay) with sword facing 'facing' can hit target at (tx, ty)
// Both characters are 2x2, sword extends 2 cells from attacker
func canHit(ax, ay int, facing rune, tx, ty int) bool {
	// Get both sword positions (2-char sword)
	var sx1, sy1, sx2, sy2 int
	switch facing {
	case 'w':
		sx1, sy1 = ax, ay-1
		sx2, sy2 = ax, ay-2
	case 's':
		sx1, sy1 = ax, ay+2
		sx2, sy2 = ax, ay+3
	case 'a':
		sx1, sy1 = ax-1, ay
		sx2, sy2 = ax-2, ay
	case 'd':
		sx1, sy1 = ax+2, ay
		sx2, sy2 = ax+3, ay
	default:
		sx1, sy1 = ax+2, ay
		sx2, sy2 = ax+3, ay
	}

	// Check if either sword position overlaps with target's 2x2 area
	// Target occupies (tx, ty) to (tx+1, ty+1)
	if (sx1 >= tx && sx1 <= tx+1 && sy1 >= ty && sy1 <= ty+1) ||
		(sx2 >= tx && sx2 <= tx+1 && sy2 >= ty && sy2 <= ty+1) {
		return true
	}

	// Also check if characters themselves overlap (melee range)
	// Attacker occupies (ax, ay) to (ax+1, ay+1)
	// Check if the two 2x2 boxes are within 1 cell of each other
	axMax, ayMax := ax+1, ay+1
	txMax, tyMax := tx+1, ty+1

	// Characters are in hit range if gap is <= 1 cell
	xOverlap := ax <= txMax+1 && axMax >= tx-1
	yOverlap := ay <= tyMax+1 && ayMax >= ty-1

	return xOverlap && yOverlap
}

func (g *Game) checkCollision(x, y int, facing rune) bool {
	return canHit(x, y, facing, g.PlayerX, g.PlayerY)
}

// Check if we can hit the enemy from our position
func (g *Game) canHitEnemy() bool {
	return canHit(g.PlayerX, g.PlayerY, g.facing, g.enemyX, g.enemyY)
}

// Get sword position based on character position and facing direction
// Sword appears next to top of 2x2 grid on the side they're facing
func (g *Game) getSwordPosition(x, y int, facing rune) (int, int) {
	switch facing {
	case 'w': // facing up - sword above top-left
		return x, y - 1
	case 's': // facing down - sword below bottom-left
		return x, y + 2
	case 'a': // facing left - sword to left of top-left
		return x - 1, y
	case 'd': // facing right - sword to right of top-right
		return x + 2, y
	default:
		return x + 2, y // default right
	}
}

// Draw a 2-character sword based on facing direction
func (g *Game) drawSword(x, y int, facing rune, style tcell.Style) {
	switch facing {
	case 'w': // facing up - vertical sword above
		g.screen.SetContent(x, y-1, '|', nil, style)
		g.screen.SetContent(x, y-2, '|', nil, style)
	case 's': // facing down - vertical sword below
		g.screen.SetContent(x, y+2, '|', nil, style)
		g.screen.SetContent(x, y+3, '|', nil, style)
	case 'a': // facing left - horizontal sword to left
		g.screen.SetContent(x-1, y, '-', nil, style)
		g.screen.SetContent(x-2, y, '-', nil, style)
	case 'd': // facing right - horizontal sword to right
		g.screen.SetContent(x+2, y, '-', nil, style)
		g.screen.SetContent(x+3, y, '-', nil, style)
	default: // default right
		g.screen.SetContent(x+2, y, '-', nil, style)
		g.screen.SetContent(x+3, y, '-', nil, style)
	}
}

// Draw a 2x2 character sprite based on facing direction
// Little knight/warrior that faces the direction they're moving
func (g *Game) drawCharacter(x, y int, facing rune, style tcell.Style) {
	switch facing {
	case 'w': // facing up
		// ^_^
		// /|\
		g.screen.SetContent(x, y, 'o', nil, style)
		g.screen.SetContent(x+1, y, '^', nil, style)
		g.screen.SetContent(x, y+1, '|', nil, style)
		g.screen.SetContent(x+1, y+1, '\\', nil, style)
	case 's': // facing down
		// v_v
		// /|\
		g.screen.SetContent(x, y, 'v', nil, style)
		g.screen.SetContent(x+1, y, 'o', nil, style)
		g.screen.SetContent(x, y+1, '/', nil, style)
		g.screen.SetContent(x+1, y+1, '|', nil, style)
	case 'a': // facing left
		// <o
		// /|
		g.screen.SetContent(x, y, '<', nil, style)
		g.screen.SetContent(x+1, y, 'o', nil, style)
		g.screen.SetContent(x, y+1, '/', nil, style)
		g.screen.SetContent(x+1, y+1, '|', nil, style)
	case 'd': // facing right
		// o>
		// |\
		g.screen.SetContent(x, y, 'o', nil, style)
		g.screen.SetContent(x+1, y, '>', nil, style)
		g.screen.SetContent(x, y+1, '|', nil, style)
		g.screen.SetContent(x+1, y+1, '\\', nil, style)
	default: // default right
		g.screen.SetContent(x, y, 'o', nil, style)
		g.screen.SetContent(x+1, y, '>', nil, style)
		g.screen.SetContent(x, y+1, '|', nil, style)
		g.screen.SetContent(x+1, y+1, '\\', nil, style)
	}
}

func (g *Game) Run(netChan <-chan RemoteState, sendState func(RemoteState)) {
	ticker := time.NewTicker(30 * time.Millisecond)
	defer g.screen.Fini()
	heartbeat := 150 * time.Millisecond
	lastSend := time.Now()

	// Track key states
	keysHeld := make(map[rune]bool)
	attackPressed := false
	quitChan := make(chan bool)

	// Use tcell's event polling for responsive input
	go func() {
		for {
			ev := g.screen.PollEvent()
			switch ev := ev.(type) {
			case *tcell.EventKey:
				var key rune
				isPress := true

				switch ev.Key() {
				case tcell.KeyRune:
					key = ev.Rune()
				case tcell.KeyUp:
					key = 'w'
				case tcell.KeyDown:
					key = 's'
				case tcell.KeyLeft:
					key = 'a'
				case tcell.KeyRight:
					key = 'd'
				case tcell.KeyEscape, tcell.KeyCtrlC:
					quitChan <- true
					return
				}

				if key != 0 {
					if key == ' ' && isPress {
						attackPressed = true
					} else {
						keysHeld[key] = isPress
					}
				}
			}
		}
	}()

	attackCooldown := 300 * time.Millisecond

	for {
		select {
		case <-quitChan:
			return

		case <-ticker.C:
			oldX, oldY, oldHP := g.PlayerX, g.PlayerY, g.hp
			step := 1
			moved := false

			// Process held movement keys
			if keysHeld['w'] {
				g.PlayerY -= step
				g.facing = 'w'
				moved = true
			}
			if keysHeld['s'] {
				g.PlayerY += step
				g.facing = 's'
				moved = true
			}
			if keysHeld['a'] {
				g.PlayerX -= step
				g.facing = 'a'
				moved = true
			}
			if keysHeld['d'] {
				g.PlayerX += step
				g.facing = 'd'
				moved = true
			}

			// Clear movement keys (require re-press)
			keysHeld['w'] = false
			keysHeld['s'] = false
			keysHeld['a'] = false
			keysHeld['d'] = false

			// Clamp to arena bounds (character is 2x2)
			if g.PlayerX < arenaLeft+1 {
				g.PlayerX = arenaLeft + 1
			}
			if g.PlayerX > arenaRight-2 {
				g.PlayerX = arenaRight - 2
			}
			if g.PlayerY < arenaTop+1 {
				g.PlayerY = arenaTop + 1
			}
			if g.PlayerY > arenaBottom-2 {
				g.PlayerY = arenaBottom - 2
			}

			// Handle attack with cooldown
			attacking := false
			if attackPressed && time.Since(g.lastAttack) >= attackCooldown {
				attacking = true
				g.attacking = time.Now()
				g.lastAttack = time.Now()
				if g.canHitEnemy() {
					g.enemyHitFlash = time.Now()
				}
			}
			attackPressed = false

			if moved || g.PlayerX != oldX || g.PlayerY != oldY || g.hp != oldHP || attacking {
				sendState(RemoteState{
					X:      g.PlayerX,
					Y:      g.PlayerY,
					HP:     g.hp,
					Attack: attacking,
					Facing: g.facing,
				})
				lastSend = time.Now()
			}

			// Heartbeat
			if time.Since(lastSend) > heartbeat {
				sendState(RemoteState{
					X:      g.PlayerX,
					Y:      g.PlayerY,
					HP:     g.hp,
					Attack: false,
					Facing: g.facing,
				})
				lastSend = time.Now()
			}

			g.draw()

		case st := <-netChan:
			// Handle player count updates
			if st.TotalPlayers > 0 {
				g.totalPlayers = st.TotalPlayers
			}

			// Skip if this is just a player count message
			if st.X == 0 && st.Y == 0 && st.HP == 0 {
				continue
			}

			g.enemyConnected = true
			g.enemyX = st.X
			g.enemyY = st.Y
			g.enemyHP = st.HP
			if st.Facing != 0 {
				g.enemyFacing = st.Facing
			}

			// Enemy attacked - check if we got hit
			if st.Attack {
				g.enemyAttack = time.Now()
				if time.Since(g.lastHit) > 300*time.Millisecond {
					if g.checkCollision(g.enemyX, g.enemyY, g.enemyFacing) {
						g.hp -= 10
						g.hitFlash = time.Now()
						g.lastHit = time.Now()
						// Immediately send updated HP so attacker knows they hit
						sendState(RemoteState{
							X:      g.PlayerX,
							Y:      g.PlayerY,
							HP:     g.hp,
							Attack: false,
							Facing: g.facing,
						})
						lastSend = time.Now()
					}
				}
			}
		}
	}
}

func (g *Game) drawArena() {
	borderStyle := tcell.StyleDefault.Foreground(tcell.ColorDarkGray)
	flagBlue := tcell.StyleDefault.Foreground(tcell.ColorBlue)
	flagRed := tcell.StyleDefault.Foreground(tcell.ColorRed)

	// Top border with flags
	g.screen.SetContent(arenaLeft, arenaTop, '╔', nil, borderStyle)
	g.screen.SetContent(arenaRight, arenaTop, '╗', nil, borderStyle)
	for x := arenaLeft + 1; x < arenaRight; x++ {
		g.screen.SetContent(x, arenaTop, '═', nil, borderStyle)
	}

	// Bottom border
	g.screen.SetContent(arenaLeft, arenaBottom, '╚', nil, borderStyle)
	g.screen.SetContent(arenaRight, arenaBottom, '╝', nil, borderStyle)
	for x := arenaLeft + 1; x < arenaRight; x++ {
		g.screen.SetContent(x, arenaBottom, '═', nil, borderStyle)
	}

	// Side borders
	for y := arenaTop + 1; y < arenaBottom; y++ {
		g.screen.SetContent(arenaLeft, y, '║', nil, borderStyle)
		g.screen.SetContent(arenaRight, y, '║', nil, borderStyle)
	}

	// Blue flag on left side (top)
	g.screen.SetContent(arenaLeft+2, arenaTop-1, '▄', nil, flagBlue)
	g.screen.SetContent(arenaLeft+3, arenaTop-1, '▄', nil, flagBlue)
	g.screen.SetContent(arenaLeft+2, arenaTop-2, '█', nil, flagBlue)
	g.screen.SetContent(arenaLeft+3, arenaTop-2, '▀', nil, flagBlue)
	g.screen.SetContent(arenaLeft+1, arenaTop-2, '│', nil, borderStyle)
	g.screen.SetContent(arenaLeft+1, arenaTop-1, '│', nil, borderStyle)

	// Red flag on right side (top)
	g.screen.SetContent(arenaRight-3, arenaTop-1, '▄', nil, flagRed)
	g.screen.SetContent(arenaRight-2, arenaTop-1, '▄', nil, flagRed)
	g.screen.SetContent(arenaRight-3, arenaTop-2, '▀', nil, flagRed)
	g.screen.SetContent(arenaRight-2, arenaTop-2, '█', nil, flagRed)
	g.screen.SetContent(arenaRight-1, arenaTop-2, '│', nil, borderStyle)
	g.screen.SetContent(arenaRight-1, arenaTop-1, '│', nil, borderStyle)

	// Center line (subtle)
	centerX := (arenaLeft + arenaRight) / 2
	for y := arenaTop + 1; y < arenaBottom; y += 2 {
		g.screen.SetContent(centerX, y, '·', nil, borderStyle)
	}
}

func (g *Game) draw() {
	g.screen.Clear()

	// Draw arena first (background)
	g.drawArena()

	// local player - little knight facing their direction
	style := tcell.StyleDefault.Foreground(g.playerColor)
	if time.Since(g.hitFlash) < 200*time.Millisecond {
		style = style.Foreground(tcell.ColorWhite)
	}
	g.drawCharacter(g.PlayerX, g.PlayerY, g.facing, style)

	// enemy (only if connected)
	if g.enemyConnected {
		eStyle := tcell.StyleDefault.Foreground(g.enemyColor)
		if time.Since(g.enemyHitFlash) < 200*time.Millisecond {
			eStyle = eStyle.Foreground(tcell.ColorWhite)
		}
		g.drawCharacter(g.enemyX, g.enemyY, g.enemyFacing, eStyle)
	}

	// Sword slashes (drawn last so they appear on top)
	slashDuration := 150 * time.Millisecond

	// Local player sword slash (matches player color)
	if time.Since(g.attacking) < slashDuration {
		playerSwordStyle := tcell.StyleDefault.Foreground(g.playerColor)
		g.drawSword(g.PlayerX, g.PlayerY, g.facing, playerSwordStyle)
	}

	// Enemy sword slash (matches enemy color)
	if g.enemyConnected && time.Since(g.enemyAttack) < slashDuration {
		enemySwordStyle := tcell.StyleDefault.Foreground(g.enemyColor)
		g.drawSword(g.enemyX, g.enemyY, g.enemyFacing, enemySwordStyle)
	}

	// HP display (centered horizontally)
	centerX := (arenaLeft + arenaRight) / 2
	localHP := fmt.Sprintf("You: %d HP", g.hp)
	startX := centerX - len(localHP)/2
	for i, r := range localHP {
		g.screen.SetContent(startX+i, 0, r, nil, tcell.StyleDefault.Foreground(g.playerColor))
	}
	if g.enemyConnected {
		enemyHP := fmt.Sprintf("Enemy: %d HP", g.enemyHP)
		startX = centerX - len(enemyHP)/2
		for i, r := range enemyHP {
			g.screen.SetContent(startX+i, 1, r, nil, tcell.StyleDefault.Foreground(g.enemyColor))
		}
	} else {
		msg := "Waiting for opponent..."
		startX = centerX - len(msg)/2
		for i, r := range msg {
			g.screen.SetContent(startX+i, 1, r, nil, tcell.StyleDefault)
		}
	}

	// Total players online (bottom right of terminal)
	if g.totalPlayers > 0 {
		w, h := g.screen.Size()
		online := fmt.Sprintf("%d online", g.totalPlayers)
		for i, r := range online {
			g.screen.SetContent(w-len(online)+i, h-1, r, nil, tcell.StyleDefault.Foreground(tcell.ColorDarkGray))
		}
	}

	// death
	if g.hp <= 0 {
		msg := "YOU DIED"
		for i, r := range msg {
			g.screen.SetContent(10+i, 5, r, nil, tcell.StyleDefault)
		}
		g.screen.Show()
		time.Sleep(2 * time.Second)
		g.screen.Fini()
		os.Exit(0)
	}
	if g.enemyHP <= 0 {
		msg := "YOU WIN!"
		for i, r := range msg {
			g.screen.SetContent(10+i, 5, r, nil, tcell.StyleDefault)
		}
		g.screen.Show()
		time.Sleep(2 * time.Second)
		g.screen.Fini()
		os.Exit(0)
	}

	g.screen.Show()
}
