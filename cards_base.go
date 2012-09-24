package main

import (
	"fmt"
)

var cardsBase = `
Copper,0,Treasure,$1
Silver,3,Treasure,$2
Gold,6,Treasure,$3
Estate,2,Victory,#1
Duchy,5,Victory,#3
Province,8,Victory,#6
Curse,0,Curse,#-1

Cellar,2,Action,+A1
Chapel,2,Action
Moat,2,Action-Reaction,+C2
Chancellor,3,Action,$2
Village,3,Action,+C1,+A2
Woodcutter,3,Action,+B1,$2
Workshop,3,Action
Bureaucrat,4,Action-Attack
Feast,4,Action
Gardens,4,Victory
Militia,4,Action-Attack,$2
Moneylender,4,Action
Remodel,4,Action
Smithy,4,Action,+C3
Spy,4,Action-Attack,+C1,+A1
Thief,4,Action-Attack
Throne Room,4,Action
Council Room,5,Action,+C4,+B1
Festival,5,Action,+A2,+B1,$2
Laboratory,5,Action,+C2,+A1
Library,5,Action
Market,5,Action,+C1,+A1,+B1,$1
Mine,5,Action
Witch,5,Action-Attack,+C2
Adventurer,6,Action
`

var cardsBaseAct = map[string]func(game *Game) {
	"Cellar": func(game *Game) {
		p := game.NowPlaying()
		selected := pickHand(game, p, len(p.hand), false, nil)
		count := 0
		for i := len(p.hand)-1; i >= 0; i-- {
			if selected[i] {
				p.discard = append(p.discard, p.hand[i])
				p.hand = append(p.hand[:i], p.hand[i+1:]...)
				count++
			}
		}
		if count > 0 {
			game.Report(Event{s:"discard", n:p.n, i:count})
		}
		game.draw(p, count)
	},
	"Chapel": func(game *Game) {
		p := game.NowPlaying()
		selected := pickHand(game, p, 4, false, nil)
		for i := len(p.hand)-1; i >= 0; i-- {
			if selected[i] {
				game.TrashHand(p, i)
			}
		}
	},
	"Chancellor": func(game *Game) {
		p := game.NowPlaying()
		if len(p.deck) > 0 && game.getBool(p) {
			i := len(p.deck)
			p.discard, p.deck = append(p.discard, p.deck...), nil
			game.Report(Event{s:"discarddeck", n:p.n, i:i})
		}
	},
	"Bureaucrat": func(game *Game) {
		p := game.NowPlaying()
		if game.MaybeGain(p, GetCard("Silver")) {
			n := len(p.discard)
			p.deck = append(p.discard[n-1:n], p.deck...)
			p.discard = p.discard[:n-1]
		}
		game.attack(func(other *Player) {
			if !game.inHand(other, isVictory) {
				game.revealHand(other)
				return
			}
			sel := pickHand(game, other, 1, true, func(c *Card) string {
				if !isVictory(c) {
					return "must pick Victory card"
				}
				return ""
			})
			for i := len(other.hand)-1; i >= 0; i-- {
				if sel[i] {
					fmt.Printf("%v decks %v\n", other.name, other.hand[i].name)
					other.deck = append(other.hand[i:i+1], other.deck...)
					other.hand = append(other.hand[:i], other.hand[i+1:]...)
					break
				}
			}
		})
	},
	"Feast": func(game *Game) {
		p := game.NowPlaying()
		game.trash = append(game.trash, p.played[len(p.played)-1])
		p.played = p.played[:len(p.played)-1]
		pickGain(game, 5)
	},
	"Workshop": func(game *Game) { pickGain(game, 4) },
	"Militia": func(game *Game) {
		game.attack(func(other *Player) {
			if len(other.hand) <= 3 {
				return
			}
			sel := pickHand(game, other, 3, true, nil)
			count := 0
			for i := len(other.hand)-1; i >= 0; i-- {
				if !sel[i] {
					other.discard = append(other.discard, other.hand[i])
					other.hand = append(other.hand[:i], other.hand[i+1:]...)
					count++
				}
			}
			if count > 0 {
				game.Report(Event{s:"discard", n:other.n, i:count})
			}
		})
	},
	"Moneylender": func(game *Game) {
		p := game.NowPlaying()
		copper := 0
		if game.isServer {
			for _, c := range p.hand {
				if c.name == "Copper" {
					copper = 1
					break
				}
			}
			game.cast("moneylender", copper)
		} else {
			copper = PanickyAtoi(game.fetch()[0])
		}
		if copper == 1 {
			for i, c := range p.hand {
				if c == nil || c == GetCard("Copper") {
					p.hand[i] = GetCard("Copper")
					game.TrashHand(p, i)
					p.c += 3
					break
				}
			}
		}
	},
	"Remodel": func(game *Game) {
		p := game.NowPlaying()
		if len(p.hand) == 0 {
			return
		}
		sel := pickHand(game, p, 1, true, nil)
		for i, c := range p.hand {
			if sel[i] {
				game.TrashHand(p, i)
				pickGain(game, c.cost + 2)
				return
			}
		}
		panic("unreachable")
	},
	"Spy": func(game *Game) {
		p := game.NowPlaying()
		game.attack(func(other *Player) {
			if !other.MaybeShuffle() {
				return
			}
			c := game.reveal(other)
			if game.getBool(p) {
				other.discard = append(other.discard, c)
				other.deck = other.deck[1:]
				game.Report(Event{s:"discard", n:other.n, i:1})
			}
		})
	},
	"Thief": func(game *Game) {
		p := game.NowPlaying()
		game.attack(func(other *Player) {
			var v []*Card
			found := 0
			trashi := 0
			for i := 0; i < 2 && other.MaybeShuffle(); i++ {
				c := game.reveal(other)
				other.deck = other.deck[1:]
				v = append(v, c)
				if isTreasure(c) {
					trashi = i
					found++
				}
			}
			if found > 1 {
				sel := game.pick(v, p, pickOpts{n:1, exact:true, cond:func(c *Card) string {
					if !isTreasure(c) {
						return "must pick Treasure"
					}
					return ""
				}})
				for i, b := range sel {
					if b {
						trashi = i
					}
				}
			}
			if found > 0 {
				c := v[trashi]
				game.Report(Event{s:"trash", n:other.n, card:c})
				v = append(v[:trashi], v[trashi+1:]...)
				if c.supply > 0 && game.getBool(p) {
					game.Gain(p, c)
				}
			}
			for _, c := range v {
				fmt.Printf("%v discards %v\n", other.name, c.name)
				other.discard = append(other.discard, c)
			}
		})
	},
	"Throne Room": func(game *Game) {
		p := game.NowPlaying()
		if !game.inHand(p, isAction) {
			return
		}
		sel := pickHand(game, p, 1, true, func(c *Card) string {
			if !isAction(c) {
				return "must pick Action"
			}
			return ""
		})
		for i, _ := range p.hand {
			if sel[i] {
				p.a++  // Compensate for Action deducted by MultiPlay().
				game.MultiPlay(p.n, p.hand[i], 2)
				return
			}
		}
	},
	"Council Room": func(game *Game) {
		game.ForOthers(func(other *Player) { game.draw(other, 1) })
	},
	"Library": func(game *Game) {
		p := game.NowPlaying()
		var v []*Card
		for len(p.hand) < 7 && game.draw(p, 1) == 1 {
			mustAsk := false
			if game.isServer {
				c := p.hand[len(p.hand)-1]
				cmd := "done"
				if isAction(c) {
					mustAsk = true
					cmd = "yes"
				}
				game.cast("library", cmd)
			} else {
				mustAsk = game.fetch()[0] == "yes"
			}
			if mustAsk && game.getBool(p) {
				var c *Card
				if game.isServer {
					c = p.hand[len(p.hand)-1]
					game.cast("library2", c)
				} else {
					c = game.keyToCard(game.fetch()[0][0])
				}
				fmt.Printf("%v sets aside %v\n", p.name, c.name)
				p.hand = p.hand[:len(p.hand)-1]
				v = append(v, c)
			}
		}
		p.discard = append(p.discard, v...)
	},
	"Mine": func(game *Game) {
		p := game.NowPlaying()
		if !game.inHand(p, isTreasure) {
			return
		}
		f := func(c *Card) string {
			if !isTreasure(c) {
				return "must pick Treasure"
			}
			return ""
		}
		sel := pickHand(game, p, 1, true, f)
		for i, c := range p.hand {
			if sel[i] {
				game.TrashHand(p ,i)
				choice := pickGainCond(game, c.cost + 3, f)
				fmt.Printf("%v puts %v into hand\n", p.name, choice.name)
				p.hand = append(p.hand, choice)
				p.discard = p.discard[:len(p.discard)-1]
				return
			}
		}
		panic("unreachable")
	},
	"Witch": func(game *Game) {
		game.attack(func(other *Player) { game.MaybeGain(other, GetCard("Curse")) })
	},
	"Adventurer": func(game *Game) {
		p := game.NowPlaying()
		for n := 2; n > 0 && p.MaybeShuffle(); {
			c := game.reveal(p)
			if isTreasure(c) {
				fmt.Printf("%v puts %v in hand\n", p.name, c.name)
				p.hand = append(p.hand, c)
				n--
			} else {
				fmt.Printf("%v discards %v\n", p.name, c.name)
				p.discard = append(p.discard, c)
			}
			p.deck = p.deck[1:]
		}
	},
}
