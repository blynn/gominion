package main

import (
	"fmt"
	"regexp"
	"strings"
	"testing"
)

func parse(t *testing.T, playerFun func(string) *Player, pileFun func(*Pile, Pile) string, lines string) {
	var p *Player
	for _, line := range strings.Split(lines, "\n") {
		if line == "" {
			continue
		}
		if v := regexp.MustCompile("= (.*) =").FindStringSubmatch(line); v != nil {
			p = playerFun(v[1])
			continue
		}
		if p == nil {
			panic("no player name yet")
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
		var pile Pile
		for _, s := range strings.Split(v[1], ",") {
			if s == "" {
				continue
			}
			pile = append(pile, GetCard(s))
		}
		if msg := pileFun(pp, pile); msg != "" {
			t.Fatalf(fmt.Sprintf("%v %v: %v", p.name, v[0], msg))
		}
	}
}

func CheckPiles(t *testing.T, players []*Player, lines string) {
	parse(t, func(name string) *Player {
		for _, p := range players {
			if p.name == name {
				return p
			}
		}
		panic("no such player: " + name)
	}, func(pp *Pile, pile Pile) string {
		i := 0
		for _, c := range pile {
			if i >= len(*pp) {
				return fmt.Sprintf("too many cards given")
			}
			if (*pp)[i] != c {
				return fmt.Sprintf("want %q, got %q", (*pp)[i].name, c.name)
			}
			i++
		}
		if i < len(*pp) {
			return fmt.Sprintf("too few cards given")
		}
		return ""
	}, lines)
}

func TestBase(t *testing.T) {
	var players []*Player
	parse(t, func(name string) *Player {
		p := &Player{name: name, n: len(players), trigger: make(chan bool)}
		players = append(players, p)
		return p
	}, func(pp *Pile, pile Pile) string {
		*pp = append(*pp, pile...)
		return ""
	}, `
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
`)
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
	done := make(chan bool)
	// Alice plays Bureaucrat.
	GetCard("Silver").supply = 8
	game.p = players[0]
	go func() {
		// Bob chooses to deck a Duchy.
		<-players[1].trigger
		game.ch <- Command{s: "pick", c: GetCard("Duchy")}
		// Carol: no Victory cards, so she reveals hand.
		// Dave: choice is forced (Estate).
		// Eve reveals Moat to stop attack.
		<-players[4].trigger
		game.ch <- Command{s: "pick", c: GetCard("Moat")}
		// One can reveal the same Reaction multiple times.
		<-players[4].trigger
		game.ch <- Command{s: "pick", c: GetCard("Moat")}
		<-players[4].trigger
		game.ch <- Command{s: "done"}
		done <- true
	}()
	game.Play(GetCard("Bureaucrat"))
	<-done
	CheckPiles(t, players, `
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
`)
	// Carol plays Witch.
	GetCard("Curse").supply = 3
	game.p = players[2]
	go func() {
		// Eve abstains from revealing Moat(!)
		<-players[4].trigger
		game.ch <- Command{s: "done"}
		done <- true
	}()
	game.Play(GetCard("Witch"))
	<-done
	// Curses are given starting from Carol's left, until they run out.
	CheckPiles(t, players, `
= Alice =
played:Bureaucrat
deck:Silver,Copper
discard:Curse
= Bob =
hand:Province,Estate
deck:Duchy,Copper
discard:Moat
= Carol =
hand:Silver,Copper,Copper,Gold
played:Witch
deck:
discard:
= Dave =
hand:Copper,Estate,Estate
deck:Estate
discard:Curse
= Eve =
hand:Moat
discard:Curse
`)
}
