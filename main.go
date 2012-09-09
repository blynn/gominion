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

func (game *Game) multiplay(k int, n int) {
	p := game.NowPlaying()
	c := p.hand[k]
	p.hand = append(p.hand[:k], p.hand[k+1:]...)
	p.played = append(p.played, c)
	if c.HasKind(kAction) {
		p.a--
	}
	for ;n > 0; n-- {
		fmt.Printf("%v plays %v\n", p.name, c.name)
		if c.act == nil {
			fmt.Printf("unimplemented  :(")
			return
		}
		game.stack = append(game.stack, &Frame{card:c})
		for _, f := range c.act {
			f(game)
		}
		game.stack = game.stack[:len(game.stack)-1]
	}
}

func (game *Game) play(k int) {
	game.multiplay(k, 1)
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
	card *Card
}

type Command struct {
	s string
	c *Card
}

type PlayFun interface {
	start(chan *Game, *Player)
}

type Player struct {
	name string
	fun PlayFun
	a, b, c int
	deck, hand, played, discard Pile
	ch chan *Game
}

func (p *Player) MaybeShuffle() {
	if len(p.deck) == 0 {
		if len(p.discard) == 0 {
			return
		}
		p.deck, p.discard = p.discard, p.deck
		p.deck.shuffle()
	}
}

func (p *Player) draw(n int) int {
	if n == 0 {
		return 0
	}
	p.MaybeShuffle()
	fmt.Printf("%v draws %v\n", p.name, p.deck[0].name)
	p.hand, p.deck = append(p.hand, p.deck[0]), p.deck[1:]
	return 1 + p.draw(n-1)
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
			if isVictory(c) || c.HasKind(kCurse) {
				v := m[c]
				v.count++
				v.pts += c.vp
				m[c] = v
				score += c.vp
			}
		}
		fmt.Printf("%v: %v\n", p.name, score)
		for _, c := range game.suplist {
			if isVictory(c) || c.HasKind(kCurse) {
				v := m[c]
				fmt.Printf("%v x %v = %v\n", v.count, c.name, v.pts)
			}
		}
	}
}

func (game *Game) getCommand(p *Player) Command {
	p.ch <- game
	cmd := <-game.ch
	if cmd.s == "quit" {
		p.cleanup()
		game.Over()
		os.Exit(0)
	}
	return cmd
}

