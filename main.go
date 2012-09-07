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
	kTreasure, kVictory, kCurse *Kind
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

type Game struct {
	players []*Player
	n int  // Index of current player.
	suplist Pile
	ch chan Command
	newTurn bool
}

type Command struct {
	s string
	c *Card
}

type Player struct {
	name string
	fun func(chan *Game)
	a, b, c int
	deck, hand, played, discard Pile
	ch chan *Game
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
}

func getKind(s string) *Kind {
	k, ok := KindDict[s]
	if !ok {
		panic("no such kind: " + s)
	}
	return k
}

func CanBuy(p *Player, c *Card) string {
	switch {
		case p.b == 0:
			return "no buys left"
		case c.cost > p.c:
			return "insufficient money"
		case c.supply == 0:
			return "supply exhausted"
	}
	return ""
}

func main() {
	rand.Seed(2)
	for _, s := range []string{"Treasure", "Victory", "Curse"} {
		KindDict[s] = &Kind{s}
	}
	kTreasure = getKind("Treasure")
	kVictory = getKind("Victory")
	kCurse = getKind("Curse")

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

	game := &Game{ch: make(chan Command)}
	game.players = []*Player{
		&Player{name:"Ben", fun:consoleGamer},
		&Player{name:"AI", fun:consoleGamer},
	}
	players := game.players

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
		for i := 0; i < 3; i++ {
			p.deck.Add("Estate")
		}
		for i := 0; i < 7; i++ {
			p.deck.Add("Copper")
		}
		p.deck.shuffle()
		p.draw(5)
		p.ch = make(chan *Game)
		go p.fun(p.ch)
	}
	layout := func(s string, key byte) {
		c, ok := CardDict[s]
		if !ok {
			panic("unknown card: " + s)
		}
		game.suplist = append(game.suplist, c)
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
			for _, c := range game.suplist {
				if c.HasKind(kVictory) || c.HasKind(kCurse) {
					v := m[c]
					fmt.Printf("%v x %v = %v\n", v.count, c.name, v.pts)
				}
			}
		}
	}

	for game.n = 0;; game.n = (game.n+1) % len(players) {
		p := players[game.n]
		p.a, p.b, p.c = 1, 1, 0
		fmt.Printf("%v to move\n", p.name)
		game.newTurn = true
		for p.b > 0 {
			p.ch <- game
			cmd := <-game.ch
			game.newTurn = false
			switch cmd.s {
				case "buy":
					choice := cmd.c
					if CanBuy(p, choice) != "" {
						panic(CanBuy(p, choice))
					}
					fmt.Printf("%v spends %v coins\n", p.name, choice.cost)
					fmt.Printf("%v gains %v\n", p.name, choice.name)
					p.discard = append(p.discard, choice)
					choice.supply--
					p.c -= choice.cost
					p.b--
				case "play":
					var k int
					for k = len(p.hand)-1; k >= 0; k-- {
						if p.hand[k] == cmd.c {
							p.play(k)
							break
						}
					}
					if k < 0 {
						panic("unplayable")
					}
				case "next":
					p.b = 0
				case "quit":
					p.cleanup()
					gameOver()
					return
			}
		}
		fmt.Println("Cleaning up...")
		p.cleanup()
		n := 0
		for _, c := range game.suplist {
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

func consoleGamer(ch chan *Game) {
	reader := bufio.NewReader(os.Stdin)
	var game *Game
	keyToCard := func(key byte) *Card {
		for _, c := range game.suplist {
			if key == c.key {
				return c
			}
		}
		return nil
	}
	dump := func(p *Player) {
		for _, c := range game.suplist {
			fmt.Printf("[%c] %v(%v) $%v\n", c.key, c.name, c.supply, c.cost)
		}
		fmt.Printf("Player/Deck/Hand/Discard\n")
		for _, p := range game.players {
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
	i := 0
	prog := ""
	allTreasures := false
	for {
		game = <-ch
		p := game.players[game.n]
		if game.newTurn {
			dump(p)
		}
		game.ch <- func() Command {
			for {
				if allTreasures {
					for k := len(p.hand)-1; k >= 0; k-- {
						if p.hand[k].HasKind(kTreasure) {
							return Command{"play", p.hand[k]}
						}
					}
				}
				allTreasures = false
				i++
				for i >= len(prog) {
					fmt.Printf("a:%v b:%v c:%v> ", p.a, p.b, p.c)
					s, err := reader.ReadString('\n')
					if err == io.EOF {
						fmt.Printf("\nQuitting game...\n")
						p.cleanup()
						return Command{"quit", nil}
					}
					if err != nil {
						panic(err)
					}
					prog, i = s, 0
				}
				msg := ""
				switch prog[i] {
				case '\n':
				case ' ':
				case '+':
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
					msg = CanBuy(p, choice)
					if msg == "" {
						return Command{"buy", choice}
					}
				case '?':
					dump(p)
				case '.':
					return Command{"next", nil}
				case '*':
					allTreasures = true
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
							return Command{"play", c}
						}
					}
					msg = "none in hand"
				}
				if msg != "" {
					fmt.Printf("Error: %v\n  %v\n  ", msg, prog)
					for j := 0; j < i; j++ {
						fmt.Printf(" ")
					}
					fmt.Printf("^\n")
					prog, i = "", 0
					continue
				}
			}
			panic("unreachable")
		}()
	}
}
