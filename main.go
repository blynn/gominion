package main

import (
	"bufio"
	"flag"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"math/rand"
	"net/http"
	"os"
	"regexp"
	"strconv"
	"strings"
	"time"
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
	vp func(*Game) int
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

func isVictory(c *Card) bool { return c.HasKind(kVictory) }
func isTreasure(c *Card) bool { return c.HasKind(kTreasure) }
func isAction(c *Card) bool { return c.HasKind(kAction) }

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
        isServer bool
}

const (
	phAction = iota
	phBuy
	phCleanup
	phCard
)

func (game *Game) dump() {
	cols := []int {3,3,1,3,3,3,1}
	for _, c := range game.suplist {
		fmt.Printf("  [%c] %v(%v) $%v", c.key, c.name, c.supply, c.cost)
		cols[0]--
		if cols[0] == 0 {
			fmt.Println()
			cols = cols[1:]
		}
	}
	fmt.Printf("Player/Deck/Hand/Discard\n")
	for _, p := range game.players {
		fmt.Printf("%v/%v/%v/%v", p.name, len(p.deck), len(p.hand), len(p.discard))
		if len(p.discard) > 0 {
			fmt.Printf(":%v", p.discard[len(p.discard)-1].name)
		}
		fmt.Println("")
	}
}

func (p *Player) dumpHand() {
	fmt.Println("Hand:")
	n := 0
	for _, c := range p.hand {
		fmt.Printf(" [%c] %v", c.key, c.name)
		n = (n+1)%5
		if n == 0 {
			println()
		}
	}
	if n != 0 {
		println()
	}
	if len(p.played) > 0 {
		fmt.Printf("Played:")
		for _, c := range p.played {
			fmt.Printf(" %v", c.name)
		}
		fmt.Println("");
	}
}

func (game *Game) TrashHand(p *Player, i int) {
	fmt.Printf("%v trashes %v\n", p.name, p.hand[i].name)
	game.trash = append(game.trash, p.hand[i])
	p.hand = append(p.hand[:i], p.hand[i+1:]...)
}

func (game *Game) keyToCard(key byte) *Card {
	for _, c := range game.suplist {
		if key == c.key {
			return c
		}
	}
	return nil
}

func (game *Game) MultiPlay(n int, c *Card, m int) {
	p := game.players[n]
	if !p.hidden {
		var k int
		for k = len(p.hand)-1; k >= 0; k-- {
			if p.hand[k] == c {
				p.hand = append(p.hand[:k], p.hand[k+1:]...)
				break
			}
		}
		if k < 0 {
			panic("unplayable")
		}
	}
	p.played = append(p.played, c)
	if isAction(c) {
		p.a--
	}
	for ;m > 0; m-- {
		if c.act == nil {
			fmt.Printf("%v unimplemented  :(\n", c.name)
			return
		}
		game.stack = append(game.stack, &Frame{card:c})
		for _, f := range c.act {
			f(game)
		}
		game.stack = game.stack[:len(game.stack)-1]
	}
}

func (game *Game) Play(n int, c *Card) {
	game.cast(Event{s:"play", card:c, n:n})
	game.MultiPlay(n, c, 1)
}

func (game *Game) Spend(n int, c *Card) {
	game.cast(Event{s:"buy", card:c, n:game.n})
	p := game.players[n]
	p.c -= c.cost
	p.b--
}

func (game Game) addCoins(n int) {
	game.cast(Event{s:"+C", n:game.n, i:n})
	game.NowPlaying().c += n
}

func (game Game) addActions(n int) {
	game.cast(Event{s:"+A", n:game.n, i:n})
	game.NowPlaying().a += n
}

func (game Game) addBuys(n int) {
	game.cast(Event{s:"+B", n:game.n, i:n})
	game.NowPlaying().b += n
}

func (game Game) addCards(n int) {
	game.draw(game.NowPlaying(), n)
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
	start(*Game, *Player)
}

