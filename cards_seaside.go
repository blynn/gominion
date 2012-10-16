package main

import (
	"fmt"
)

func (game *Game) addDuration(fun func()) {
	key := "Duration/" + game.p.name
	var list []func()
	if v, ok := game.data[key].([]func()); ok {
		list = v
	}
	game.data[key] = append(list, fun)
}
func (game *Game) peek(c *Card) *Card {
	p := game.p
	if game.isServer {
		game.castCond(func(x *Player) bool { return x == p }, "peek", c)
		game.castCond(func(x *Player) bool { return x != p }, "peek", "?")
		return c
	}
	key := game.fetch()[0][0]
	if key != '?' {
		return game.keyToCard(key)
	}
	return nil
}

var cardsSeaside = CardDB{
	List: `
Embargo,2,Action,$2
Haven,2,Action-Duration,+C1,+A1
Lighthouse,2,Action-Duration,+A1,$1
Native Village,2,Action,+A2
Pearl Diver,2,Action,+C1,+A1
Fishing Village,3,Action-Duration,+A2,$1
Lookout,3,Action,+A1
Smugglers,3,Action
Warehouse,3,Action,+C3,+A1
Caravan,4,Action-Duration,+C1,+A1
Cutpurse,4,Action-Attack,$2
Island,4,Action-Victory,#2
Navigator,4,Action,$2
Pirate Ship,4,Action
Salvager,4,Action,+B1
Sea Hag,4,Action-Attack
Treasure Map,4,Action
Bazaar,5,Action,+C1,+A2,$1
Explorer,5,Action
Ghost Ship,5,Action-Attack,+C2
Merchant Ship,5,Action-Duration,$2
Tactician,5,Action-Duration
Treasury,5,Action,+C1,+A1,$1
Wharf,5,Action-Duration,+C2,+B1
`,
	Fun: map[string]func(game *Game){
		"Embargo": func(game *Game) {
			game.SetTrashMe()
			// TODO: Must pick card in Supply.
			c := pickCard(game, game.p, CardOpts{any: true})
			m := game.data["Embargo"].(map[*Card]int)
			m[c]++
		},
		"Haven": func(game *Game) {
			p := game.p
			selected := game.pickHand(p, "1")
			if len(selected) == 0 {
				return
			}
			game.addDuration(func() { p.hand.Add(selected...) })
		},
		"Lighthouse": func(game *Game) {
			key := "Lighthouse/" + game.p.name
			game.data[key] = true
			game.addDuration(func() {
				game.addCoins(1)
				delete(game.data, key)
			})
		},
		"Native Village": func(game *Game) {
			p := game.p
			key := "Native Village/" + p.name
			game.Choose(p, 1, []NameFun{
				{"Set aside top card to Native Village", func() {
					if !p.MaybeShuffle() {
						return
					}
					c := p.deck[0]
					p.deck = p.deck[1:]
					if _, ok := game.data[key]; !ok {
						game.data[key] = make(Pile, 0)
					}
					mat := game.data[key].(Pile)
					game.data[key] = append(mat, c)
				}},
				{"Put all Native Village cards into hand", func() {
					if _, ok := game.data[key]; !ok {
						return
					}
					p.hand.Add(game.data[key].(Pile)...)
					delete(game.data, key)
				}},
			})
		},
		"Pearl Diver": func(game *Game) {
			p := game.p
			if !p.MaybeShuffle() {
				return
			}
			c := game.peek(p.deck[len(p.deck)-1])
			if c == nil {
				fmt.Printf("%v looks at bottom card\n", p.name)
			} else {
				fmt.Printf("%v looks at %v\n", p.name, c.name)
			}
			if game.getBool(game.p, "move to top?") {
				p.deck = append(Pile{c}, p.deck[:len(p.deck)-1]...)
			}
		},
		"Fishing Village": func(game *Game) {
			game.addDuration(func() {
				game.addActions(1)
				game.addCoins(1)
			})
		},
		"Lookout": func(game *Game) {
			p := game.p
			var v Pile
			for i := 0; i < 3; i++ {
				if !p.MaybeShuffle() {
					break
				}
				c := game.peek(p.deck[0])
				if c == nil {
					fmt.Printf("%v looks at card #%v\n", p.name, i+1)
				} else {
					fmt.Printf("%v looks at [%c] %v\n", p.name, c.key, c.name)
				}
				v.Add(c)
				p.deck = p.deck[1:]
			}
			for i := 0; i < 3; i++ {
				if len(v) == 0 {
					break
				}
				var selected Pile
				selected, v = game.split(v, p, "1")
				switch i {
				case 0:
					game.TrashList(p, selected)
				case 1:
					game.DiscardList(p, selected)
				case 2:
					p.deck = append(selected, p.deck...)
				}
			}
		},
		"Smugglers": func(game *Game) {
			key := "Smugglers/" + game.RightOf(game.p).name
			if x, ok := game.data[key]; ok {
				p := game.p
				for _, c := range x.(Pile) {
					fmt.Printf("[%c] %v\n", c.key, c.name)
				}
				selected, _ := game.split(x.(Pile), p, "1")
				for _, c := range selected {
					game.MaybeGain(p, c)
				}
			}
		},
		"Warehouse": func(game *Game) { game.DiscardList(game.p, game.pickHand(game.p, "3")) },
		"Caravan":   func(game *Game) { game.addDuration(func() { game.addCards(1) }) },
		"Cutpurse": func(game *Game) {
			game.attack(func(other *Player) {
				selected := game.pickHand(other, "1,card Copper")
				if len(selected) == 0 {
					game.revealHand(other)
					return
				}
				game.DiscardList(other, selected)
			})
		},
		"Island": func(game *Game) {
			p := game.p
			key := "Island/" + p.name
			if _, ok := game.data[key]; !ok {
				game.data[key] = make(Pile, 0)
			}
			aside := game.data[key].(Pile)
			frame := game.StackTop()
			frame.popHook = func() { aside.Add(frame.card) }
			selected := game.pickHand(p, "1")
			if len(selected) > 0 {
				fmt.Printf("%v sets aside %v\n", p.name, selected[0].name)
				aside.Add(selected[0])
			}
			game.data[key] = aside
		},
		"Navigator": func(game *Game) {
			p := game.p
			var v Pile
			for i := 0; i < 5; i++ {
				if !p.MaybeShuffle() {
					break
				}
				c := game.peek(p.deck[0])
				if c == nil {
					fmt.Printf("%v looks at #%v\n", p.name, i+1)
				} else {
					fmt.Printf("%v looks at [%c] %v\n", p.name, c.key, c.name)
				}
				v.Add(c)
				p.deck = p.deck[1:]
			}
			if len(v) == 0 {
				return
			}
			if game.getBool(game.p, "discard?") {
				game.DiscardList(p, v)
				return
			}
			var perm Pile
			for len(v) > 0 {
				var selected Pile
				selected, v = game.split(v, p, "1")
				perm.Add(selected...)
			}
			p.deck = append(perm, p.deck...)
		},
		"Pirate Ship": func(game *Game) {
			key := "Pirate Ship/" + game.p.name
			var n int
			if x, ok := game.data[key]; ok {
				n = x.(int)
			}
			game.Choose(game.p, 1, []NameFun{
				{"Arr! Loot others!", func() {
					var loot, junk Pile
					var found bool
					game.attack(func(other *Player) {
						for i := 0; i < 2 && other.MaybeShuffle(); i++ {
							c := game.reveal(other)
							other.deck = other.deck[1:]
							if c.IsTreasure() {
								loot.Add(c)
							} else {
								junk.Add(c)
							}
						}
						if len(loot) > 0 {
							found = true
							var rest Pile
							loot, rest = game.split(loot, game.p, "1,kind Treasure")
							junk.Add(rest...)
							game.TrashList(other, loot)
							game.DiscardList(other, junk)
						}
					})
					if found {
						game.data[key] = n + 1
					}
				}},
				{fmt.Sprintf("+%v Coins", n), func() { game.addCoins(n) }},
			})
		},
		"Salvager": func(game *Game) {
			p := game.p
			selected := game.pickHand(p, "1")
			if len(selected) == 0 {
				return
			}
			game.TrashList(game.p, selected)
			pickGain(game, game.Cost(selected[0]))
		},
		"Sea Hag": func(game *Game) {
			game.attack(func(other *Player) {
				if other.MaybeShuffle() {
					game.DiscardList(other, other.deck[:1])
					other.deck = other.deck[1:]
				}
				game.MaybeDeckGain(other, GetCard("Curse"))
			})
		},
		"Treasure Map": func(game *Game) {
			p := game.p
			game.SetTrashMe()
			selected := game.pickHand(p, "1,card Treasure Map")
			if len(selected) == 0 {
				return
			}
			game.TrashList(p, selected)
			for i := 0; i < 4 && game.MaybeDeckGain(p, GetCard("Gold")); i++ {
			}
		},
		"Explorer": func(game *Game) {
			p := game.p
			selected, _ := game.split(p.hand, p, "1-,card Province")
			var c *Card
			if len(selected) == 0 {
				c = GetCard("Silver")
			} else {
				c = GetCard("Gold")
			}
			if game.MaybeGain(p, c) {
				p.hand.Add(c)
				p.discard = p.discard[:len(p.discard)-1]
			}
		},
		"Ghost Ship": func(game *Game) {
			game.attack(func(other *Player) {
				if len(other.hand) <= 3 {
					return
				}
				var selected Pile
				other.hand, selected = game.split(other.hand, other, "3")
				other.deck = append(selected, other.deck...)
			})
		},
		"Merchant Ship": func(game *Game) { game.addDuration(func() { game.addCoins(2) }) },
		"Tactician": func(game *Game) {
			p := game.p
			if len(p.hand) == 0 {
				return
			}
			game.DiscardList(p, p.hand)
			p.hand = nil
			game.addDuration(func() {
				game.addCards(5)
				game.addBuys(1)
				game.addActions(1)
			})
		},
		"Wharf": func(game *Game) {
			game.addDuration(func() {
				game.addCards(2)
				game.addBuys(1)
			})
		},
	},
	Setup: func() {
		HookNewGame(func(game *Game) { game.data["Embargo"] = make(map[*Card]int) })
		HookBuy(func(game *Game, c *Card) {
			m := game.data["Embargo"].(map[*Card]int)
			if n, ok := m[c]; ok {
				for i := 0; i < n; i++ {
					if !game.MaybeGain(game.p, GetCard("Curse")) {
						break
					}
				}
			}
			if c.IsVictory() {
				game.data["Treasury"] = true
			}
		})
		HookTurn(func(game *Game) {
			key := "Duration/" + game.p.name
			if v, ok := game.data[key].([]func()); ok {
				for _, f := range v {
					f()
				}
			}
			delete(game.data, key)

			delete(game.data, "Treasury")

			delete(game.data, "Smugglers/" + game.p.name)
		})
		HookClean(func(game *Game, c *Card) {
			if c == GetCard("Treasury") {
				if _, ok := game.data["Treasury"]; !ok {
					p := game.p
					frame := &Frame{card: c}
					game.stack = append(game.stack, frame)
					if game.getBool(p, "deck Treasury?") {
						p.deck = append(Pile{c}, p.deck...)
						p.discard = p.discard[:len(p.discard)-1]
					}
					game.stack = game.stack[:len(game.stack)-1]
				}
			}
		})
		HookAttack(func(game *Game) {
			key := "Lighthouse/" + game.p.name
			if _, ok := game.data[key].([]func()); ok {
				fmt.Printf("%v: Lighthouse stops attack", game.p.name)
				game.noAttack = true
			}
		})
		HookGain(func(game *Game, c *Card) {
			key := "Smugglers/" + game.p.name
			var p Pile
			if x, ok := game.data[key]; ok {
				p = x.(Pile)
			}
			if game.Cost(c) <= 6 {
				p.Add(c)
			}
			game.data[key] = p
		})
	},
	Presets: `
Test:Pearl Diver,Lookout,Navigator,Treasure Map,Pirate Ship,Treasury,Smugglers,Village,Woodcutter,Workshop
`,
}