func (game *Game) pick(list []*Card, p *Player, n int, exact bool, cond func(*Card) string) []bool {
	sel := make([]bool, len(list))
	game.SetParse(func(b byte) (Command, string) {
		if b == '/' {
			return Command{"done", nil}, ""
		}
		choice := game.keyToCard(b)
		if choice == nil {
			return errCmd, "unrecognized card"
		}
		if !func() bool {
			for i, c := range list {
				if sel[i] {
					continue
				}
				if c == choice {
					return true
				}
			}
			return false
		}() {
			return errCmd, "invalid choice"
		}
		if cond != nil {
			if msg := cond(choice); msg != "" {
				return errCmd, msg
			}
		}
		return Command{"pick", choice}, ""
	})
	for stop := false; !stop; {
		cmd := game.getCommand(p)
		switch cmd.s {
			case "pick":
				found := false
				for i, c := range list {
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
				if exact && n > 0 {
					panic("must pick more")
				}
				stop = true
			default:
			  panic("bad command: " + cmd.s)
		}
	}
	return sel
}

func pickHand(game *Game, p *Player, n int, exact bool, cond func(*Card) string) []bool {
	return game.pick(p.hand, p, n, exact, cond)
}

func (game *Game) Gain(p *Player, c *Card) {
	if c.supply == 0 {
		panic("out of supply")
	}
	fmt.Printf("%v gains %v\n", p.name, c.name)
	p.discard = append(p.discard, c)
	c.supply--
}

func (game *Game) GainIfPossible(p *Player, c *Card) {
	if c.supply > 0 {
		game.Gain(p, c)
	}
}

func pickGain(game *Game, max int) {
	game.SetParse(func(b byte) (Command, string) {
		c := game.keyToCard(b)
		if c == nil {
			return errCmd, "expected card"
		}
		if c.cost > max {
			return errCmd, "too expensive"
		}
		if c.supply == 0 {
			return errCmd, "supply exhausted"
		}
		return Command{"pick", c}, ""
	})
	p := game.NowPlaying()
	cmd := game.getCommand(p)
	if cmd.s != "pick" {
		panic("bad command: " + cmd.s)
	}
	if cmd.c.cost > max {
		panic("too expensive")
	}
	game.Gain(p, cmd.c)
}

var errCmd Command

func inHand(p *Player, cond func(*Card) bool) bool {
	for _, c := range p.hand {
		if cond(c) {
			return true
		}
	}
	return false
}

func reacts(game *Game, p *Player) bool {
	if !inHand(p, func(c *Card) bool { return c.name == "Moat" }) {
		return false
	}
	sel := pickHand(game, p, 1, false, func(c *Card) string {
		if c.name != "Moat" {
			return "pick Moat or nothing"
		}
		return ""
	})
	for i, c := range p.hand {
		if sel[i] {
			if c.name != "Moat" {
				panic("invalid attack reaction: " + c.name)
			}
			fmt.Printf("%v reveals %v\n", p.name, c.name)
			return true
		}
	}
	return false
}

func (game *Game) ForOthers(fun func(*Player)) {
	m := len(game.players)
	for i := (game.n+1)%m; i != game.n; i = (i+1)%m {
		fun(game.players[i])
	}
}

func (game *Game) attack(fun func(*Player)) {
	m := len(game.players)
	for i := (game.n+1)%m; i != game.n; i = (i+1)%m {
		other := game.players[i]
		fmt.Printf("%v attacks %v\n", game.NowPlaying().name, other.name)
		if reacts(game, other) {
			continue
		}
		fun(other)
	}
}

func isVictory(c *Card) bool { return c.HasKind(kVictory) }

func (game *Game) getBool(p *Player) bool {
	game.SetParse(func(b byte) (Command, string) {
		switch b {
			case '\\':
				return Command{"yes", nil}, ""
			case '/':
				return Command{"done", nil}, ""
		}
		return errCmd, "\\ for yes, / for no"
	})
	cmd := game.getCommand(p)
	switch cmd.s {
	case "yes":
		return true
	case "done":
		return false
	}
	panic("bad command: " + cmd.s)
}

func main() {
	rand.Seed(60)
	for _, s := range []string{"Treasure", "Victory", "Curse", "Action", "Attack", "Reaction"} {
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
Moat,2,Action-Reaction,+C2
Chancellor,3,Action,$2
Village,3,Action,+C1,+A2
Woodcutter,3,Action,+B1,$2
Workshop,3,Action
Bureaucrat,4,Action-Attack
Feast,4,Action
Militia,4,Action-Attack,$2
Moneylender,4,Action
Remodel,4,Action
Smithy,4,Action,+C3
Spy,4,Action-Attack,+C1,+A1
Thief,4,Action-Attack
Throne Room,4,Action
Council Room,5,Action,+C4,+B1
Festival,5,Action,+A2,+B1,$2
Laboratory,5,Action,+C2,+A1
Library,5,Action
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
		for _, s := range strings.Split(a[2], "-") {
			kind, ok := KindDict[s]
			if !ok {
				panic("no such kind: " + a[2])
			}
			c.kind = append(c.kind, kind)
		}
		CardDict[c.name] = c
		add := func(fun func(game *Game)) {
			c.act = append(c.act, fun)
		}
		for i := 3; i < len(a); i++ {
			s := a[i]
			switch s[0] {
			case '$':
				add(func(game *Game) { game.addCoins(PanickyAtoi(s[1:])) })
			case '#':
				c.vp = PanickyAtoi(s[1:])
			case '+':
				switch s[1] {
					case 'A':
						add(func(game *Game) { game.addActions(PanickyAtoi(s[2:])) })
					case 'B':
						add(func(game *Game) { game.addBuys(PanickyAtoi(s[2:])) })
					case 'C':
						add(func(game *Game) { game.addCards(PanickyAtoi(s[2:])) })
					default:
						panic(s)
				}
			default:
				panic(s)
			}
		}
		switch c.name {
		case "Cellar":
			add(func(game *Game) {
				p := game.NowPlaying()
				selected := pickHand(game, p, len(p.hand), false, nil)
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
			add(func(game *Game) {
				p := game.NowPlaying()
				selected := pickHand(game, p, 4, false, nil)
				for i := len(p.hand)-1; i >= 0; i-- {
					if selected[i] {
						fmt.Printf("%v trashes %v\n", p.name, p.hand[i].name)
						game.trash = append(game.trash, p.hand[i])
						p.hand = append(p.hand[:i], p.hand[i+1:]...)
					}
				}
			})
		case "Chancellor":
			add(func(game *Game) {
				p := game.NowPlaying()
				if game.getBool(p) {
					fmt.Printf("%v discards deck\n", p.name)
					p.discard, p.deck = append(p.discard, p.deck...), nil
				}
			})
		case "Bureaucrat":
			add(func(game *Game) {
				p := game.NowPlaying()
				game.GainIfPossible(p, GetCard("Silver"))
				game.attack(func(other *Player) {
					if !inHand(other, isVictory) {
						for _, c := range other.hand {
							fmt.Printf("%v reveals %v\n", other.name, c.name)
						}
						return
					}
					sel := pickHand(game, other, 1, true, func(c *Card) string {
						if !isVictory(c) {
							return "must pick Victory card"
						}
						return ""
					})
					for i := len(other.hand)-1; i >= 0; i-- {
						if sel[i] {
							fmt.Printf("%v decks %v\n", other.name, other.hand[i].name)
							other.deck = append(other.deck, other.hand[i])
							other.hand = append(other.hand[:i], other.hand[i+1:]...)
							break
						}
					}
				})
			})
		case "Feast":
			add(func(game *Game) {
				p := game.NowPlaying()
				game.trash = append(game.trash, p.played[len(p.played)-1])
				p.played = p.played[:len(p.played)-1]
				pickGain(game, 5)
			})
		case "Workshop":
			add(func(game *Game) { pickGain(game, 4) })
		case "Militia":
			add(func(game *Game) {
				game.attack(func(other *Player) {
					if len(other.hand) <= 3 {
						return
					}
					sel := pickHand(game, other, 3, true, nil)
					for i := len(other.hand)-1; i >= 0; i-- {
						if !sel[i] {
							fmt.Printf("%v discards %v\n", other.name, other.hand[i].name)
							other.discard = append(other.discard, other.hand[i])
							other.hand = append(other.hand[:i], other.hand[i+1:]...)
						}
					}
				})
			})
		case "Moneylender":
			add(func(game *Game) {
				p := game.NowPlaying()
				isCopper := func(c *Card) bool { return c.name == "Copper" }
				if !inHand(p, isCopper) {
					return
				}
				for i, c := range p.hand {
					if isCopper(c) {
						fmt.Printf("%v trashes %v\n", p.name, c.name)
						game.trash = append(game.trash, p.hand[i])
						p.hand = append(p.hand[:i], p.hand[i+1:]...)
						p.c += 3
						return
					}
				}
			})
		case "Remodel":
			add(func(game *Game) {
				p := game.NowPlaying()
				if len(p.hand) == 0 {
					return
				}
				sel := pickHand(game, p, 1, true, nil)
				for i, c := range p.hand {
					if sel[i] {
						fmt.Printf("%v trashes %v\n", p.name, c.name)
						game.trash = append(game.trash, p.hand[i])
						p.hand = append(p.hand[:i], p.hand[i+1:]...)
						pickGain(game, c.cost + 2)
						return
					}
				}
			})
		case "Spy":
			add(func(game *Game) {
				p := game.NowPlaying()
				game.attack(func(other *Player) {
					other.MaybeShuffle()
					if len(other.deck) == 0 {
						return
					}
					c := other.deck[0]
					fmt.Printf("%v reveals %v\n", other.name, c.name)
					if game.getBool(p) {
						fmt.Printf("%v discards %v\n", other.name, c.name)
						other.discard = append(other.discard, c)
						other.deck = other.deck[1:]
					}
				})
			})
		case "Thief":
			add(func(game *Game) {
				p := game.NowPlaying()
				game.attack(func(other *Player) {
					var v []*Card
					found := 0
					trashi := 0
					for i := 0; i < 2; i++ {
						other.MaybeShuffle()
						if len(other.deck) == 0 {
							break
						}
						c := other.deck[0]
						fmt.Printf("%v reveals %v\n", other.name, c.name)
						other.deck = other.deck[1:]
						v = append(v, c)
						if c.HasKind(kTreasure) {
							trashi = i
							found++
						}
					}
					if found > 1 {
						sel := game.pick(v, p, 1, true, func(c *Card) string {
							if !c.HasKind(kTreasure) {
								return "must pick Treasure"
							}
							return ""
						})
						for i, b := range sel {
							if b {
								trashi = i
							}
						}
					}
					if found > 0 {
						c := v[trashi]
						fmt.Printf("%v trashes %v\n", other.name, c.name)
						v = append(v[:trashi], v[trashi+1:]...)
						if c.supply > 0 && game.getBool(p) {
							game.Gain(p, c)
						}
					}
					for _, c := range v {
						fmt.Printf("%v discards %v\n", other.name, c.name)
						other.discard = append(other.discard, c)
					}
				})
			})
		case "Throne Room":
			add(func(game *Game) {
				p := game.NowPlaying()
				isAction := func(c *Card) bool { return c.HasKind(kAction) }
				if !inHand(p, isAction) {
					return
				}
				sel := pickHand(game, p, 1, true, func(c *Card) string {
					if !isAction(c) {
						return "must pick Action"
					}
					return ""
				})
				for i, _ := range p.hand {
					if sel[i] {
						p.a++  // Compensate for Action deducted by multiplay().
						game.multiplay(i, 2)
						return
					}
				}
			})
		case "Council Room":
			add(func(game *Game) {
				game.ForOthers(func(other *Player) { other.draw(1) })
			})
		case "Library":
			add(func(game *Game) {
				p := game.NowPlaying()
				var v []*Card
				for {
					if len(p.hand) == 7 {
						break
					}
					if p.draw(1) == 0 {
						break
					}
					c := p.hand[len(p.hand)-1]
					if c.HasKind(kAction) {
						if game.getBool(p) {
							fmt.Printf("%v sets aside %v\n", p.name, c.name)
							p.hand = p.hand[:len(p.hand)-1]
							v = append(v, c)
						}
					}
				}
				p.discard = append(p.discard, v...)
			})
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
		go p.fun.start(p.ch, p)
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
	for i, s := range strings.Split("Cellar,Bureaucrat,Village,Workshop,Moneylender,Remodel,Throne Room,Militia,Library", ",") {
		setSupply(s, 10)
		layout(s, keys[i])
	}

	for game.n = 0;; game.n = (game.n+1) % len(players) {
		p := game.NowPlaying()
		p.a, p.b, p.c = 1, 1, 0
		for game.phase = phAction; game.phase <= phCleanup; {
			cmd := game.getCommand(p)
			switch cmd.s {
				case "buy":
					choice := cmd.c
					if err := CanBuy(game, choice); err != "" {
						panic(err)
					}
					fmt.Printf("%v spends %v coins\n", p.name, choice.cost)
					p.c -= choice.cost
					p.b--
					game.Gain(p, choice)
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
		fmt.Printf("%v cleans up\n", p.name)
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

func (consoleGamer) start(ch chan *Game, p *Player) {
	reader := bufio.NewReader(os.Stdin)
	var game *Game
	dump := func() {
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
			dump()
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
					if p.a == 0 || !inHand(p, func(c *Card) bool { return c.HasKind(kAction) }) {
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
					fmt.Printf("a:%v b:%v c:%v %v", p.a, p.b, p.c, p.name)
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
					dump()
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

func (this simpleBuyer) start(ch chan *Game, p *Player) {
	for {
		game := <-ch
		for {
			frame := game.StackTop()
			if frame == nil {
				break
			}
			switch frame.card.name {
			case "Bureaucrat":
				for _, c := range p.hand {
					if isVictory(c) {
						game.ch <- Command{"pick", c}
						game = <-ch
						break
					}
				}
			case "Militia":
				// TODO: Better discard strategy.
				for i := 0; i < 3; i++ {
					game.ch <- Command{"pick", p.hand[i]}
					game = <-ch
				}
			default:
				panic("AI unimplemented: " + frame.card.name)
			}
		}
		game.ch<- func() Command {
			if game.phase == phAction {
				return Command{"next", nil}
			}
			if game.phase == phCleanup {
				return Command{"next", nil}
			}
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
