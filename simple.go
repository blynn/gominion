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
					if c.IsVictory() {
						game.ch <- Command{s: "pick", c: c}
						break
					}
				}
			case "Militia":
				// Keep first 3 cards.
				for i := 0; i < 3; i++ {
					// TODO: Support picks with multiple cards.
					if i > 0 {
						<-p.trigger
					}
					game.ch <- Command{s: "pick", c: p.hand[i]}
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
				if p.hand[k].IsTreasure() {
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
