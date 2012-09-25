package main

import (
	"fmt"
)

var cardsIntrigue = CardDB{
	List: `
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
	Fun: map[string]func(game *Game){
		"Courtyard": func(game *Game) {
			p := game.NowPlaying()
			var selected Pile
			selected, p.hand = game.split(p.hand, p, pickOpts{n: 1, exact: true})
			for _, c := range selected {
				fmt.Printf("%v decks %v\n", p.name, c.name)
				p.deck = append(Pile{c}, p.deck...)
			}
		},
		"Baron": func(game *Game) {
			p := game.NowPlaying()
			var selected Pile
			selected, p.hand = game.split(p.hand, p, pickOpts{n: 1, cond: func(c *Card) string {
				if c.name != "Estate" {
					return "must pick Estate"
				}
				return ""
			}})
			if len(selected) == 0 {
				game.MaybeGain(p, GetCard("Estate"))
			} else {
				game.c += 4
				game.DiscardList(p, selected)
			}
		},
		"Bridge": func(game *Game) { game.discount++ },
		"Conspirator": func(game *Game) {
			if game.aCount >= 3 {
				game.a++
				game.addCards(1)
			}
		},
		"Coppersmith": func(game *Game) {
			game.copperbonus++
		},
		"Copper": func(game *Game) {
			game.c += game.copperbonus
		},
		"Ironworks": func(game *Game) {
			c := pickGain(game, 4)
			if isAction(c) {
				game.a++
			}
			if isTreasure(c) {
				game.c++
			}
			if isVictory(c) {
				game.addCards(1)
			}
		},
		"Mining Village": func(game *Game) {
			p := game.NowPlaying()
			if game.getBool(p) {
				game.trash = append(game.trash, p.played[len(p.played)-1])
				p.played = p.played[:len(p.played)-1]
				game.c += 2
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
				var selected Pile
				selected, v = game.split(v, p, pickOpts{n: 1, exact: true})
				fmt.Printf("%v decks %v\n", p.name, selected[0].name)
				p.deck = append(selected, p.deck...)
			}
		},
	},
	VP: map[string]func(game *Game) int{
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
