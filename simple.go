package main

type SimpleBuyer struct {
	list []string
}

func (this SimpleBuyer) start(game *Game, p *Player) {
	for {
		<-p.trigger
		if frame := game.StackTop(); frame != nil {
			switch frame.card.name {
			case "Bureaucrat":
				for _, c := range p.hand {
					if isVictory(c) {
						game.ch <- Command{s: "pick", c: c}
						break
					}
				}
			case "Militia":
				// Keep first 3 cards.
				for i := 0; i < 3; i++ {
					game.ch <- Command{s: "pick", c: p.hand[i]}
					<-p.trigger
				}
			case "Masquerade":
				// Throw away first card.
				game.ch <- Command{s: "pick", c: p.hand[0]}
				continue
			case "Saboteur":
				// Pick nothing.
				game.ch <- Command{s: "pick"}
				continue
			case "Torturer":
				// Choose to gain a Curse.
				game.ch <- Command{s: "2"}
				continue
			default:
				panic("AI unimplemented: " + frame.card.name)
			}
			continue
		}
		game.ch <- func() Command {
			if game.phase == phAction {
				return Command{s: "next"}
			}
			if game.phase != phBuy {
				panic("unreachable")
			}
			for k := len(p.hand) - 1; k >= 0; k-- {
				if isTreasure(p.hand[k]) {
					return Command{s: "play", c: p.hand[k]}
				}
			}
			for _, s := range this.list {
				c := GetCard(s)
				if game.c >= game.Cost(c) && c.supply > 0 {
					return Command{s: "buy", c: c}
				}
			}
			return Command{s: "next"}
		}()
	}
}
