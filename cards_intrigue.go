package main

import (
	"fmt"
)

var cardsIntrigue = CardDB{
	List: `
Courtyard,2,Action,+C3
Pawn,2,Action
Secret Chamber,2,Action-Reaction
Great Hall,3,Action-Victory,+C1,+A1,#1
Masquerade,3,Action,+C2
Shanty Town,3,Action,+A2
Steward,3,Action
Swindler,3,Action-Attack,$2
Wishing Well,3,Action,+C1,+A1
Baron,4,Action,+B1
Bridge,4,Action,+B1,$1
Conspirator,4,Action,$2
Coppersmith,4,Action
Ironworks,4,Action
Mining Village,4,Action,+C1,+A2
Scout,4,Action,+A1
Duke,5,Victory
Minion,5,Action-Attack,+A1
Saboteur,5,Action-Attack
Torturer,5,Action-Attack,+C3
Tribute,5,Action
Trading Post,5,Action
Upgrade,5,Action,+C1,+A1
Harem,6,Treasure-Victory,$2,#2
Nobles,6,Action-Victory,#2
`,
	Fun: map[string]func(game *Game){
		"Courtyard": func(game *Game) {
			p := game.p
			for _, c := range game.pickHand(p, pickOpts{n: 1, exact: true}) {
				fmt.Printf("%v decks %v\n", p.name, c.name)
				p.deck = append(Pile{c}, p.deck...)
			}
		},
		"Pawn": func(game *Game) {
			v := game.getInts(game.p, "+1 Card; +1 Action; +1 Buy; +$1", 2)
			for _, i := range v {
				switch i - 1 {
				case 0:
					game.addCards(1)
				case 1:
					game.a++
				case 2:
					game.b++
				case 3:
					game.c++
				}
			}
		},
		"Secret Chamber": func(game *Game) {
			p := game.p
			game.c += len(game.DiscardList(p, game.pickHand(p, pickOpts{n: len(p.hand)})))
		},
		"Masquerade": func(game *Game) {
			m := len(game.players)
			a := make([]*Card, m)
			for i := 0; i < m; i++ {
				j := (game.p.n + i) % m
				p := game.players[j]
				selected := game.pickHand(p, pickOpts{n: 1, exact: true})
				if len(selected) > 0 {
					a[j] = selected[0]
				}
			}
			for i := 0; i < m; i++ {
				j := (i+1) % m
				left := game.players[j]
				left.hand = append(left.hand, a[i])
				// TODO: Report the gained card.
			}
			game.TrashList(game.p, game.pickHand(game.p, pickOpts{n: 1}))
		},
		"Shanty Town": func(game *Game) {
			game.revealHand(game.p)
			if !game.inHand(game.p, isAction) {
				game.addCards(2)
			}
		},
		"Steward": func(game *Game) {
			v := game.getInts(game.p, "+2 Cards; +$2; trash 2 from hand", 1)
			for _, i := range v {
				switch i - 1 {
				case 0:
					game.addCards(2)
				case 1:
					game.c += 2
				case 2:
					game.TrashList(game.p, game.pickHand(game.p, pickOpts{n: 2, exact: true}))
				}
			}
		},
		"Swindler": func(game *Game) {
			game.attack(func(other *Player) {
				c := game.reveal(other)
				other.deck = other.deck[1:]
				game.TrashCard(other, c)
				sub := pickCard(game, game.p, CardOpts{cost: game.Cost(c), exact: true})
				game.Gain(other, sub)
			})
		},
		"Wishing Well": func(game *Game) {
			p := game.p
			c := pickCard(game, p, CardOpts{any: true})
			if c == game.reveal(p) {
				fmt.Printf("%v puts %v in hand\n", p.name, c.name)
				p.hand = append(p.hand, c)
				p.deck = p.deck[1:]
			}
		},
		"Baron": func(game *Game) {
			selected := game.pickHand(game.p, pickOpts{n: 1, cond: func(c *Card) string {
				if c.name != "Estate" {
					return "must pick Estate"
				}
				return ""
			}})
			if len(selected) == 0 {
				game.MaybeGain(game.p, GetCard("Estate"))
			} else {
				game.c += 4
				game.DiscardList(game.p, selected)
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
			p := game.p
			if game.getBool(p) {
				game.trash = append(game.trash, p.played[len(p.played)-1])
				p.played = p.played[:len(p.played)-1]
				game.c += 2
			}
		},
		"Scout": func(game *Game) {
			p := game.p
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
		"Minion": func(game *Game) {
			v := game.getInts(game.p, "+$2; discard hand, +4 Cards", 1)
			for _, i := range v {
				switch i - 1 {
				case 0:
					game.c += 2
				case 1:
					p := game.p
					game.DiscardList(p, p.hand)
					p.hand = nil
					game.addCards(4)
					game.attack(func(other *Player) {
						if len(other.hand) >= 5 {
							game.DiscardList(other, other.hand)
							other.hand = nil
							game.draw(other, 4)
						}
					})
				}
			}
		},
		"Saboteur": func(game *Game) {
			game.attack(func(other *Player) {
				var v Pile
				var c *Card
				for {
					c = game.reveal(other)
					if c == nil {
						break
					}
					other.deck = other.deck[1:]
					if game.Cost(c) >= 3 {
						break
					}
					v = append(v, c)
				}
				if c != nil {
					game.TrashCard(other, c)
					sub := pickCard(game, other, CardOpts{cost: game.Cost(c) - 2, optional: true})
					if sub != nil {
						game.Gain(other, sub)
					}
				}
				if len(v) > 0 {
					game.DiscardList(other, v)
				}
			})
		},
		"Torturer": func(game *Game) {
			game.attack(func(other *Player) {
				v := game.getInts(other, "discard 2; gain Curse in hand", 1)
				switch v[0] - 1 {
				case 0:
					game.DiscardList(other, game.pickHand(other, pickOpts{n: 2, exact: true}))
				case 1:
					game.MaybeGain(other, GetCard("Curse"))
				}
			})
		},
		"Tribute": func(game *Game) {
			p := game.p
			left := game.LeftOf(p)
			var prev *Card
			for i := 0; i < 2; i++ {
				c := game.reveal(left)
				if c == nil {
					return
				}
				left.deck = left.deck[1:]
				game.DiscardList(left, Pile{c})
				if c == prev {
					break
				}
				if isAction(c) {
					game.a += 2
				}
				if isTreasure(c) {
					game.c += 2
				}
				if isVictory(c) {
					game.addCards(2)
				}
				prev = c
			}
		},
		"Trading Post": func(game *Game) {
			p := game.p
			selected := game.pickHand(p, pickOpts{n: 2, exact: true})
			game.TrashList(p, selected)
			if len(selected) == 2 && game.MaybeGain(p, GetCard("Silver")) {
				n := len(p.discard)
				p.hand = append(p.hand, p.discard[n-1])
				p.discard = p.discard[:n-1]
			}
		},
		"Upgrade": func(game *Game) {
			p := game.p
			selected := game.pickHand(p, pickOpts{n: 1, exact: true})
			if len(selected) == 0 {
				return
			}
			game.TrashCard(p, selected[0])
			c := pickCard(game, p, CardOpts{cost: game.Cost(selected[0]) + 1, exact: true})
			game.Gain(p, c)
		},
		"Nobles": func(game *Game) {
			v := game.getInts(game.p, "+3 Cards; +2 Actions", 1)
			for _, i := range v {
				switch i - 1 {
				case 0:
					game.addCards(3)
				case 1:
					game.a += 2
				}
			}
		},
	},
	VP: map[string]func(game *Game) int{
		"Duke": func(game *Game) int {
			n := 0
			for _, c := range game.p.manifest {
				if c.name == "Duchy" {
					n++
				}
			}
			return n
		},
	},
}