type Player struct {
	name string
	fun PlayFun
	a, b, c int
	deck, hand, played, discard Pile
	tv chan Event
	hidden bool
	n int
}

type Event struct {
	s string
	card *Card
	parse func(b byte) (Command, string)
	n int
	phase int
	i int
}

// MaybeShuffle returns true if deck is non-empty, shuffling the discards
// into a new deck if necessary.
func (p *Player) MaybeShuffle() bool {
	if len(p.deck) == 0 {
		if len(p.discard) == 0 {
			return false
		}
		p.deck, p.discard = p.discard, p.deck
		p.deck.shuffle()
	}
	return true
}

func (game *Game) drawOne(p *Player) bool {
	if !p.MaybeShuffle() {
		return false
	}
	c := p.deck[0]
	p.deck, p.hand = p.deck[1:], append(p.hand, c)
	return true
}

func (game *Game) draw(p *Player, n int) int {
	i := 0
	for ; i < n; i++ {
		if !game.drawOne(p) {
			break
		}
	}
	if i > 0 {
		game.cast(Event{s:"draw", n:game.n, i:i})
	}
	return i
}

func (game *Game) Cleanup(p *Player) {
	game.cast(Event{s:"cleanup", n:game.n})
	p.discard, p.played = append(p.discard, p.played...), nil
	p.discard, p.hand = append(p.discard, p.hand...), nil
}

