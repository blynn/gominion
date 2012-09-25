package main

import (
	"fmt"
)

var cardsBase = CardDB{
	List: `
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
`,
	Fun: map[string]func(game *Game){
		"Cellar": func(game *Game) {
			p := game.NowPlaying()
			var selected Pile
			selected, p.hand = game.split(p.hand, p, pickOpts{n: len(p.hand)})
			if len(selected) > 0 {
				game.DiscardList(p, selected)
				game.draw(p, len(selected))
			}
		},
		"Chapel": func(game *Game) {
			p := game.NowPlaying()
			var selected Pile
			selected, p.hand = game.split(p.hand, p, pickOpts{n: 4})
			game.TrashList(p, selected)
		},
		"Chancellor": func(game *Game) {
			p := game.NowPlaying()
			if len(p.deck) > 0 && game.getBool(p) {
				p.discard, p.deck = append(p.discard, p.deck...), nil
				game.Report(Event{s: "discarddeck", n: p.n, i: len(p.deck)})
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
				var selected Pile
				selected, other.hand = game.split(other.hand, other, pickOpts{n: 1, exact: true, cond: func(c *Card) string {
					if !isVictory(c) {
						return "must pick Victory card"
					}
					return ""
				}})
				if len(selected) == 0 {
					game.revealHand(other)
					return
				}
				fmt.Printf("%v decks %v\n", other.name, selected[0].name)
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
				var lost Pile
				other.hand, lost = game.split(other.hand, other, pickOpts{n: 3, exact: true})
				game.DiscardList(other, lost)
			})
		},
		"Moneylender": func(game *Game) {
			p := game.NowPlaying()
			cardCopper := GetCard("Copper")
			copper := 0
			if game.isServer {
				for _, c := range p.hand {
					if c == cardCopper {
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
					if c == nil || c == cardCopper {
						p.hand = append(p.hand[:i], p.hand[i+1:]...)
						game.TrashCard(p, cardCopper)
						game.c += 3
						break
					}
				}
			}
		},
		"Remodel": func(game *Game) {
			p := game.NowPlaying()
			var selected Pile
			selected, p.hand = game.split(p.hand, p, pickOpts{n: 1, exact: true})
			if len(selected) > 0 {
				game.TrashCard(p, selected[0])
				pickGain(game, game.Cost(selected[0])+2)
			}
		},
		"Spy": func(game *Game) {
			p := game.NowPlaying()
			game.attack(func(other *Player) {
				if !other.MaybeShuffle() {
					return
				}
				c := game.reveal(other)
				if game.getBool(p) {
					game.DiscardList(other, Pile{c})
					other.deck = other.deck[1:]
				}
			})
		},
		"Thief": func(game *Game) {
			p := game.NowPlaying()
			game.attack(func(other *Player) {
				var loot, junk Pile
				for i := 0; i < 2 && other.MaybeShuffle(); i++ {
					c := game.reveal(other)
					other.deck = other.deck[1:]
					if isTreasure(c) {
						loot = append(loot, c)
					} else {
						junk = append(junk, c)
					}
				}
				if len(loot) > 1 {
					var rest Pile
					loot, rest = game.split(loot, p, pickOpts{n: 1, exact: true, cond: func(c *Card) string {
						if !isTreasure(c) {
							return "must pick Treasure"
						}
						return ""
					}})
					junk = append(junk, rest...)
				}
				if len(loot) > 0 {
					c := loot[0]
					game.TrashCard(other, c)
					if c.supply > 0 && game.getBool(p) {
						game.Gain(p, c)
					}
				}
				game.DiscardList(other, junk)
			})
		},
		"Throne Room": func(game *Game) {
			p := game.NowPlaying()
			var selected Pile
			selected, p.hand = game.split(p.hand, p, pickOpts{n: 1, exact: true, cond: func(c *Card) string {
				if !isAction(c) {
					return "must pick Action"
				}
				return ""
			}})
			if len(selected) > 0 {
				game.MultiPlay(p, selected[0], 2)
			}
		},
		"Council Room": func(game *Game) {
			game.ForOthers(func(other *Player) { game.draw(other, 1) })
		},
		"Library": func(game *Game) {
			p := game.NowPlaying()
			var v Pile
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
			var selected Pile
			f := func(c *Card) string {
				if !isTreasure(c) {
					return "must pick Treasure"
				}
				return ""
			}
			selected, p.hand = game.split(p.hand, p, pickOpts{n: 1, exact: true, cond: f})
			if len(selected) == 0 {
				return
			}
			game.TrashCard(p, selected[0])
			choice := pickGainCond(game, game.Cost(selected[0])+3, f)
			fmt.Printf("%v puts %v into hand\n", p.name, choice.name)
			p.hand = append(p.hand, choice)
			p.discard = p.discard[:len(p.discard)-1]
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
					game.DiscardList(p, Pile{c})
				}
				p.deck = p.deck[1:]
			}
		},
	},
	VP: map[string]func(game *Game) int{
		"Gardens": func(game *Game) int { return len(game.NowPlaying().manifest) / 10 },
	},
}
