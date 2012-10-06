package main

import (
	"regexp"
	"strings"
	"testing"
)

func TestBase(t *testing.T) {
	var players []*Player
	var p *Player
	for _, line := range strings.Split(`
= Alice =
hand:Bureaucrat
deck:Copper
= Bob =
hand:Province,Duchy,Estate
deck:Copper
discard:Moat
= Carol =
hand:Silver,Copper,Witch
deck:Copper
discard:Gold
= Dave =
hand:Copper,Estate,Estate,Estate
= Eve =
hand:Moat
`, "\n") {
		if line == "" {
			continue
		}
		if v := regexp.MustCompile("= (.*) =").FindStringSubmatch(line); v != nil {
			p = &Player{name: v[1], n: len(players), trigger: make(chan bool)}
			players = append(players, p)
			continue
		}
		v := strings.Split(line, ":")
		var pp *Pile
		switch v[0] {
		case "discard":
			pp = &p.discard
		case "deck":
			pp = &p.deck
		case "hand":
			pp = &p.hand
		}
		if pp == nil {
			panic("bad Pile: " + v[0])
		}
		for _, s := range strings.Split(v[1], ",") {
			*pp = append(*pp, GetCard(s))
		}
	}
	game := &Game{ch: make(chan Command), isServer: true,
		sendCmd:    func(game *Game, p *Player, cmd *Command) {},
		GetDiscard: func(game *Game, p *Player) string { return p.discard[len(p.discard)-1].name },
		players:    players,
	}
	game.a, game.b, game.c = 1, 1, 0
	game.discount = 0
	game.copperbonus = 0
	game.aCount = 0
	game.phase = phAction
	game.p = players[0]
	GetCard("Silver").supply = 8
	done := make(chan bool)
	go func() {
		// Bob chooses to deck a Duchy.
		<-players[1].trigger
		game.ch <- Command{s: "pick", c: GetCard("Duchy")}
		// Carol: no Victory cards, so she reveals hand.
		// Dave: choice is forced (Estate).
		// Eve reveals Moat to stop attack.
		<-players[4].trigger
		game.ch <- Command{s: "pick", c: GetCard("Moat")}
		<-players[4].trigger
		game.ch <- Command{s: "done"}
		done <- true
	}()
	game.Play(GetCard("Bureaucrat"))
	<-done
	for _, line := range strings.Split(`
= Alice =
played:Bureaucrat
deck:Silver,Copper
= Bob =
hand:Province,Estate
deck:Duchy,Copper
discard:Moat
= Carol =
hand:Silver,Copper,Witch
deck:Copper
discard:Gold
= Dave =
hand:Copper,Estate,Estate
deck:Estate
= Eve =
hand:Moat
`, "\n") {
		if line == "" {
			continue
		}
		if v := regexp.MustCompile("= (.*) =").FindStringSubmatch(line); v != nil {
			p = nil
			for _, x := range players {
				if x.name == v[1] {
					p = x
					break
				}
			}
			if p == nil {
				panic("no such player: " + v[1])
			}
			continue
		}
		v := strings.Split(line, ":")
		var pp *Pile
		switch v[0] {
		case "played":
			pp = &p.played
		case "discard":
			pp = &p.discard
		case "deck":
			pp = &p.deck
		case "hand":
			pp = &p.hand
		}
		if pp == nil {
			panic("bad Pile: " + v[0])
		}
		i := 0
		for _, s := range strings.Split(v[1], ",") {
			if (*pp)[i] != GetCard(s) {
				t.Fatalf("%v: %v: want %q, got %q", p.name, v[0], s, (*pp)[i].name)
			}
			i++
		}
	}
}
