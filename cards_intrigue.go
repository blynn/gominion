package main

var cardsIntrigue = `
Baron,4,Action,+B1
Bridge,4,Action,+B1,$1
`

var cardsIntrigueAct = map[string]func(game *Game) {
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
}
