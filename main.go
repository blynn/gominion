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
	vp int
	supply int
	act []func(*Game)
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

func (c *Card) AddEffect(fun func(game *Game)) {
	c.act = append(c.act, fun)
}

type Pile []*Card

var (
	KindDict = make(map[string]*Kind)
	CardDict = make(map[string]*Card)
	kTreasure, kVictory, kCurse, kAction *Kind
)

func GetCard(s string) *Card {
	c, ok := CardDict[s]
	if !ok {
		panic("unknown card: " + s)
	}
	return c
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

type Game struct {
	players []*Player
	n int  // Index of current player.
	suplist Pile
	ch chan Command
	phase int
	stack []*Frame
	trash Pile
}

const (
	phAction = iota
	phBuy
	phCleanup
)

func (game *Game) keyToCard(key byte) *Card {
	for _, c := range game.suplist {
		if key == c.key {
			return c
		}
	}
	return nil
}

func (game *Game) play(k int) {
	p := game.NowPlaying()
	c := p.hand[k]
	p.hand = append(p.hand[:k], p.hand[k+1:]...)
	p.played = append(p.played, c)
	if c.HasKind(kAction) {
		p.a--
	}
	fmt.Printf("%v plays %v\n", p.name, c.name)
	if c.act == nil {
		fmt.Printf("unimplemented  :(\n")
		return
	}
	game.stack = append(game.stack, &Frame{card:c, game:game})
	for _, f := range c.act {
		f(game)
	}
	game.stack = game.stack[:len(game.stack)-1]
}

func (game Game) addCoins(n int) {
	game.NowPlaying().c += n
}

func (game Game) addActions(n int) {
	game.NowPlaying().a += n
}

func (game Game) addBuys(n int) {
	game.NowPlaying().b += n
}

func (game Game) addCards(n int) {
	game.NowPlaying().draw(n)
}

func (game *Game) SetParse(fun func(b byte) (Command, string)) {
	game.stack[len(game.stack)-1].Parse = fun
}

func (game *Game) HasStack() bool {
	return len(game.stack) > 0
}

func (game *Game) StackTop() *Frame {
	n := len(game.stack)
	if n == 0 {
		return nil
	}
	return game.stack[n-1]
}

func (game *Game) NowPlaying() *Player {
	return game.players[game.n]
}

type Frame struct {
	Parse func(b byte) (Command, string)
	game *Game
	card *Card
}

type Command struct {
	s string
	c *Card
}

type PlayFun interface {
	start(chan *Game)
}

type Player struct {
	name string
	fun PlayFun
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
	fmt.Printf("%v draws %v\n", p.name, p.deck[0].name)
	p.hand, p.deck = append(p.hand, p.deck[0]), p.deck[1:]
	p.draw(n-1)
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

func CanPlay(game *Game, c *Card) string {
	switch {
		case c.HasKind(kAction):
			if game.phase != phAction {
				return "wrong phase"
			}
			if game.NowPlaying().a == 0 {
				return "out of actions"
			}
		case c.HasKind(kTreasure):
			if game.phase != phBuy {
				return "wrong phase"
			}
		default:
				return "unplayable card"
	}
	return ""
}

func CanBuy(game *Game, c *Card) string {
	p := game.NowPlaying()
	switch {
		case game.phase != phBuy:
			return "wrong phase"
		case p.b == 0:
			return "no buys left"
		case c.cost > p.c:
			return "insufficient money"
		case c.supply == 0:
			return "supply exhausted"
	}
	return ""
}

func (game *Game) Over() {
	fmt.Printf("Game over\n")
	for _, p := range game.players {
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
				v.pts += c.vp
				m[c] = v
				score += c.vp
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

func (game *Game) getCommand() Command {
	p := game.NowPlaying()
	p.ch <- game
	cmd := <-game.ch
	if cmd.s == "quit" {
		p.cleanup()
		game.Over()
		os.Exit(0)
	}
	return cmd
}

func pickHand(game *Game, n int) []bool {
	p := game.NowPlaying()
	sel := make([]bool, len(p.hand))
	game.SetParse(func(b byte) (Command, string) {
		if b == '/' {
			return Command{"done", nil}, ""
		}
		choice := game.keyToCard(b)
		if choice == nil {
			return Command{}, "unrecognized card"
		}
		if !func() bool {
			for i, c := range p.hand {
				if sel[i] {
					continue
				}
				if c == choice {
					return true
				}
			}
			return false
		}() {
			return Command{}, "invalid choice"
		}
		return Command{"pick", choice}, ""
	})
	for stop := false; !stop; {
		cmd := game.getCommand()
		switch cmd.s {
			case "pick":
				found := false
				for i, c := range p.hand {
					if !sel[i] && c == cmd.c {
						sel[i] = true
						n--
						found = true
						break
					}
				}
				if !found {
					panic("invalid selection")
				}
				stop = n == 0
			case "done":
				stop = true
			default:
			  panic("bad command: " + cmd.s)
		}
	}
	return sel
}

func pickGain(game *Game, max int) {
	game.SetParse(func(b byte) (Command, string) {
		c := game.keyToCard(b)
		if c == nil {
			return Command{}, "expected card"
		}
		if c.cost > max {
			return Command{}, "too expensive"
		}
		if c.supply == 0 {
			return Command{}, "supply exhausted"
		}
		return Command{"pick", c}, ""
	})
	cmd := game.getCommand()
	if cmd.s != "pick" {
		panic("bad command: " + cmd.s)
	}
	if cmd.c.cost > max || cmd.c.supply == 0 {
		panic("invalid choice")
	}
	p := game.NowPlaying()
	fmt.Printf("%v gains %v\n", p.name, cmd.c.name)
	p.discard = append(p.discard, cmd.c)
	cmd.c.supply--
}

func main() {
	rand.Seed(60)
	for _, s := range []string{"Treasure", "Victory", "Curse", "Action"} {
		KindDict[s] = &Kind{s}
	}
	kTreasure = getKind("Treasure")
	kVictory = getKind("Victory")
	kCurse = getKind("Curse")
	kAction = getKind("Action")
	for _, s := range strings.Split(`
Copper,0,Treasure,$1
Silver,3,Treasure,$2
Gold,6,Treasure,$3
Estate,2,Victory,#1
Duchy,5,Victory,#3
Province,8,Victory,#6
Curse,0,Curse,#-1

Cellar,2,Action,+A1
Chapel,2,Action
Chancellor,3,Action,$2
Village,3,Action,+C1,+A2
Woodcutter,3,Action,+B1,$2
Workshop,3,Action
Feast,4,Action
Smithy,4,Action,+C3
Festival,5,Action,+A2,+B1,$2
Laboratory,5,Action,+C2,+A1
Market,5,Action,+C1,+A1,+B1,$1
`, "\n") {
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
				c.AddEffect(func(game *Game) { game.addCoins(PanickyAtoi(s[1:])) })
			case '#':
				c.vp = PanickyAtoi(s[1:])
			case '+':
				switch s[1] {
					case 'A':
						c.AddEffect(func(game *Game) { game.addActions(PanickyAtoi(s[2:])) })
					case 'B':
						c.AddEffect(func(game *Game) { game.addBuys(PanickyAtoi(s[2:])) })
					case 'C':
						c.AddEffect(func(game *Game) { game.addCards(PanickyAtoi(s[2:])) })
					default:
						panic(s)
				}
			default:
				panic(s)
			}
		}
		switch c.name {
		case "Cellar":
			c.AddEffect(func(game *Game) {
				p := game.NowPlaying()
				selected := pickHand(game, len(p.hand))
				for i := len(p.hand)-1; i >= 0; i-- {
					if selected[i] {
						fmt.Printf("%v discards %v\n", p.name, p.hand[i].name)
						p.discard = append(p.discard, p.hand[i])
						p.hand = append(p.hand[:i], p.hand[i+1:]...)
					}
				}
				p.draw(len(selected))
			})
		case "Chapel":
			c.AddEffect(func(game *Game) {
				p := game.NowPlaying()
				selected := pickHand(game, 4)
				for i := len(p.hand)-1; i >= 0; i-- {
					if selected[i] {
						fmt.Printf("%v trashes %v\n", p.name, p.hand[i].name)
						game.trash = append(game.trash, p.hand[i])
						p.hand = append(p.hand[:i], p.hand[i+1:]...)
					}
				}
			})
		case "Chancellor":
			c.AddEffect(func(game *Game) {
				game.SetParse(func(b byte) (Command, string) {
					switch b {
						case '\\':
							return Command{"yes", nil}, ""
						case '/':
							return Command{"done", nil}, ""
					}
					return Command{}, "\\ for yes, / for no"
				})
				cmd := game.getCommand()
				switch cmd.s {
				case "yes":
					p := game.NowPlaying()
					fmt.Printf("%v discards deck\n", p.name)
					p.discard, p.deck = append(p.discard, p.deck...), nil
				case "done":
				default:
					panic("bad command: " + cmd.s)
				}
			})
		case "Feast":
			c.AddEffect(func(game *Game) {
				p := game.NowPlaying()
				game.trash = append(game.trash, p.played[len(p.played)-1])
				p.played = p.played[:len(p.played)-1]
				pickGain(game, 5)
			})
		case "Workshop":
			c.AddEffect(func(game *Game) { pickGain(game, 4) })
		}
	}

	fmt.Println("Gominion")

	game := &Game{ch: make(chan Command)}
	game.players = []*Player{
		&Player{name:"Ben", fun:consoleGamer{}},
		&Player{name:"AI", fun:simpleBuyer{ []string{"Province", "Gold", "Silver"} }},
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
		go p.fun.start(p.ch)
	}
	layout := func(s string, key byte) {
		c := GetCard(s)
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
	keys := "asdfgzxcvb"
	for i, s := range strings.Split("Cellar,Chapel,Chancellor,Village,Woodcutter,Workshop,Feast,Smithy", ",") {
		setSupply(s, 10)
		layout(s, keys[i])
	}

	for game.n = 0;; game.n = (game.n+1) % len(players) {
		p := game.NowPlaying()
		p.a, p.b, p.c = 1, 1, 0
		fmt.Printf("%v to play\n", p.name)
		for game.phase = phAction; game.phase <= phCleanup; {
			cmd := game.getCommand()
			switch cmd.s {
				case "buy":
					choice := cmd.c
					if err := CanBuy(game, choice); err != "" {
						panic(err)
					}
					fmt.Printf("%v spends %v coins\n", p.name, choice.cost)
					fmt.Printf("%v gains %v\n", p.name, choice.name)
					p.discard = append(p.discard, choice)
					choice.supply--
					p.c -= choice.cost
					p.b--
				case "play":
					if err := CanPlay(game, cmd.c); err != "" {
						panic(err)
					}
					var k int
					for k = len(p.hand)-1; k >= 0; k-- {
						if p.hand[k] == cmd.c {
							game.play(k)
							break
						}
					}
					if k < 0 {
						panic("unplayable")
					}
				case "next":
					game.phase++
			}
		}
		fmt.Printf("%v Cleanup phase\n", p.name)
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
			game.Over()
			return
		}
	}
}

type consoleGamer struct {}

func (consoleGamer) start(ch chan *Game) {
	reader := bufio.NewReader(os.Stdin)
	var game *Game
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
	newTurn := true
	wildCard := false
	for {
		game = <-ch
		p := game.NowPlaying()
		if newTurn {
			dump(p)
			newTurn = false
		}
		game.ch <- func() Command {
			// Automatically advance to next phase when it's obvious.
			if !game.HasStack() {
				if game.phase == phCleanup {
					newTurn = true
					return Command{"next", nil}
				}
				if game.phase == phAction {
					if p.a == 0 || func() bool {
						for _, c := range p.hand {
							if c.HasKind(kAction) {
								return false
							}
						}
						return true
					}() {
						return Command{"next", nil}
					}
				}
				if p.b == 0 {
					return Command{"next", nil}
				}
			}

			for {
				// TODO: Change this to modify 'prog' (by inserting all treasure
				// cards) when first encountered.
				if wildCard {
					for k := len(p.hand)-1; k >= 0; k-- {
						if p.hand[k].HasKind(kTreasure) {
							return Command{"play", p.hand[k]}
						}
					}
				}
				wildCard = false
				i++
				frame := game.StackTop()
				for i >= len(prog) {
					fmt.Printf("a:%v b:%v c:%v", p.a, p.b, p.c)
					if frame != nil {
						fmt.Printf(" %v", frame.card.name)
					}
					fmt.Printf("> ")
					s, err := reader.ReadString('\n')
					if err == io.EOF {
						fmt.Printf("\nQuitting game...\n")
						return Command{"quit", nil}
					}
					if err != nil {
						panic(err)
					}
					prog, i = s, 0
				}
				match := true
				switch prog[i] {
				case '\n':
				case ' ':
				case '?':
					dump(p)
				default:
					match = false
				}
				if (match) {
					continue
				}
				msg := ""
				if frame != nil {
					var cmd Command
					if cmd, msg = frame.Parse(prog[i]); msg == "" {
						return cmd
					}
				} else {
					switch prog[i] {
					case '+': fallthrough
					case ';':
						i++
						if i == len(prog) {
							msg = "expected card"
							break
						}
						choice := game.keyToCard(prog[i])
						if choice == nil {
							msg = "no such card"
							break
						}
						if msg = CanBuy(game, choice); msg != "" {
							break
						}
						return Command{"buy", choice}
					case '.':
						return Command{"next", nil}
					case '*':
						if game.phase != phBuy {
							msg = "wrong phase"
							break
						}
						wildCard = true
					default:
						c := game.keyToCard(prog[i])
						if c == nil {
							msg = "unrecognized command"
							break
						}
						if msg = CanPlay(game, c); msg != "" {
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

type simpleBuyer struct {
	list []string
}

func (this simpleBuyer) start(ch chan *Game) {
	for {
		game := <-ch
		game.ch<- func() Command {
			if game.phase == phAction {
				return Command{"next", nil}
			}
			if game.phase == phCleanup {
				return Command{"next", nil}
			}
			p := game.NowPlaying()
			if p.b == 0 {
				return Command{"next", nil}
			}
			for k := len(p.hand)-1; k >= 0; k-- {
				if p.hand[k].HasKind(kTreasure) {
					return Command{"play", p.hand[k]}
				}
			}
			for _, s := range this.list {
				c := GetCard(s)
				if p.c >= c.cost {
					return Command{"buy", c}
				}
			}
			return Command{"next", nil}
		}()
	}
}
