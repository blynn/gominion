package main

import (
	"fmt"
	"io"
	"math/rand"
	"regexp"
	"strconv"
	"strings"
)

type Kind struct {
	name string
}

type Card struct {
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

type Keycard struct {
	key byte
	card *Card
}

var (
	KindDict = make(map[string]*Kind)
	CardDict = make(map[string]*Card)
	supply = make(map[*Card]int)
	suplist []Keycard
)

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
		suplist = append(suplist, Keycard{key, c})
	}
	layout("Copper", 'q')
	layout("Silver", 'w')
	layout("Gold", 'e')
	layout("Estate", 'a')
	layout("Duchy", 's')
	layout("Province", 'd')
	layout("Curse", '-')
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
			fmt.Printf("Next turn!\n")
			discard, inplay = append(discard, inplay...), nil
			discard, hand = append(discard, hand...), nil
			player = Player{a:1, b:1, c:0}
			draw(5)
		}
		for _, v := range suplist {
			fmt.Printf("[%c] %v(%v) $%v\n", v.key, v.card.name, supply[v.card], v.card.cost)
		}
		fmt.Printf("Deck size: %v\n", len(deck))
		fmt.Printf("Discard size: %v ", len(discard))
		if len(discard) > 0 {
			fmt.Printf("Top: %v\n", discard[len(discard)-1].name)
		} else {
			fmt.Printf("(empty)\n")
		}
		fmt.Println("Hand:")
		for i, c := range hand {
			fmt.Printf("%v: %v\n", i+1, c.name)
		}
		if len(inplay) > 0 {
			fmt.Printf("in play:")
			for _, c := range inplay {
				fmt.Printf(" %v", c.name)
			}
			fmt.Println("");
		}
		fmt.Printf("[a:%v b:%v c:%v]> ", player.a, player.b, player.c)
		var cmd string
		_, err := fmt.Scanf("%s", &cmd)
		if err == io.EOF {
						panic("unexpected EOF")
		}
		match, err := regexp.MatchString("[0-9]+", cmd)
		if err != nil {
			panic("bad regexp")
		}
		if match {
			k, err := strconv.Atoi(cmd)
			if err != nil {
				fmt.Println("bad number")
				continue
			}
			if k < 0 || k > len(hand) {
				fmt.Println("out of range")
				continue
			}
			if k == 0 {
				player.b = 0
				continue
			}
			c := hand[k-1]
			if !c.HasKind(kTreasure) {
				fmt.Println("treasure expected")
				continue
			}
			player.c += c.n
			hand = append(hand[:k-1], hand[k:]...)
			inplay = append(inplay, c)
			continue
		}
		if len(cmd) != 1 {
			fmt.Println("single character expected")
			continue
		}
		var choice *Card
		for _, v := range suplist {
			if cmd[0] == v.key {
				choice = v.card
				break
			}
		}
		if choice == nil {
			fmt.Println("no such card")
			continue
		}
		if choice.cost > player.c {
			fmt.Println("insufficient money")
			continue
		}
		if supply[choice] == 0 {
			fmt.Println("supply exhausted")
			continue
		}
		fmt.Printf("%v gained\n", choice.name)
		discard = append(discard, choice)
		supply[choice]--
		player.c -= choice.cost
		player.b--
	}
}
