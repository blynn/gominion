package main

import (
	"fmt"
	"io"
	"math/rand"
	"strconv"
	"strings"
)

type Kind struct {
	name string
}

type Card struct {
	key byte
	name string
	cost int
	kind []*Kind
	coin int
	n int
}

func (c *Card) HasKind(k *Kind) bool {
	for _, v := range c.kind {
		if v == k {
			return true
		}
	}
	return false
}

type Pile []*Card

var (
	KindDict = make(map[string]*Kind)
	CardDict = make(map[string]*Card)
	supply = make(map[*Card]int)
	suplist Pile
)

func keyToCard(key byte) *Card {
	for _, c := range suplist {
		if key == c.key {
			return c
		}
	}
	return nil
}

func (deck Pile) shuffle() {
	if len(deck) < 1 {
		return
	}
	n := rand.Intn(len(deck))
	deck[0], deck[n] = deck[n], deck[0]
	deck[1:].shuffle()
}

func (deck *Pile) Add(s string) {
	c, ok := CardDict[s]
	if !ok {
		panic("no such card: " + s)
	}
	*deck = append(*deck, c)
}

type Player struct {
	a, b, c int
}

func getKind(s string) *Kind {
	k, ok := KindDict[s]
	if !ok {
		panic("no such kind: " + s)
	}
	return k
}

func main() {
	initKind := func(s string) {
		KindDict[s] = &Kind{s}
	}
	for _, s := range []string{"Treasure", "Victory", "Curse"} {
		initKind(s)
	}
	kTreasure := getKind("Treasure")

	initCard := func(s string) {
		a := strings.Split(s, ";")
		if len(a) < 3 {
			panic(s)
		}
		if _, ok := CardDict[a[0]]; ok {
			panic(s)
		}
		cost, err := strconv.Atoi(a[1]);
		if err != nil {
			panic(s)
		}
		c := &Card{name:a[0], cost:cost}
		for _, s := range strings.Split(a[2], ",") {
			kind, ok := KindDict[s]
			if !ok {
				panic("no such kind: " + a[2])
			}
			c.kind = append(c.kind, kind)
		}
		CardDict[c.name] = c
		for i := 3; i < len(a); i++ {
			s := a[i]
			switch s[0] {
			case '$':
				n, err := strconv.Atoi(s[1:])
				if err != nil {
					panic(s)
				}
				c.n = n
			}
		}
	}
	db := `
Copper;0;Treasure;$1
Silver;3;Treasure;$2
Gold;6;Treasure;$3
Estate;2;Victory;V1
Duchy;5;Victory;V3
Province;8;Victory;V6
Curse;0;Curse;C1
`
	for _, line := range strings.Split(db, "\n") {
		if len(line) > 0 {
			initCard(line)
		}
	}
	fmt.Println("Gominion")
	setSupply := func(s string, n int) {
		c, ok := CardDict[s]
		if !ok {
			panic("no such card: " + s)
		}
		supply[c] = n
	}
	setSupply("Copper", 60 - 7)
	setSupply("Silver", 40)
	setSupply("Gold", 30)
	// 8, 12, 12
	vpcount := 8
	for _, s := range []string{"Estate", "Duchy", "Province"} {
		setSupply(s, vpcount)
	}
	// 10, 20, 30
	setSupply("Curse", 10)
	var deck, hand, inplay, discard Pile
	for i := 0; i < 3; i++ {
		deck.Add("Estate")
	}
	for i := 0; i < 7; i++ {
		deck.Add("Copper")
	}
	rand.Seed(1234)
	deck.shuffle()
	fmt.Println("")

	layout := func(s string, key byte) {
		c, ok := CardDict[s]
		if !ok {
			panic("unknown card: " + s)
		}
		suplist = append(suplist, c)
                c.key = key
	}
	layout("Copper", 'q')
	layout("Silver", 'w')
	layout("Gold", 'e')
	layout("Estate", 'a')
	layout("Duchy", 's')
	layout("Province", 'd')
	layout("Curse", '!')
	var draw func(n int)
	draw = func(n int) {
		if n == 0 {
			return
		}
		if len(deck) == 0 {
			if len(discard) == 0 {
				return
			}
			deck, discard = discard, deck
			deck.shuffle()
		}
		hand, deck = append(hand, deck[0]), deck[1:]
		draw(n-1)
	}
	player := Player{a:1, b:1, c:0}
	draw(5)
	for {
		if player.b == 0 {
			fmt.Printf("*** end of turn ***\n")
			discard, inplay = append(discard, inplay...), nil
			discard, hand = append(discard, hand...), nil
			player = Player{a:1, b:1, c:0}
			draw(5)
		}
		for _, c := range suplist {
			fmt.Printf("[%c] %v(%v) $%v\n", c.key, c.name, supply[c], c.cost)
		}
		fmt.Printf("Deck size: %v\n", len(deck))
		fmt.Printf("Discard size: %v ", len(discard))
		if len(discard) > 0 {
			fmt.Printf("Top: %v\n", discard[len(discard)-1].name)
		} else {
			fmt.Printf("(empty)\n")
		}
		fmt.Println("Hand:")
		for _, c := range hand {
			fmt.Printf("[%c] %v\n", c.key, c.name)
		}
		if len(inplay) > 0 {
			fmt.Printf("in play:")
			for _, c := range inplay {
				fmt.Printf(" %v", c.name)
			}
			fmt.Println("");
		}
		fmt.Printf("[a:%v b:%v c:%v]> ", player.a, player.b, player.c)
		prog := ""
		_, err := fmt.Scanf("%s", &prog)
		if err == io.EOF {
						panic("unexpected EOF")
		}
		i := 0
		msg := ""
		for ; i < len(prog); i++ {
			switch prog[i] {
			case '+':
				if player.b == 0 {
					msg = "no buys left"
					break
				}
				i++
				if i == len(prog) {
					msg = "expected card"
					break
				}
				choice := keyToCard(prog[i])
				if choice == nil {
					msg = "no such card"
					break
				}
				if choice.cost > player.c {
					msg = "insufficient money"
					break
				}
				if supply[choice] == 0 {
					msg = "supply exhausted"
					break
				}
				fmt.Printf("%v gained\n", choice.name)
				discard = append(discard, choice)
				supply[choice]--
				player.c -= choice.cost
				player.b--
			case '.':
				player.b = 0
			default:
				c := keyToCard(prog[i])
				if c == nil {
					msg = "unrecognized command"
					break
				}
				if !c.HasKind(kTreasure) {
					msg = "treasure expected"
					break
				}
				var k int
				for k = len(hand)-1; k >= 0; k-- {
					if hand[k] == c {
						player.c += c.n
						hand = append(hand[:k], hand[k+1:]...)
						inplay = append(inplay, c)
						break
					}
				}
				if k < 0 {
					msg = "none in hand"
				}
			}
			if msg != "" {
				fmt.Printf("Error: %v\n  %v\n  ", msg, prog)
				for j := 0; j < i; j++ {
					fmt.Printf(" ")
				}
				fmt.Printf("^\n")
				break
			}
		}
	}
}
