package main

import (
	"bufio"
	"fmt"
	"io"
	"math/rand"
	"os"
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
	supply int
}

func PanickyAtoi(s string) int {
	n, err := strconv.Atoi(s)
	if err != nil {
		panic(s)
	}
	return n
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
	deck, hand, played, discard Pile
}

func (p *Player) draw(n int) {
	if n == 0 {
		return
	}
	if len(p.deck) == 0 {
		if len(p.discard) == 0 {
			return
		}
		p.deck, p.discard = p.discard, p.deck
		p.deck.shuffle()
	}
	p.hand, p.deck = append(p.hand, p.deck[0]), p.deck[1:]
	p.draw(n-1)
}

func (p *Player) play(k int) {
	c := p.hand[k]
	p.c += c.n
	p.hand = append(p.hand[:k], p.hand[k+1:]...)
	p.played = append(p.played, c)
}

func (p *Player) cleanup() {
	p.discard, p.played = append(p.discard, p.played...), nil
	p.discard, p.hand = append(p.discard, p.hand...), nil
	p.draw(5)
	p.a, p.b, p.c = 1, 1, 0
}

func getKind(s string) *Kind {
	k, ok := KindDict[s]
	if !ok {
		panic("no such kind: " + s)
	}
	return k
}

func main() {
	rand.Seed(2)
	initKind := func(s string) {
		KindDict[s] = &Kind{s}
	}
	for _, s := range []string{"Treasure", "Victory", "Curse"} {
		initKind(s)
	}
	kTreasure := getKind("Treasure")
	kVictory := getKind("Victory")
	kCurse := getKind("Curse")

	initCard := func(s string) {
		a := strings.Split(s, ",")
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
				c.n = PanickyAtoi(s[1:])
			case 'V':
				c.n = PanickyAtoi(s[1:])
			case 'C':
				c.n = -PanickyAtoi(s[1:])
			}
		}
	}
	db := `
Copper,0,Treasure,$1
Silver,3,Treasure,$2
Gold,6,Treasure,$3
Estate,2,Victory,V1
Duchy,5,Victory,V3
Province,8,Victory,V6
Curse,0,Curse,C1
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
		c.supply = n
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
	player := Player{a:1, b:1, c:0}
	for i := 0; i < 3; i++ {
		player.deck.Add("Estate")
	}
	for i := 0; i < 7; i++ {
		player.deck.Add("Copper")
	}
	player.deck.shuffle()
	fmt.Println("")

	layout := func(s string, key byte) {
		c, ok := CardDict[s]
		if !ok {
			panic("unknown card: " + s)
		}
		suplist = append(suplist, c)
                c.key = key
	}
	layout("Copper", '1')
	layout("Silver", '2')
	layout("Gold", '3')
	layout("Estate", 'q')
	layout("Duchy", 'w')
	layout("Province", 'e')
	layout("Curse", '!')
	gameOver := func() {
		fmt.Printf("Game over\n")
		player.deck, player.discard = append(player.deck, player.discard...), nil
		player.deck, player.hand = append(player.deck, player.hand...), nil
		score := 0
		m := make(map[*Card]struct {
			count, pts int
		})
		for _, c := range player.deck {
			if c.HasKind(kVictory) || c.HasKind(kCurse) {
				v := m[c]
				v.count++
				v.pts += c.n
				m[c] = v
				score += c.n
			}
		}
		for _, c := range suplist {
			if c.HasKind(kVictory) || c.HasKind(kCurse) {
				v := m[c]
				fmt.Printf("%v x %v: %v\n", v.count, c.name, v.pts)
			}
		}
		fmt.Printf("Score: %v\n", score)
	}
	player.draw(5)
	reader := bufio.NewReader(os.Stdin)
	for {
		if player.b == 0 {
			fmt.Println("Cleaning up...")
			player.cleanup()
			n := 0
			for _, c := range suplist {
				if c.supply == 0 {
					if c.name == "Province" {
						n = 3
						break
					}
					n++
					if n == 3 {
						break
					}
				}
			}
			if n == 3 {
				gameOver()
				return
			}
		}
		for _, c := range suplist {
			fmt.Printf("[%c] %v(%v) $%v\n", c.key, c.name, c.supply, c.cost)
		}
		fmt.Printf("Deck(%v)\n", len(player.deck))
		fmt.Printf("Discard(%v): ", len(player.discard))
		if len(player.discard) > 0 {
			fmt.Printf("%v\n", player.discard[len(player.discard)-1].name)
		} else {
			fmt.Printf("(empty)\n")
		}
		fmt.Println("Hand:")
		for _, c := range player.hand {
			fmt.Printf("[%c] %v\n", c.key, c.name)
		}
		if len(player.played) > 0 {
			fmt.Printf("Played:")
			for _, c := range player.played {
				fmt.Printf(" %v", c.name)
			}
			fmt.Println("");
		}
		fmt.Printf("a:%v b:%v c:%v> ", player.a, player.b, player.c)
		prog, err := reader.ReadString('\n')
		if err == io.EOF {
			fmt.Printf("\nQuitting game...\n")
			player.cleanup()
			gameOver()
			return
		}
		if err != nil {
			panic(err)
		}
		i := 0
		msg := ""
		for ; i < len(prog); i++ {
			switch prog[i] {
			case '\n':
			case ' ':
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
				if choice.supply == 0 {
					msg = "supply exhausted"
					break
				}
				fmt.Printf("%v gained\n", choice.name)
				player.discard = append(player.discard, choice)
				choice.supply--
				player.c -= choice.cost
				player.b--
			case '.':
				player.b = 0
			case '*':
				for k := len(player.hand)-1; k >= 0; k-- {
					if player.hand[k].HasKind(kTreasure) {
						player.play(k)
					}
				}
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
				for k = len(player.hand)-1; k >= 0; k-- {
					if player.hand[k] == c {
						player.play(k)
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
