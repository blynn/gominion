package main

import (
	"fmt"
	"regexp"
	"strings"
	"testing"
)

// Utilities for tests.

func ParsePile(s string) Pile {
	var pile Pile
	for _, s := range strings.Split(s, ",") {
		if s == "" {
			continue
		}
		pile = append(pile, GetCard(s))
	}
	return pile
}

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
		if msg := pileFun(pp, ParsePile(v[1])); msg != "" {
			t.Errorf(fmt.Sprintf("%v %v: %v", p.name, v[0], msg))
		}
	}
}

func ComparePiles(got, want Pile) string {
	i := 0
	for _, c := range want {
		if i >= len(got) {
			return fmt.Sprintf("too many cards given")
		}
		if got[i] != c {
			return fmt.Sprintf("want %q, got %q", c.name, got[i].name)
		}
		i++
	}
	if i < len(got) {
		return fmt.Sprintf("too few cards given")
	}
	return ""
}

func CheckPiles(t *testing.T, players []*Player, lines string) {
	parse(t, func(name string) *Player {
		for _, p := range players {
			if p.name == name {
				return p
			}
		}
		panic("no such player: " + name)
	}, func(pp *Pile, pile Pile) string { return ComparePiles(*pp, pile) },
		lines)
}

func Setup(t *testing.T, lines string) []*Player {
	var players []*Player
	parse(t, func(name string) *Player {
		p := &Player{name: name, n: len(players), trigger: make(chan bool)}
		players = append(players, p)
		return p
	}, func(pp *Pile, pile Pile) string {
		*pp = append(*pp, pile...)
		return ""
	}, lines)
	return players
}