func (game *Game) cast(ev Event) {
	for _, p := range game.players {
		if p.tv != nil {
			p.tv <- ev
		}
	}
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
		case isAction(c):
			if game.phase != phAction {
				return "wrong phase"
			}
			if game.NowPlaying().a == 0 {
				return "out of actions"
			}
		case isTreasure(c):
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
	for i, p := range game.players {
		game.n = i  // We want NowPlaying() for some VP computations.
		p.deck, p.discard = append(p.deck, p.discard...), nil
		p.deck, p.hand = append(p.deck, p.hand...), nil
		score := 0
		m := make(map[*Card]struct {
			count, pts int
		})
		for _, c := range p.deck {
			if isVictory(c) || c.HasKind(kCurse) {
				if c.vp == nil {
					fmt.Printf("%v unimplemented  :(\n", c.name)
					continue
				}
				n := c.vp(game)
				v := m[c]
				v.count++
				v.pts += n
				m[c] = v
				score += n
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
	p.tv <- func() Event {
		frame := game.StackTop()
		if frame != nil {
			return Event{s:"go", phase:phCard, card:frame.card, parse:frame.Parse}
		}
		return Event{s:"go", n:p.n, phase:game.phase}
	}()
	cmd := <-game.ch
	if cmd.s == "quit" {
		game.Cleanup(p)
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
	game.cast(Event{s:"gain", card:c, n:game.n})
	p.discard = append(p.discard, c)
	c.supply--
}

func (game *Game) GainIfPossible(p *Player, c *Card) {
	if c.supply > 0 {
		game.Gain(p, c)
	}
}

func pickGainCond(game *Game, max int, fun func(*Card) string) *Card {
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
		if fun != nil {
			if msg := fun(c); msg != "" {
				return errCmd, msg
			}
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
	return cmd.c
}

func pickGain(game *Game, max int) {
	pickGainCond(game, max, nil)
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
Gardens,4,Victory
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
Mine,5,Action
Witch,5,Action-Attack,+C2
Adventurer,6,Action
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
				c.vp = func(game *Game) int { return PanickyAtoi(s[1:]) }
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
				n := 0
				for i := len(p.hand)-1; i >= 0; i-- {
					if selected[i] {
						fmt.Printf("%v discards %v\n", p.name, p.hand[i].name)
						p.discard = append(p.discard, p.hand[i])
						p.hand = append(p.hand[:i], p.hand[i+1:]...)
						n++
					}
				}
				game.draw(p, n)
			})
		case "Chapel":
			add(func(game *Game) {
				p := game.NowPlaying()
				selected := pickHand(game, p, 4, false, nil)
				for i := len(p.hand)-1; i >= 0; i-- {
					if selected[i] {
						game.TrashHand(p, i)
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
		case "Gardens":
			c.vp = func(game *Game) int { return len(game.NowPlaying().deck) / 10 }
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
						game.TrashHand(p, i)
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
						game.TrashHand(p, i)
						pickGain(game, c.cost + 2)
						return
					}
				}
				panic("unreachable")
			})
		case "Spy":
			add(func(game *Game) {
				p := game.NowPlaying()
				game.attack(func(other *Player) {
					if !other.MaybeShuffle() {
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
						if !other.MaybeShuffle() {
							break
						}
						c := other.deck[0]
						fmt.Printf("%v reveals %v\n", other.name, c.name)
						other.deck = other.deck[1:]
						v = append(v, c)
						if isTreasure(c) {
							trashi = i
							found++
						}
					}
					if found > 1 {
						sel := game.pick(v, p, 1, true, func(c *Card) string {
							if !isTreasure(c) {
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
						p.a++  // Compensate for Action deducted by MultiPlay().
						game.MultiPlay(p.n, p.hand[i], 2)
						return
					}
				}
			})
		case "Council Room":
			add(func(game *Game) {
				game.ForOthers(func(other *Player) { game.draw(other, 1) })
			})
		case "Library":
			add(func(game *Game) {
				p := game.NowPlaying()
				var v []*Card
				for {
					if len(p.hand) == 7 {
						break
					}
					if game.draw(p, 1) == 0 {
						break
					}
					c := p.hand[len(p.hand)-1]
					if isAction(c) && game.getBool(p) {
						fmt.Printf("%v sets aside %v\n", p.name, c.name)
						p.hand = p.hand[:len(p.hand)-1]
						v = append(v, c)
					}
				}
				p.discard = append(p.discard, v...)
			})
		case "Mine":
			add(func(game *Game) {
				p := game.NowPlaying()
				if !inHand(p, isTreasure) {
					return
				}
				f := func(c *Card) string {
					if !isTreasure(c) {
						return "must pick Treasure"
					}
					return ""
				}
				sel := pickHand(game, p, 1, true, f)
				for i, c := range p.hand {
					if sel[i] {
						game.TrashHand(p ,i)
						choice := pickGainCond(game, c.cost + 3, f)
						fmt.Printf("%v puts %v into hand\n", p.name, choice.name)
						p.hand = append(p.hand, choice)
						p.discard = p.discard[:len(p.discard)-1]
						return
					}
				}
				panic("unreachable")
			})
		case "Witch":
			add(func(game *Game) {
				game.ForOthers(func(other *Player) { game.GainIfPossible(other, GetCard("Curse")) })
			})
		case "Adventurer":
			add(func(game *Game) {
				p := game.NowPlaying()
				for n := 2; n > 0 && p.MaybeShuffle(); {
					if isTreasure(p.deck[0]) {
						fmt.Printf("%v puts %v in hand\n", p.name, p.deck[0].name)
						p.hand = append(p.hand, p.deck[0])
						n--
					} else {
						fmt.Printf("%v discards %v\n", p.name, p.deck[0].name)
						p.discard = append(p.discard, p.deck[0])
					}
					p.deck = p.deck[1:]
				}
			})
		}
	}

	flag.Parse()
	if flag.NArg() > 0 {
		client()
		return
	}

	fmt.Println("= Gominion =")
	rand.Seed(time.Now().Unix())
	game := &Game{ch: make(chan Command), isServer: true}
	ng := netGamer{
		in: make(chan Command),
		out: make(chan string),
	}
	game.players = []*Player{
		&Player{name:"Anonymous", fun:ng},
		&Player{name:"Ben", fun:consoleGamer{}},
		//&Player{name:"AI", fun:simpleBuyer{ []string{"Province", "Gold", "Silver"} }},
	}
	players := game.players

	http.HandleFunc("/poll", func(w http.ResponseWriter, r *http.Request) {
		ng.in <- Command{s:"poll"}
		fmt.Fprintf(w, <-ng.out)
	})

	http.HandleFunc("/cmd", func(w http.ResponseWriter, r *http.Request) {
		s := r.FormValue("s")
		if s == "" {
			fmt.Fprintf(w, "error: no command")
			return
		}
		cmd := Command{s:s}
		if c := r.FormValue("c"); c != "" {
			if len(c) != 1 {
				fmt.Fprintf(w, "error: malformed card")
			}
			cmd.c = game.keyToCard(c[0])
			if cmd.c == nil {
				fmt.Fprintf(w, "error: no such card")
				return
			}
		}
		ng.in <- cmd
		fmt.Fprintf(w, <-ng.out)
	})

  go func() {
		log.Fatal(http.ListenAndServe(":8080", nil))
	}()
	for n := 0;; n++ {
		resp, err := http.Get("http://:8080/")
		if err != nil {
			if n > 3 {
				log.Fatal("failed to connect 3 times: ", err)
			}
			time.Sleep(1 * time.Second)
			continue
		}
		defer resp.Body.Close()
		break;
	}

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

	type Preset struct {
		name string
		cards []*Card
	}
	var presets []Preset
	for _, line := range strings.Split(`
First Game:Cellar,Market,Militia,Mine,Moat,Remodel,Smithy,Village,Woodcutter,Workshop
Big Money:Adventurer,Bureaucrat,Chancellor,Chapel,Feast,Laboratory,Market,Mine,Moneylender,Throne Room
Interaction:Bureaucrat,Chancellor,Council Room,Festival,Library,Militia,Moat,Spy,Thief,Village
Size Distortion:Cellar,Chapel,Feast,Gardens,Laboratory,Thief,Village,Witch,Woodcutter,Workshop
Village Square:Bureaucrat,Cellar,Festival,Library,Market,Remodel,Smithy,Throne Room,Village,Woodcutter
`, "\n") {
		if len(line) == 0 {
			continue
		}
		s := strings.Split(line, ":")
		pr := Preset{name:s[0]}
		for _, s := range strings.Split(s[1], ",") {
			c := GetCard(s)
			// Insertion sort.
			pr.cards = func(cards []*Card) []*Card {
				for i, x := range cards {
					if x.cost == c.cost && x.name > c.name || x.cost > c.cost {
						return append(cards[:i], append([]*Card{c}, cards[i:]...)...)
					}
				}
				return append(cards, c)
			}(pr.cards)
		}
		presets = append(presets, pr)
	}

	fmt.Println("Picking preset:")
	for _, pr := range presets {
		fmt.Printf("  %v", pr.name)
	}
	fmt.Println();
	pr := presets[rand.Intn(len(presets))]
	fmt.Printf("Playing \"%v\"\n", pr.name)
	for i, c := range pr.cards {
		c.supply = 10
		layout(c.name, keys[i])
	}
setSupply("Province", 1)
	for i, p := range players {
		for i := 0; i < 3; i++ {
			p.deck.Add("Estate")
		}
		for i := 0; i < 7; i++ {
			p.deck.Add("Copper")
		}
		p.deck.shuffle()
		p.hand, p.deck = p.deck[:5], p.deck[5:]
		p.tv = make(chan Event)
		p.n = i;
		go p.fun.start(game, p)
		p.tv <- Event{s:"new"}
	}

	for game.n = 0;; game.n = (game.n+1) % len(players) {
		p := game.NowPlaying()
		p.a, p.b, p.c = 1, 1, 0
		first := true
		for game.phase = phAction; game.phase <= phCleanup; {
			if first {
				game.cast(Event{s:"phase", phase:game.phase, n:game.n})
				first = false
			}
			cmd := game.getCommand(p)
			switch cmd.s {
				case "buy":
					choice := cmd.c
					if err := CanBuy(game, choice); err != "" {
						panic(err)
					}
					game.Spend(game.n, choice)
					game.Gain(p, choice)
				case "play":
					if err := CanPlay(game, cmd.c); err != "" {
						panic(err)
					}
					game.Play(game.n, cmd.c)
				case "next":
					game.phase++
					first = true
			}
		}
		game.Cleanup(p)
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
		game.draw(p, 5)
	}
}

type consoleGamer struct {}

func (consoleGamer) start(game *Game, p *Player) {
	<-p.tv
	game.dump()
	reader := bufio.NewReader(os.Stdin)
	i := 0
	prog := ""
	wildCard := false
	buyMode := false
	for {
		ev := <-p.tv
		switch ev.s {
		case "phase":
			if ev.n == p.n && ev.phase == phAction {
				p.dumpHand()
			}
		case "play":
			fmt.Printf("%v plays %v\n", game.players[ev.n].name, ev.card.name)
		case "gain":
			fmt.Printf("%v gains %v\n", game.players[ev.n].name, ev.card.name)
		case "buy":
			fmt.Printf("%v buys %v for $%v\n", game.players[ev.n].name, ev.card.name, ev.card.cost)
		case "draw":
			if p.n != ev.n {
				fmt.Printf("%v draws %v cards\n", game.players[ev.n].name, ev.i)
			} else {
				for i := ev.i; i > 0; i-- {
					c := p.hand[len(p.hand)-i]
					fmt.Printf("%v draws [%c] %v\n", p.name, c.key, c.name)
				}
			}
		case "cleanup":
			fmt.Printf("%v cleans up\n", game.players[ev.n].name)
		case "go": game.ch <- func() Command {
			// Automatically advance to next phase when it's obvious.
			if ev.phase == phAction && (p.a == 0 || !inHand(p, isAction)) {
				return Command{"next", nil}
			}
			if ev.phase == phBuy && p.b == 0 {
				return Command{"next", nil}
			}
			if ev.phase == phCleanup {
				return Command{"next", nil}
			}
			if ev.phase != phBuy {
				buyMode = false
			} else if !inHand(p, isTreasure) {
				buyMode = true
			}

			for {
				if wildCard {
					if ev.phase == phBuy {
						for k := len(p.hand)-1; k >= 0; k-- {
							if isTreasure(p.hand[k]) {
								return Command{"play", p.hand[k]}
							}
						}
					}
					wildCard = false
				}
				i++
				for i >= len(prog) {
					fmt.Printf("a:%v b:%v c:%v %v", p.a, p.b, p.c, p.name)
					if ev.phase == phCard {
						fmt.Printf(" %v", ev.card.name)
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
					game.dump()
					p.dumpHand()
				default:
					match = false
				}
				if (match) {
					continue
				}
				msg := ""
				if ev.phase == phCard {
					var cmd Command
					if cmd, msg = ev.parse(prog[i]); msg == "" {
						return cmd
					}
				} else {
					switch prog[i] {
					case '+': fallthrough
					case ';':
						if ev.phase != phBuy {
							msg = "wrong phase"
							break
						}
						if inHand(p, isTreasure) {
							buyMode = !buyMode
						}
					case '.':
						return Command{"next", nil}
					case '*':
						if ev.phase != phBuy {
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
						if buyMode {
							if msg = CanBuy(game, c); msg != "" {
								break
							}
							return Command{"buy", c}
						}
						if msg = CanPlay(game, c); msg != "" {
							break
						}
						// TODO: Move this to CanPlay().
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
		default:
			log.Printf("ignoring event %q", ev.s)
		}
	}
}

type simpleBuyer struct {
	list []string
}

func (this simpleBuyer) start(game *Game, p *Player) {
	<-p.tv
	for {
		ev := <-p.tv
		if ev.s != "go"{
			continue
		}
		if ev.phase == phCard {
			switch ev.card.name {
			case "Bureaucrat":
				for _, c := range p.hand {
					if isVictory(c) {
						game.ch <- Command{"pick", c}
						ev = <-p.tv
						break
					}
				}
				panic("unreachable")
			case "Militia":
				// TODO: Better discard strategy.
				for i := 0; i < 3; i++ {
					game.ch <- Command{"pick", p.hand[i]}
					ev = <-p.tv
				}
			default:
				panic("AI unimplemented: " + ev.card.name)
			}
			continue
		}
		game.ch<- func() Command {
			switch ev.phase {
			case phAction:
				return Command{"next", nil}
			case phCleanup:
				return Command{"next", nil}
			}
			if ev.phase != phBuy {
				panic("unknown event: " + ev.s)
			}
			if p.b == 0 {
				return Command{"next", nil}
			}
			for k := len(p.hand)-1; k >= 0; k-- {
				if isTreasure(p.hand[k]) {
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

type netGamer struct {
	in chan Command
	out chan string
}

func encodeKingdom(game *Game) string {
	s := ""
	for _, c := range game.suplist {
		s += fmt.Sprintf("%v,%v,%v\n", c.name, c.supply, c.key)
	}
	return s
}

func encodeHand(p *Player) string {
	s := ""
	for _, c := range p.hand {
		s += string(c.key)
	}
	return s + "\n"
}

func encodePlayers(ps []*Player) string {
	s := ""
	for _, p := range ps {
		s += p.name + "\n"
	}
	return s
}

func (this netGamer) start(game *Game, p *Player) {
	var q []Event
	ready := false
	for {
		select {
		case ev := <-p.tv:
			q = append(q, ev)
		case cmd := <-this.in:
			switch cmd.s {
			case "poll":
				if len(q) == 0 {
					if ready {
						this.out <- "Go!"
					} else {
						this.out <- "wait"
					}
					break
				}
				ev := q[0]
				q = q[1:]
				switch ev.s {
				case "new":
					this.out <- "new\n= Players =\n" + encodePlayers(game.players) + "= Kingdom =\n" + encodeKingdom(game) + "= Hand =\n" + encodeHand(p)
				case "go":
					ready = true
				  fallthrough
				case "phase":
					this.out <- fmt.Sprintf("%v\n%v,%v\n", ev.s, ev.n, ev.phase)
				case "cleanup":
					this.out <- fmt.Sprintf("%v\n%v\n", ev.s, ev.n)
				case "play":
				  fallthrough
				case "buy":
				  fallthrough
				case "gain":
					this.out <- fmt.Sprintf("%v\n%v,%c\n", ev.s, ev.n, ev.card.key)
				case "draw":
					s := ""
					for i := ev.i; i > 0; i-- {
						if p.n == ev.n {
							s += string(p.hand[len(p.hand) - i].key)
						} else {
							s += "?"
						}
					}
					this.out <- fmt.Sprintf("%v\n%v,%v\n", ev.s, ev.n, s)
				case "+C":
					fallthrough
				case "+A":
					fallthrough
				case "+B":
					this.out <- fmt.Sprintf("%v\n%v,%v\n", ev.s, ev.n, ev.i)
				default:
					this.out <- "BUG"
				}
			default:
				if !ready {
					this.out <- "error: not ready"
				} else {
					ready = false
					game.ch<-cmd;
					this.out <- "sent"
				}
			}
		}
	}
}

func client() {
	send := func(u string) string {
		resp, err := http.Get(u)
		if err != nil {
			log.Fatal(err)
		}
		defer resp.Body.Close()
		body, err := ioutil.ReadAll(resp.Body)
		if err != nil {
			log.Fatal(err)
		}
		return string(body)
	}
	var game *Game
	var p *Player
	for {
		body := send("http://:8080/poll")
		if body == "wait" {
			time.Sleep(200 * time.Millisecond)
			continue
		}
		v := strings.Split(body, "\n")
		splitComma := func() []string {
			if len(v) < 2 {
				log.Fatalf("malformed response: %q", body)
			}
			return strings.Split(v[1], ",")
		}
		if len(v) == 0 {
			log.Printf("malformed response: %q", body)
			continue
		}
		switch v[0] {
		case "new":
			game = &Game{ch: make(chan Command)}
			p = &Player{name:"Anonymous", fun:consoleGamer{}, tv:make(chan Event)}
			heading := ""
			for _, line := range v[1:] {
				if len(line) == 0 {
					continue
				}
				re := regexp.MustCompile("= (.*) =")
				if h := re.FindStringSubmatch(line); h != nil {
					heading = h[1]
					continue
				}
				switch heading {
				case "Players":
					if line == p.name {
						game.players = append(game.players, p)
					} else {
						game.players = append(game.players, &Player{name:line, hidden:true})
					}
				case "Hand":
					for _, c := range []byte(line) {
						p.hand = append(p.hand, game.keyToCard(c))
					}
				case "Kingdom":
					w := strings.Split(line, ",")
					if len(w) != 3 {
						log.Printf("malformed line: %q", line)
						break
					}
					c := GetCard(w[0])
					game.suplist = append(game.suplist, c)
					c.supply = PanickyAtoi(w[1])
					c.key = byte(PanickyAtoi(w[2]))
				default:
					log.Printf("unknown heading: %q", heading)
				}
			}
			go p.fun.start(game, p)
			p.tv <- Event{s:"new"}
		case "buy":
			w := splitComma()
			ev := Event{s:"buy", n:PanickyAtoi(w[0]), card:game.keyToCard(w[1][0])}
			game.Spend(ev.n, ev.card)
		case "cleanup":
			if len(v) < 2 {
				log.Printf("malformed response: %q", body)
				continue
			}
			ev := Event{s:"cleanup", n:PanickyAtoi(v[1])}
			game.Cleanup(game.players[ev.n])
		case "gain":
			w := splitComma()
			ev := Event{s:"gain", n:PanickyAtoi(w[0]), card:game.keyToCard(w[1][0])}
			game.Gain(game.players[ev.n], ev.card)
		case "play":
			w := splitComma()
			ev := Event{s:"play", n:PanickyAtoi(w[0]), card:game.keyToCard(w[1][0])}
			if p.n == ev.n {
				var k int
				for k = len(p.hand)-1; k >= 0; k-- {
					if p.hand[k] == ev.card {
						p.hand = append(p.hand[:k], p.hand[k+1:]...)
						break
					}
				}
				if k < 0 {
					panic("unplayable")
				}
			}
			p.tv <- ev
		case "phase":
			w := splitComma()
			ev := Event{s:"phase", n:PanickyAtoi(w[0]), phase:PanickyAtoi(w[1])}
			game.n, game.phase = ev.n, ev.phase
			if ev.n == p.n && ev.phase == phAction {
				p.a, p.b, p.c = 1, 1, 0
			}
			p.tv <- ev
		case "go":
			w := splitComma()
			p.tv <- Event{s:"go", n:PanickyAtoi(w[0]), phase:PanickyAtoi(w[1])}
			cmd := <-game.ch
			u := "http://:8080/cmd?s=" + cmd.s
			if cmd.c != nil {
				u += "&c=" + string(cmd.c.key)
			}
		  send(u)
		case "draw":
			w := splitComma()
			ev := Event{s:"draw", n:PanickyAtoi(w[0]), i:len(w[1])}
			if ev.n == p.n {
				for _, b := range []byte(w[1]) {
					p.hand = append(p.hand, game.keyToCard(b))
				}
			}
			p.tv <- ev
		case "+A":
			w := splitComma()
			if p.n == PanickyAtoi(w[0]) {
				p.a += PanickyAtoi(w[1])
			}
		case "+B":
			w := splitComma()
			if p.n == PanickyAtoi(w[0]) {
				p.b += PanickyAtoi(w[1])
			}
		case "+C":
			w := splitComma()
			if p.n == PanickyAtoi(w[0]) {
				p.c += PanickyAtoi(w[1])
			}
		default:
			log.Fatalf("unknown: %q", body)
		}
	}
}
