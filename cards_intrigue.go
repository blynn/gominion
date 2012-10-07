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
			for _, c := range game.pickHand(p, "1") {
				fmt.Printf("%v decks a card\n", p.name)
				p.deck = append(Pile{c}, p.deck...)
			}
		},
		"Pawn": func(game *Game) {
			game.Choose(game.p, 2, []NameFun{
				{"+1 Card", func() { game.addCards(1) }},
				{"+1 Action", func() { game.a++ }},
				{"+1 Buy", func() { game.b++ }},
				{"+$1", func() { game.c++ }},
			})
		},
		"Secret Chamber": func(game *Game) { game.c += len(game.DiscardList(game.p, game.pickHand(game.p, "*"))) },
		"Masquerade": func(game *Game) {
			m := len(game.players)
			a := make([]*Card, m)
			for i := 0; i < m; i++ {
				j := (game.p.n + i) % m
				p := game.players[j]
				selected := game.pickHand(p, "1")
				if len(selected) > 0 {
					a[j] = selected[0]
				}
			}
			for i := 0; i < m; i++ {
				j := (i + 1) % m
				left := game.players[j]
				left.hand.Add(a[i])
				// TODO: Report the gained card.
			}
			game.TrashList(game.p, game.pickHand(game.p, "1"))
		},
		"Shanty Town": func(game *Game) {
			game.revealHand(game.p)
			if !game.inHand(game.p, (*Card).IsAction) {
				game.addCards(2)
			}
		},
		"Steward": func(game *Game) {
			game.Choose(game.p, 1, []NameFun{
				{"+2 Cards", func() { game.addCards(2) }},
				{"+$2", func() { game.c += 2 }},
				{"trash 2 from hand", func() {
					game.TrashList(game.p, game.pickHand(game.p, "2"))
				}},
			})
		},
		"Swindler": func(game *Game) {
			game.attack(func(other *Player) {
				if !other.MaybeShuffle() {
					return
				}
				c := game.reveal(other)
				other.deck = other.deck[1:]
				game.TrashCard(other, c)
				game.MaybeGain(other, pickCard(game, game.p, CardOpts{cost: game.Cost(c), exact: true}))
			})
		},
		"Wishing Well": func(game *Game) {
			p := game.p
			if !p.MaybeShuffle() {
				return
			}
			c := pickCard(game, p, CardOpts{any: true})
			if c == game.reveal(p) {
				fmt.Printf("%v puts %v in hand\n", p.name, c.name)
				p.hand.Add(c)
				p.deck = p.deck[1:]
			}
		},
		"Baron": func(game *Game) {
			selected := game.pickHand(game.p, "1,card Estate")
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
		"Coppersmith": func(game *Game) { game.data["Coppersmith"] = game.data["Coppersmith"].(int) + 1 },
		"Copper":      func(game *Game) { game.c += game.data["Coppersmith"].(int) },
		"Ironworks": func(game *Game) {
			c := pickGain(game, 4)
			if c.IsAction() {
				game.a++
			}
			if c.IsTreasure() {
				game.c++
			}
			if c.IsVictory() {
				game.addCards(1)
			}
		},
		"Mining Village": func(game *Game) {
			if !game.WillTrashMe() && game.getBool(game.p, "trash for $2?") {
				game.SetTrashMe()
				game.c += 2
			}
		},
		"Scout": func(game *Game) {
			p := game.p
			var v Pile
			for n := 0; n < 4 && p.MaybeShuffle(); n++ {
				c := game.reveal(p)
				if c.IsVictory() {
					fmt.Printf("%v puts %v in hand\n", p.name, c.name)
					p.hand.Add(c)
				} else {
					v.Add(c)
				}
				p.deck = p.deck[1:]
			}
			for len(v) > 0 {
				var selected Pile
				selected, v = game.split(v, p, "1")
				fmt.Printf("%v decks %v\n", p.name, selected[0].name)
				p.deck = append(selected, p.deck...)
			}
		},
		"Minion": func(game *Game) {
			p := game.p
			game.Choose(p, 1, []NameFun{
				{"+$2", func() { game.c += 2 }},
				{"discard hand, +4 Cards", func() {
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
				}},
			})
		},
		"Saboteur": func(game *Game) {
			game.attack(func(other *Player) {
				var v Pile
				var c *Card
				for other.MaybeShuffle() {
					c = game.reveal(other)
					other.deck = other.deck[1:]
					if game.Cost(c) >= 3 {
						break
					}
					v.Add(c)
				}
				if c != nil {
					game.TrashCard(other, c)
					game.MaybeGain(other, pickCard(game, other, CardOpts{cost: game.Cost(c) - 2, optional: true}))
				}
				if len(v) > 0 {
					game.DiscardList(other, v)
				}
			})
		},
		"Torturer": func(game *Game) {
			game.attack(func(other *Player) {
				game.Choose(other, 1, []NameFun{
					{"discard 2", func() { game.DiscardList(other, game.pickHand(other, "2")) }},
					{"gain Curse in hand", func() { game.MaybeGain(other, GetCard("Curse")) }},
				})
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
				if c.IsAction() {
					game.a += 2
				}
				if c.IsTreasure() {
					game.c += 2
				}
				if c.IsVictory() {
					game.addCards(2)
				}
				prev = c
			}
		},
		"Trading Post": func(game *Game) {
			p := game.p
			selected := game.pickHand(p, "2")
			game.TrashList(p, selected)
			if len(selected) == 2 && game.MaybeGain(p, GetCard("Silver")) {
				n := len(p.discard)
				p.hand.Add(p.discard[n-1])
				p.discard = p.discard[:n-1]
			}
		},
		"Upgrade": func(game *Game) {
			p := game.p
			selected := game.pickHand(p, "1")
			if len(selected) == 0 {
				return
			}
			game.TrashCard(p, selected[0])
			game.MaybeGain(p, pickCard(game, p, CardOpts{cost: game.Cost(selected[0]) + 1, exact: true}))
		},
		"Nobles": func(game *Game) {
			game.Choose(game.p, 1, []NameFun{
				{"+3 Cards", func() { game.addCards(3) }},
				{"+2 Actions", func() { game.a += 2 }},
			})
		},
	},
	VP: map[string]func(game *Game) int{
		"Duke": func(game *Game) int {
			n := 0
			for _, c := range game.p.manifest {
				if c == GetCard("Duchy") {
					n++
				}
			}
			return n
		},
	},
	React: map[string]func(*Game, *Player){
		"Secret Chamber": func(game *Game, p *Player) {
			game.draw(p, 2)
			selected := game.pickHand(p, "2")
			fmt.Printf("%v decks %v cards\n", p.name, len(selected))
			p.deck = append(selected, p.deck...)
		},
	},
	Presets: `
Victory Dance:Bridge,Duke,Great Hall,Harem,Ironworks,Masquerade,Nobles,Pawn,Scout,Upgrade
Secret Schemes:Conspirator,Harem,Ironworks,Pawn,Saboteur,Shanty Town,Steward,Swindler,Trading Post,Tribute
Best Wishes:Coppersmith,Courtyard,Masquerade,Scout,Shanty Town,Steward,Torturer,Trading Post,Upgrade,Wishing Well

Deconstruction:Bridge,Mining Village,Remodel,Saboteur,Secret Chamber,Spy,Swindler,Thief,Throne Room,Torturer
Hand Madness:Bureaucrat,Chancellor,Council Room,Courtyard,Mine,Militia,Minion,Nobles,Steward,Torturer
Underlings:Baron,Cellar,Festival,Library,Masquerade,Minion,Nobles,Pawn,Steward,Witch
`,
	Setup: func() { HookTurn(func(game *Game) { game.data["Coppersmith"] = 0 }) },
}

type NameFun struct {
	name string
	fun  func()
}

func (game *Game) Choose(p *Player, n int, nfs []NameFun) {
	for i, nf := range nfs {
		fmt.Printf("[%d] %v\n", i+1, nf.name)
	}
	var selection []int
	game.SetParse(fmt.Sprintf("Choose %v:", n), func(b byte) (Command, string) {
		if b < '1' || b > '0'+byte(len(nfs)) {
			i := int(b - '1')
			for _, x := range selection {
				if x == i {
					return errCmd, "already chosen " + string(b)
				}
			}
			return errCmd, "enter digit within range"
		}
		return Command{s: string(b)}, ""
	})
	for len(selection) < n {
		selection = append(selection, int(game.getCommand(p).s[0]-'1'))
	}
	for _, v := range selection {
		nfs[v].fun()
	}
}
