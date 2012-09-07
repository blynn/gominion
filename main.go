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
	name string
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
	fmt.Printf("%v plays %v\n", p.name, c.name)
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
	for _, s := range []string{"Treasure", "Victory", "Curse"} {
		KindDict[s] = &Kind{s}
	}
	kTreasure := getKind("Treasure")
	kVictory := getKind("Victory")
	kCurse := getKind("Curse")

	db := `
Copper,0,Treasure,$1
Silver,3,Treasure,$2
Gold,6,Treasure,$3
Estate,2,Victory,V1
Duchy,5,Victory,V3
Province,8,Victory,V6
Curse,0,Curse,C1
`
	for _, s := range strings.Split(db, "\n") {
		if len(s) == 0 {
			continue
		}
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

	fmt.Println("Gominion")

	players := []*Player{&Player{name:"Ben"}, &Player{name:"AI"}}

	setSupply := func(s string, n int) {
		c, ok := CardDict[s]
		if !ok {
			panic("no such card: " + s)
		}
		c.supply = n
	}
	setSupply("Copper", 60 - 7*len(players))
	setSupply("Silver", 40)
	setSupply("Gold", 30)

	numVictoryCards := func(n int) int {
		switch n {
		case 2: return 8
		case 3: return 12
		case 4: return 12
		}
		panic(n)
	}
	for _, s := range []string{"Estate", "Duchy", "Province"} {
		setSupply(s, numVictoryCards(len(players)))
	}
	setSupply("Curse", 10*(len(players) - 1))
  for _, p := range players {
		p.a, p.b, p.c = 1, 1, 0
		for i := 0; i < 3; i++ {
			p.deck.Add("Estate")
		}
		for i := 0; i < 7; i++ {
			p.deck.Add("Copper")
		}
		p.deck.shuffle()
		p.draw(5)
	}
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
		for _, p := range players {
			p.deck, p.discard = append(p.deck, p.discard...), nil
			p.deck, p.hand = append(p.deck, p.hand...), nil
			score := 0
			m := make(map[*Card]struct {
				count, pts int
			})
			for _, c := range p.deck {
				if c.HasKind(kVictory) || c.HasKind(kCurse) {
					v := m[c]
					v.count++
					v.pts += c.n
					m[c] = v
					score += c.n
				}
			}
			fmt.Printf("%v: %v\n", p.name, score)
			for _, c := range suplist {
				if c.HasKind(kVictory) || c.HasKind(kCurse) {
					v := m[c]
					fmt.Printf("%v x %v = %v\n", v.count, c.name, v.pts)
				}
			}
		}
	}
	reader := bufio.NewReader(os.Stdin)
	dump := func(p *Player) {
		for _, c := range suplist {
			fmt.Printf("[%c] %v(%v) $%v\n", c.key, c.name, c.supply, c.cost)
		}
		fmt.Printf("Player/Deck/Hand/Discard\n")
		for _, p := range players {
			fmt.Printf("%v/%v/%v/%v", p.name, len(p.deck), len(p.hand), len(p.discard))
			if len(p.discard) > 0 {
				fmt.Printf(":%v", p.discard[len(p.discard)-1].name)
			}
			fmt.Println("")
		}
		fmt.Println("Hand:")
		for _, c := range p.hand {
			fmt.Printf("[%c] %v\n", c.key, c.name)
		}
		if len(p.played) > 0 {
			fmt.Printf("Played:")
			for _, c := range p.played {
				fmt.Printf(" %v", c.name)
			}
			fmt.Println("");
		}
	}

	for k := 0;; k = (k + 1) % len(players) {
		p := players[k]
		fmt.Printf("%v to move\n", p.name)
		dump(p)
		for {
			if p.b == 0 {
				fmt.Println("Cleaning up...")
				p.cleanup()
				break
			}
			fmt.Printf("a:%v b:%v c:%v> ", p.a, p.b, p.c)
			prog, err := reader.ReadString('\n')
			if err == io.EOF {
				fmt.Printf("\nQuitting game...\n")
				p.cleanup()
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
					if p.b == 0 {
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
					if choice.cost > p.c {
						msg = "insufficient money"
						break
					}
					if choice.supply == 0 {
						msg = "supply exhausted"
						break
					}
					fmt.Printf("%v spends %v coins\n", p.name, choice.cost)
					fmt.Printf("%v gains %v\n", p.name, choice.name)
					p.discard = append(p.discard, choice)
					choice.supply--
					p.c -= choice.cost
					p.b--
				case '?':
					dump(p)
				case '.':
					p.b = 0
				case '*':
					for k := len(p.hand)-1; k >= 0; k-- {
						if p.hand[k].HasKind(kTreasure) {
							p.play(k)
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
					for k = len(p.hand)-1; k >= 0; k-- {
						if p.hand[k] == c {
							p.play(k)
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
}
