package main

import (
	"fmt"
)

var cardsIntrigue = CardDB{
List:`
Courtyard,2,Action,+C3
Great Hall,3,Action-Victory,+C1,+A1,#1
Baron,4,Action,+B1
Bridge,4,Action,+B1,$1
Conspirator,4,Action,$2
Coppersmith,4,Action
Ironworks,4,Action
Mining Village,4,Action,+C1,+A2
Scout,4,Action,+A1
Duke,5,Victory
`,
Fun: map[string]func(game *Game) {
	"Courtyard": func(game *Game) {
		p := game.NowPlaying()
		if len(p.hand) > 0 {
			for i, b := range pickHand(game, p, 1, true, nil) {
				if b {
					fmt.Printf("%v decks %v\n", p.name, p.hand[i].name)
					p.hand = append(p.hand[:i], p.hand[i+1:]...)
					p.deck = append(p.hand[i:i+1], p.deck...)
					break
				}
			}
		}
	},
	"Baron": func(game *Game) {
		p := game.NowPlaying()

		p.b++
		selected := pickHand(game, p, 1, false, func(c *Card) string {
			if c.name != "Estate" {
				return "must pick Estate"
			}
			return ""
		})
		for i := len(p.hand)-1; i >= 0; i-- {
			if selected[i] {
				p.discard = append(p.discard, p.hand[i])
				p.hand = append(p.hand[:i], p.hand[i+1:]...)
				game.Report(Event{s:"discard", n:p.n, i:1})
				p.c+=4
				return
			}
		}
		game.MaybeGain(p, GetCard("Estate"))
	},
	"Bridge": func(game *Game) { game.discount++ },
	"Conspirator": func(game *Game) {
		p := game.NowPlaying()
		if p.aCount >= 3 {
			p.a++
			game.draw(p, 1)
		}
	},
	"Coppersmith": func(game *Game) {
		game.copperbonus++
	},
	"Copper": func(game *Game) {
		game.NowPlaying().c += game.copperbonus
	},
	"Ironworks": func(game *Game) {
		p := game.NowPlaying()
		c := pickGain(game, 4)
		if isAction(c) {
			p.a++
		}
		if isTreasure(c) {
			p.c++
		}
		if isVictory(c) {
			p.c++
			game.draw(p, 1)
		}
	},
	"Mining Village": func(game *Game) {
		p := game.NowPlaying()
		if game.getBool(p) {
			game.trash = append(game.trash, p.played[len(p.played)-1])
			p.played = p.played[:len(p.played)-1]
			p.c += 2
		}
	},
	"Scout": func(game *Game) {
		p := game.NowPlaying()
		var v Pile
		for n := 0; n < 4 && p.MaybeShuffle(); n++ {
			c := game.reveal(p)
			if isVictory(c) {
				fmt.Printf("%v puts %v in hand\n", p.name, c.name)
				p.hand = append(p.hand, c)
			} else {
				v = append(v, c)
			}
			p.deck = p.deck[1:]
		}
		for len(v) > 0 {
			for i, b := range game.pick(v, p, pickOpts{n:1, exact:true}) {
				if b {
					fmt.Printf("%v decks %v\n", p.name, v[i].name)
					p.deck = append(v[i:i+1], p.deck...)
					v = append(v[:i], v[i+1:]...)
					break
				}
			}
		}
	},
},
VP: map[string]func(game *Game) int {
	"Duke": func(game *Game) int {
		n := 0
		for _, c := range game.NowPlaying().manifest {
			if c.name == "Duchy" {
				n++
			}
		}
		return n
	},
},
}
