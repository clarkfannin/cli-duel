package main

import (
	"fmt"
	"os"
	"time"

	"github.com/gdamore/tcell/v2"
)

type Game struct {
	screen      tcell.Screen
	playerX     int
	playerY     int
	hp          int
	lastHit     time.Time
	hitFlash    time.Time
	step        int
	playerColor tcell.Color

	enemyX        int
	enemyY        int
	enemyHP       int
	enemyHitFlash time.Time
	enemyColor    tcell.Color
	enemyAttack   time.Time
}

func NewGame(isLeft bool) *Game {
	s, _ := tcell.NewScreen()
	s.Init()
	s.Clear()
	w, h := s.Size()
	startX := 2
	playerCol := tcell.ColorBlue
	enemyCol := tcell.ColorRed
	if !isLeft {
		startX = w - 4
		playerCol, enemyCol = enemyCol, playerCol
	}
	return &Game{
		screen:      s,
		playerX:     startX,
		playerY:     h/2 - 1,
		hp:          100,
		step:        3,
		playerColor: playerCol,
		enemyColor:  enemyCol,
		enemyHP:     100,
	}
}

func (g *Game) checkCollision(x, y int) bool {
	return g.playerX-1 <= x+1 && g.playerX+2 >= x-1 &&
		g.playerY-1 <= y+1 && g.playerY+2 >= y-1
}

func (g *Game) Run(inputChan <-chan rune, netChan <-chan RemoteState, sendState func(RemoteState)) {
	ticker := time.NewTicker(30 * time.Millisecond)
	defer g.screen.Fini()
	heartbeat := 150 * time.Millisecond
	lastSend := time.Now()

	for {
		select {
		case r := <-inputChan:
			attacking := r == ' '
			oldX, oldY, oldHP := g.playerX, g.playerY, g.hp
			enemyOldHP := g.enemyHP

			switch r {
			case 'w':
				g.playerY -= g.step
			case 's':
				g.playerY += g.step
			case 'a':
				g.playerX -= g.step
			case 'd':
				g.playerX += g.step
			case 'q':
				return
			}

			// check if local player attack hits enemy
			if attacking && g.checkCollision(g.enemyX, g.enemyY) && time.Since(g.lastHit) > 300*time.Millisecond {
				g.enemyHP -= 10
				g.lastHit = time.Now()
			}

			enemyTookDamage := g.enemyHP < enemyOldHP
			if g.playerX != oldX || g.playerY != oldY || g.hp != oldHP || attacking || enemyTookDamage {
				sendState(RemoteState{
					X:           g.playerX,
					Y:           g.playerY,
					HP:          g.hp,
					Attack:      attacking,
					TookDamage:  false, // we didn't take damage from our own attack
					EnemyDamage: enemyTookDamage, // enemy took damage from our attack
				})
				lastSend = time.Now()
			}

		case st := <-netChan:
			g.enemyX = st.X
			g.enemyY = st.Y
			g.enemyHP = st.HP

			if st.EnemyDamage {
				g.hitFlash = time.Now() // we took damage (enemy hit us)
			}
			if st.TookDamage {
				g.enemyHitFlash = time.Now() // enemy took damage (we hit them)
			}
			if st.Attack {
				g.enemyAttack = time.Now()
			}

		case <-ticker.C:
			// heartbeat
			if time.Since(lastSend) > heartbeat {
				sendState(RemoteState{
					X:           g.playerX,
					Y:           g.playerY,
					HP:          g.hp,
					Attack:      false,
					TookDamage:  false,
					EnemyDamage: false,
				})
				lastSend = time.Now()
			}

			// check collision continuously if enemy attacking
			if time.Since(g.enemyAttack) < 100*time.Millisecond && time.Since(g.lastHit) > 300*time.Millisecond {
				if g.checkCollision(g.enemyX, g.enemyY) {
					g.hp -= 10
					g.hitFlash = time.Now()
					g.lastHit = time.Now()
					// send that we took damage
					sendState(RemoteState{
						X:           g.playerX,
						Y:           g.playerY,
						HP:          g.hp,
						Attack:      false,
						TookDamage:  true, // we took damage
						EnemyDamage: false,
					})
					lastSend = time.Now()
				}
			}

			g.draw()
		}
	}
}

func (g *Game) draw() {
	g.screen.Clear()

	// local player
	style := tcell.StyleDefault.Foreground(g.playerColor)
	if time.Since(g.hitFlash) < 100*time.Millisecond {
		style = style.Foreground(tcell.ColorWhite)
	}
	for dx := 0; dx < 2; dx++ {
		for dy := 0; dy < 2; dy++ {
			g.screen.SetContent(g.playerX+dx, g.playerY+dy, '@', nil, style)
		}
	}

	// enemy
	eStyle := tcell.StyleDefault.Foreground(g.enemyColor)
	if time.Since(g.enemyHitFlash) < 100*time.Millisecond {
		eStyle = eStyle.Foreground(tcell.ColorWhite)
	}
	for dx := 0; dx < 2; dx++ {
		for dy := 0; dy < 2; dy++ {
			g.screen.SetContent(g.enemyX+dx, g.enemyY+dy, 'E', nil, eStyle)
		}
	}

	// HP display
	localHP := fmt.Sprintf("Your HP: %d", g.hp)
	for i, r := range localHP {
		g.screen.SetContent(i, 0, r, nil, tcell.StyleDefault)
	}
	enemyHP := fmt.Sprintf("Enemy HP: %d", g.enemyHP)
	for i, r := range enemyHP {
		g.screen.SetContent(i, 1, r, nil, tcell.StyleDefault)
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
