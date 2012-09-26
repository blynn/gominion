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
	key    byte
	name   string
	cost   int
	kind   []*Kind
	coin   int
	vp     func(*Game) int
	supply int
	act    []func(*Game)
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
	KindDict                             = make(map[string]*Kind)
	CardDict                             = make(map[string]*Card)
	kTreasure, kVictory, kCurse, kAction *Kind
)

func isVictory(c *Card) bool  { return c.HasKind(kVictory) }
func isTreasure(c *Card) bool { return c.HasKind(kTreasure) }
func isAction(c *Card) bool   { return c.HasKind(kAction) }

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
	players    []*Player
	n          int // Index of current player.
	a, b, c    int // Actions, Buys, Coins,
	suplist    Pile
	ch         chan Command
	phase      int
	stack      []*Frame
	trash      Pile
	sendCmd    func(game *Game, p *Player, cmd *Command)
	isServer   bool
	fetch      func() []string
	GetDiscard func(game *Game, p *Player) string

	// Actions played. (Can differ to actions spent because of e.g.
	// Throne Room.)
	aCount      int
	discount    int
	copperbonus int
}

const (
	phAction = iota
	phBuy
	phCleanup
)

func (game *Game) TrashCard(p *Player, c *Card) {
	game.trash = append(game.trash, c)
	game.Report(Event{s: "trash", n: p.n, card: c})
}

func (game *Game) TrashList(p *Player, list Pile) {
	for _, c := range list {
		game.TrashCard(p, c)
	}
}

func (game *Game) DiscardList(p *Player, list Pile) {
	if len(list) > 0 {
		p.discard = append(p.discard, list...)
		// TODO: Race condition: what if player shuffles discards into deck
		// while another is trying look at the top discard?
		game.Report(Event{s: "discard", n: p.n, i: len(list)})
	}
}

func (game *Game) Cost(c *Card) int {
	n := c.cost - game.discount
	if n < 0 {
		return 0
	}
	return n
}

func (game *Game) dump() {
	cols := []int{3, 3, 1, 3, 3, 3, 1}
	for _, c := range game.suplist {
		fmt.Printf("  [%c] %v(%v) $%v", c.key, c.name, c.supply, game.Cost(c))
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
			fmt.Printf(":%v", game.GetDiscard(game, p))
		}
		fmt.Println()
	}
}

func (p *Player) InitDeck() {
	for i := 0; i < 3; i++ {
		p.manifest.Add("Estate")
	}
	for i := 0; i < 7; i++ {
		p.manifest.Add("Copper")
	}
}

func (p *Player) dumpHand() {
	fmt.Println("Hand:")
	n := 0
	for _, c := range p.hand {
		fmt.Printf(" [%c] %v", c.key, c.name)
		n = (n + 1) % 5
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
		fmt.Println("")
	}
}

func (game *Game) keyToCard(key byte) *Card {
	for _, c := range game.suplist {
		if key == c.key {
			return c
		}
	}
	return nil
}

func (game *Game) MultiPlay(p *Player, c *Card, m int) {
	p.played = append(p.played, c)
	if isAction(c) {
		game.aCount++
	}
	for ; m > 0; m-- {
		fmt.Printf("%v plays %v\n", p.name, c.name)
		if c.act == nil {
			fmt.Printf("%v unimplemented  :(\n", c.name)
			return
		}
		game.stack = append(game.stack, &Frame{card: c})
		for _, f := range c.act {
			f(game)
		}
		game.stack = game.stack[:len(game.stack)-1]
	}
}

func (game *Game) Play(c *Card) {
	p := game.NowPlaying()
	var k int
	for k = len(p.hand) - 1; k >= 0; k-- {
		if p.hand[k] == c {
			p.hand = append(p.hand[:k], p.hand[k+1:]...)
			break
		}
	}
	if k < 0 {
		panic("unplayable")
	}
	if isAction(c) {
		game.a--
	}
	game.MultiPlay(p, c, 1)
}

func (game *Game) Spend(c *Card) {
	game.c -= game.Cost(c)
	game.b--
}

func (game *Game) NowPlaying() *Player { return game.players[game.n] }
func (game *Game) addCoins(n int)      { game.c += n }
func (game *Game) addActions(n int)    { game.a += n }
func (game *Game) addBuys(n int)       { game.b += n }
func (game *Game) addCards(n int)      { game.draw(game.NowPlaying(), n) }

func (game *Game) SetParse(fun func(b byte) (Command, string)) {
	game.stack[len(game.stack)-1].Parse = fun
}

func (game *Game) StackTop() *Frame {
	n := len(game.stack)
	if n == 0 {
		return nil
	}
	return game.stack[n-1]
}

type Frame struct {
	Parse func(b byte) (Command, string)
	card  *Card
}

type Command struct {
	s string
	c *Card
	i int
}

type PlayFun interface {
	start(*Game, *Player)
}

type Player struct {
	name                                  string
	n                                     int
	fun                                   PlayFun
	manifest, deck, hand, played, discard Pile
	trigger                               chan bool   // When triggered, Player sends a Command on game.ch.
	recv                                  chan string // For sending decisions to remote clients.
	herald                                chan Event  // Events that may be worth printing.
}

type Event struct {
	s    string
	card *Card
	n    int
	i    int
	cmd  string
}

// MaybeShuffle returns true if deck is non-empty, shuffling the discards
// into a new deck if necessary.
func (p *Player) MaybeShuffle() bool {
	if len(p.deck) == 0 {
		if len(p.discard) == 0 {
			return false
		}
		p.deck, p.discard = p.discard, nil
		p.deck.shuffle()
	}
	return true
}

func (game *Game) Report(ev Event) {
	for _, other := range game.players {
		if other.herald != nil {
			other.herald <- ev
		}
	}
}

func (game *Game) draw(p *Player, n int) int {
	count := 0
	if game.isServer {
		s := ""
		sSecret := ""
		i := 0
		for ; i < n && p.MaybeShuffle(); i++ {
			c := p.deck[0]
			p.deck, p.hand = p.deck[1:], append(p.hand, c)
			s += string(c.key)
			sSecret += "?"
		}
		game.castCond(func(x *Player) bool { return x == p }, "draw", s)
		game.castCond(func(x *Player) bool { return x != p }, "draw", sSecret)
		count = i
	} else {
		w := game.fetch()
		for _, b := range []byte(w[0]) {
			if len(p.deck) == 0 {
				p.deck, p.discard = p.discard, nil
			}
			p.deck = p.deck[1:]
			if b != '?' {
				p.hand = append(p.hand, game.keyToCard(b))
			} else {
				p.hand = append(p.hand, nil)
			}
		}
		count = len(w[0])
	}
	game.Report(Event{s: "draw", n: p.n, i: count})
	return count
}

func (game *Game) reveal(p *Player) *Card {
	if !p.MaybeShuffle() {
		log.Fatalf("should check for empty deck before reveal")
	}
	if game.isServer {
		c := p.deck[0]
		fmt.Printf("%v reveals %v\n", p.name, c.name)
		game.cast("reveal", c)
		return c
	}
	c := game.keyToCard(game.fetch()[0][0])
	fmt.Printf("%v reveals %v\n", p.name, c.name)
	return c
}

func (game *Game) revealHand(p *Player) {
	for i, c := range p.hand {
		if game.isServer {
			game.cast("revealHand", c)
		} else {
			p.hand[i] = game.keyToCard(game.fetch()[0][0])
		}
	}
	for _, c := range p.hand {
		fmt.Printf("%v reveals %v\n", p.name, c.name)
	}
}

func (game *Game) Cleanup(p *Player) {
	p.discard, p.played = append(p.discard, p.played...), nil
	p.discard, p.hand = append(p.discard, p.hand...), nil
}

func (game *Game) cast(comment string, vs ...interface{}) {
	game.castCond(func(*Player) bool { return true }, comment, vs...)
}

func (game *Game) castCond(cond func(*Player) bool, comment string, vs ...interface{}) {
	if !game.isServer {
		log.Fatal("nonserver cast")
	}
	s := comment
	for _, v := range vs {
		switch t := v.(type) {
		default:
			s += fmt.Sprintf(";%v", t)
		case *Card:
			s += fmt.Sprintf(";%c", t.key)
		}
	}
	for _, p := range game.players {
		if cond(p) && p.recv != nil {
			p.recv <- s
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

func (game *Game) CanPlay(p *Player, c *Card) string {
	found := false
	for i := len(p.hand) - 1; i >= 0; i-- {
		if p.hand[i] == nil || p.hand[i] == c {
			p.hand[i] = c
			found = true
			break
		}
	}
	if !found {
		return "none in hand"
	}
	switch {
	case isAction(c):
		if game.phase != phAction {
			return "wrong phase"
		}
		if game.a == 0 {
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
	switch {
	case game.phase != phBuy:
		return "wrong phase"
	case game.b == 0:
		return "no buys left"
	case game.Cost(c) > game.c:
		return "insufficient money"
	case c.supply == 0:
		return "supply exhausted"
	}
	return ""
}

func (game *Game) Over() {
	fmt.Printf("Game over\n")
	for i, p := range game.players {
		game.n = i // We want NowPlaying() for some VP computations.
		score := 0
		m := make(map[*Card]struct {
			count, pts int
		})
		for _, c := range p.manifest {
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
	// TODO: Notify clients that game is over instead of this.
	time.Sleep(1 * time.Second)
}

func (game *Game) getCommand(p *Player) Command {
	p.trigger <- true
	cmd := <-game.ch
	game.sendCmd(game, p, &cmd)
	if cmd.s == "quit" {
		game.Over()
		os.Exit(0)
	}
	return cmd
}

type pickOpts struct {
	n     int
	exact bool
	cond  func(*Card) string
}

func (game *Game) pickHand(p *Player, o pickOpts) Pile {
	var selected Pile
	selected, p.hand = game.split(p.hand, p, o)
	return selected
}

func (game *Game) split(list Pile, p *Player, o pickOpts) (Pile, Pile) {
	n := o.n
	var in Pile
	out := append(list)
	// Forced choices.
	if game.isServer {
		max := 0
		var prev *Card
		same := true
		for _, c := range out {
			if o.cond == nil || o.cond(c) == "" {
				max++
				if prev == nil {
					prev = c
				} else if prev != c {
					same = false
				}
			}
		}
		if n > max {
			n = max
		}
		if same && o.exact {
			log.Print("TODO: automate forced choice")
		}
		game.cast("max", n)
	} else {
		n = PanickyAtoi(game.fetch()[0])
	}
	if n == 0 {
		return in, out
	}
	game.SetParse(func(b byte) (Command, string) {
		if b == '.' {
			if o.exact {
				return errCmd, "must pick a card"
			}
			return Command{s: "done"}, ""
		}
		choice := game.keyToCard(b)
		if choice == nil {
			return errCmd, "unrecognized card"
		}
		found := false
		for _, c := range out {
			if c == choice {
				found = true
				break
			}
		}
		if !found {
			return errCmd, "invalid choice"
		}
		if o.cond != nil {
			if msg := o.cond(choice); msg != "" {
				return errCmd, msg
			}
		}
		return Command{s: "pick", c: choice}, ""
	})
	for stop := false; !stop; {
		cmd := game.getCommand(p)
		switch cmd.s {
		case "pick":
			found := false
			for i, c := range out {
				// nil represents unknown cards.
				if c == nil || c == cmd.c {
					in = append(in, cmd.c)
					out = append(out[:i], out[i+1:]...)
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
			if o.exact && n > 0 {
				panic("must pick more")
			}
			stop = true
		default:
			panic("bad command: " + cmd.s)
		}
	}
	return in, out
}

func (game *Game) Gain(p *Player, c *Card) {
	if c.supply == 0 {
		panic("out of supply")
	}
	game.Report(Event{s: "gain", n: p.n, card: c})
	p.discard = append(p.discard, c)
	c.supply--
}

func (game *Game) MaybeGain(p *Player, c *Card) bool {
	if c.supply == 0 {
		return false
	}
	game.Gain(p, c)
	return true
}

func pickGainCond(game *Game, max int, fun func(*Card) string) *Card {
	game.SetParse(func(b byte) (Command, string) {
		c := game.keyToCard(b)
		switch {
		case c == nil:
			return errCmd, "expected card"
		case game.Cost(c) > max:
			return errCmd, "too expensive"
		case c.supply == 0:
			return errCmd, "supply exhausted"
		case fun != nil:
			if msg := fun(c); msg != "" {
				return errCmd, msg
			}
		}
		return Command{s: "pick", c: c}, ""
	})
	p := game.NowPlaying()
	cmd := game.getCommand(p)
	if cmd.s != "pick" {
		panic("bad command: " + cmd.s)
	}
	if game.Cost(cmd.c) > max {
		panic("too expensive")
	}
	game.Gain(p, cmd.c)
	return cmd.c
}

func pickGain(game *Game, max int) *Card { return pickGainCond(game, max, nil) }

var errCmd Command

func (p *Player) inHand(cond func(*Card) bool) bool {
	for _, c := range p.hand {
		if cond(c) {
			return true
		}
	}
	return false
}

func (game *Game) inHand(p *Player, cond func(*Card) bool) bool {
	if !game.isServer {
		return game.fetch()[0] == "1"
	}
	res := p.inHand(cond)
	game.cast("inhand", func() int {
		if res {
			return 1
		}
		return 0
	}())
	return res
}

func reacts(game *Game, p *Player) bool {
	if !game.inHand(p, func(c *Card) bool { return c.name == "Moat" }) {
		return false
	}
	var selected Pile
	selected, _ = game.split(p.hand, p, pickOpts{n: 1, cond: func(c *Card) string {
		if c.name != "Moat" {
			return "pick Moat or nothing"
		}
		return ""
	}})
	if len(selected) == 0 {
		return false
	}
	c := selected[0]
	if c.name != "Moat" {
		panic("invalid attack reaction: " + c.name)
	}
	fmt.Printf("%v reveals %v\n", p.name, c.name)
	return true
}

func (game *Game) ForOthers(fun func(*Player)) {
	m := len(game.players)
	for i := (game.n + 1) % m; i != game.n; i = (i + 1) % m {
		fun(game.players[i])
	}
}

func (game *Game) attack(fun func(*Player)) {
	m := len(game.players)
	for i := (game.n + 1) % m; i != game.n; i = (i + 1) % m {
		other := game.players[i]
		fmt.Printf("%v attacks %v\n", game.NowPlaying().name, other.name)
		if reacts(game, other) {
			continue
		}
		fun(other)
	}
}

func (game *Game) getInts(p *Player, menu string, n int) []int {
	v := strings.Split(menu, ";")
	for i, s := range v {
		v[i] = strings.TrimSpace(s)
		fmt.Printf("[%d] %v\n", i+1, v[i])
	}
	var selection []int
	game.SetParse(func(b byte) (Command, string) {
		if b < '1' || b > '0' + byte(len(v)) {
			i := int(b - '0')
			log.Printf("%d", i)
			for _, x := range selection {
				if x == i {
					return errCmd, "already chosen " + string(b)
				}
			}
			return errCmd, "enter digit within range"
		}
		return Command{s: string(b)}, ""
	})
	for len(selection) < n {
		cmd := game.getCommand(p)
		selection = append(selection, int(cmd.s[0] - '0'))
	}
	return selection
}

func (game *Game) getBool(p *Player) bool {
	game.SetParse(func(b byte) (Command, string) {
		switch b {
		case '\\':
			return Command{s: "yes"}, ""
		case '.':
			return Command{s: "done"}, ""
		}
		return errCmd, "\\ for yes, . for no"
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

type CardDB struct {
	List string
	Fun  map[string]func(game *Game)
	VP   map[string]func(game *Game) int
}

func loadDB(db CardDB) {
	for _, s := range strings.Split(db.List, "\n") {
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
		cost, err := strconv.Atoi(a[1])
		if err != nil {
			panic(s)
		}
		c := &Card{name: a[0], cost: cost}
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
		if fun, ok := db.Fun[c.name]; ok {
			add(fun)
		}
		if fun, ok := db.VP[c.name]; ok {
			c.vp = fun
		}
	}
}

func main() {
	for _, s := range []string{"Treasure", "Victory", "Curse", "Action", "Attack", "Reaction"} {
		KindDict[s] = &Kind{s}
	}
	kTreasure = getKind("Treasure")
	kVictory = getKind("Victory")
	kCurse = getKind("Curse")
	kAction = getKind("Action")
	loadDB(cardsBase)
	loadDB(cardsIntrigue)

	log.SetFlags(log.Lshortfile)
	flag.Parse()
	if flag.NArg() > 0 {
		client(flag.Arg(0))
		return
	}

	rand.Seed(time.Now().Unix())
	fmt.Println("= Gominion =")
	game := &Game{ch: make(chan Command), isServer: true,
		sendCmd: func(game *Game, p *Player, cmd *Command) {
			if cmd.c == nil {
				game.cast("cmd", cmd.s)
			} else {
				game.cast("cmd", cmd.s, cmd.c)
			}
		},
	}
	ng := netGamer{
		in:  make(chan Command),
		out: make(chan string),
	}
	game.players = []*Player{
		//&Player{name: "Anonymous", fun: ng},
		&Player{name: "Ben", fun: consoleGamer{}},
		&Player{name: "AI", fun: SimpleBuyer{[]string{"Province", "Gold", "Silver"}}},
	}
	players := game.players

	http.HandleFunc("/discard", func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintf(w, game.GetDiscard(game, game.players[PanickyAtoi(r.FormValue("n"))]))
	})

	http.HandleFunc("/poll", func(w http.ResponseWriter, r *http.Request) {
		ng.in <- Command{s: "poll"}
		fmt.Fprintf(w, <-ng.out)
	})

	http.HandleFunc("/cmd", func(w http.ResponseWriter, r *http.Request) {
		s := r.FormValue("s")
		if s == "" {
			fmt.Fprintf(w, "error: no command")
			return
		}
		cmd := Command{s: s}
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
	for n := 0; ; n++ {
		resp, err := http.Get("http://:8080/")
		if err != nil {
			if n > 3 {
				log.Fatal("failed to connect 3 times: ", err)
			}
			time.Sleep(1 * time.Second)
			continue
		}
		defer resp.Body.Close()
		break
	}

	setSupply := func(s string, n int) {
		c, ok := CardDict[s]
		if !ok {
			panic("no such card: " + s)
		}
		c.supply = n
	}
	setSupply("Copper", 60-7*len(players))
	setSupply("Silver", 40)
	setSupply("Gold", 30)

	numVictoryCards := func(n int) int {
		switch n {
		case 2:
			return 8
		case 3:
			return 12
		case 4:
			return 12
		}
		panic(n)
	}
	for _, s := range []string{"Estate", "Duchy", "Province"} {
		setSupply(s, numVictoryCards(len(players)))
	}
	setSupply("Curse", 10*(len(players)-1))
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
		name  string
		cards Pile
	}
	var presets []Preset
	for _, line := range strings.Split(`
TEST:Courtyard,Pawn,Shanty Town,Steward,Minion,Harem,Nobles,Village,Woodcutter,Workshop
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
		pr := Preset{name: s[0]}
		for _, s := range strings.Split(s[1], ",") {
			c := GetCard(s)
			// Insertion sort.
			pr.cards = func(cards Pile) Pile {
				for i, x := range cards {
					if x.cost == c.cost && x.name > c.name || x.cost > c.cost {
						return append(cards[:i], append(Pile{c}, cards[i:]...)...)
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
	fmt.Println()
	//pr := presets[rand.Intn(len(presets))]
	pr := presets[0]
	fmt.Printf("Playing %q\n", pr.name)
	for i, c := range pr.cards {
		c.supply = 10
		layout(c.name, keys[i])
	}
	for i, p := range players {
		p.InitDeck()
		p.deck = append(p.deck, p.manifest...)
		p.deck.shuffle()
		p.hand, p.deck = p.deck[:5], p.deck[5:]
		p.trigger = make(chan bool)
		p.n = i
		go p.fun.start(game, p)
	}
	game.GetDiscard = func(game *Game, p *Player) string {
		if len(p.discard) == 0 {
			log.Print("BUG ", p.name)
			return "BUG!"
		}
		return p.discard[len(p.discard)-1].name
	}
	game.mainloop()
}

func (game *Game) mainloop() {
	for game.n = 0; ; game.n = (game.n + 1) % len(game.players) {
		game.a, game.b, game.c = 1, 1, 0
		game.discount = 0
		game.copperbonus = 0
		game.aCount = 0
		p := game.NowPlaying()
		prev := phCleanup
		for game.phase = phAction; game.phase <= phCleanup; {
			if prev != game.phase {
				game.Report(Event{s: "phase"})
				prev = game.phase
			}
			if game.phase == phAction && game.a == 0 || game.phase == phBuy && game.b == 0 || game.phase == phCleanup {
				game.phase++
				continue
			}
			cmd := game.getCommand(p)
			switch cmd.s {
			case "buy":
				choice := cmd.c
				if err := CanBuy(game, choice); err != "" {
					panic(err)
				}
				fmt.Printf("%v buys %v for $%v\n", game.players[game.n].name, choice.name, game.Cost(choice))
				game.Spend(choice)
				game.Gain(p, choice)
			case "play":
				if err := game.CanPlay(p, cmd.c); err != "" {
					panic(err)
				}
				game.Play(cmd.c)
			case "next":
				game.phase++
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

type consoleGamer struct{}

func (consoleGamer) start(game *Game, p *Player) {
	p.herald = make(chan Event)
	game.dump()
	reader := bufio.NewReader(os.Stdin)
	i := 0
	prog := ""
	wildCard := false
	buyMode := false
	for {
		select {
		case ev := <-p.herald:
			x := game.players[ev.n]
			switch ev.s {
			case "discard":
				if ev.i == 0 {
					log.Print("BUG: reported 0 discards")
				}
				fmt.Printf("%v discards %v cards (%v)\n", x.name, ev.i, game.GetDiscard(game, x))
			case "discarddeck":
				fmt.Printf("%v discards deck; %v cards (%v)\n", x.name, ev.i, game.GetDiscard(game, x))
			case "gain":
				fmt.Printf("%v gains %v\n", x.name, ev.card.name)
				x.manifest = append(x.manifest, ev.card)
			case "trash":
				fmt.Printf("%v trashes %v\n", x.name, ev.card.name)
				for i, c := range x.manifest {
					if c == ev.card {
						x.manifest = append(x.manifest[:i], x.manifest[i+1:]...)
						break
					}
				}
			case "phase":
				if x == p && game.phase == phAction {
					p.dumpHand()
				}
			case "draw":
				if x != p {
					fmt.Printf("%v draws %v cards\n", x.name, ev.i)
				} else {
					for i := ev.i; i > 0; i-- {
						c := p.hand[len(p.hand)-i]
						fmt.Printf("%v draws [%c] %v\n", p.name, c.key, c.name)
					}
				}
			}
		case <-p.trigger:
			game.ch <- func() Command {
				frame := game.StackTop()
				if frame == nil {
					// Automatically advance to next phase when it's obvious.
					if game.phase == phAction && !p.inHand(isAction) {
						return Command{s: "next"}
					}
					if game.phase != phBuy {
						buyMode = false
					} else if !p.inHand(isTreasure) {
						buyMode = true
					}
				}

				for {
					if wildCard {
						if game.phase == phBuy {
							for k := len(p.hand) - 1; k >= 0; k-- {
								if isTreasure(p.hand[k]) {
									return Command{s: "play", c: p.hand[k]}
								}
							}
						}
						wildCard = false
					}
					i++
					for i >= len(prog) {
						fmt.Printf("a:%v b:%v c:%v %v", game.a, game.b, game.c, p.name)
						if frame != nil {
							fmt.Printf(" %v", frame.card.name)
						}
						fmt.Printf("> ")
						s, err := reader.ReadString('\n')
						if err == io.EOF {
							fmt.Printf("\nQuitting game...\n")
							return Command{s: "quit"}
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
					if match {
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
						case '+':
							fallthrough
						case ';':
							if game.phase != phBuy {
								msg = "wrong phase"
								break
							}
							if p.inHand(isTreasure) {
								buyMode = !buyMode
							}
						case '.':
							return Command{s: "next"}
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
							if buyMode {
								if msg = CanBuy(game, c); msg != "" {
									break
								}
								return Command{s: "buy", c: c}
							}
							if msg = game.CanPlay(p, c); msg != "" {
								break
							}
							return Command{s: "play", c: c}
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

type netGamer struct {
	in  chan Command
	out chan string
}

func (this netGamer) start(game *Game, p *Player) {
	p.recv = make(chan string)
	var q []string
	fresh := true
	ready := false
	for {
		select {
		case s := <-p.recv:
			q = append(q, s)
		case <-p.trigger:
			if ready {
				log.Fatal("already ready")
			}
			q = append(q, "go")
			ready = true
		case cmd := <-this.in:
			if fresh {
				fresh = false
				this.out <- "new\n= Players =\n" + encodePlayers(game.players) + "= Kingdom =\n" + encodeKingdom(game) + "= Hand =\n" + encodeHand(p)
				break
			}
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
				s := q[0]
				q = q[1:]
				this.out <- s
			default:
				if !ready {
					log.Fatal("breach of protocol: " + cmd.s)
				} else {
					ready = false
					game.ch <- cmd
					this.out <- "sent"
				}
			}
		}
	}
}

func client(host string) {
	host = "http://" + host + "/"
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
	next := func() []string {
		for {
			body := send(host + "poll")
			if body == "wait" {
				time.Sleep(200 * time.Millisecond)
				continue
			}
			v := strings.Split(body, ";")
			if len(v) == 0 {
				log.Printf("malformed response: %q", body)
				continue
			}
			return v
		}
		panic("unreachable")
	}
	v := strings.Split(next()[0], "\n")
	if v[0] != "new" {
		log.Fatalf("want 'new', got %q", v[0])
	}
	var p *Player
	game := &Game{
		ch: make(chan Command),
		sendCmd: func(game *Game, other *Player, cmd *Command) {
			if p != other {
				return
			}
			v = next()
			if v[0] != "go" {
				log.Fatalf("want 'go', got %q", v[0])
			}
			u := host + "cmd?s=" + cmd.s
			if cmd.c != nil {
				u += "&c=" + string(cmd.c.key)
			}
			send(u)
			confirm := game.fetch()
			if confirm[0] != cmd.s {
				log.Fatalf("want %q, got %q", cmd.s, confirm[0])
			}
			if len(confirm) == 2 && confirm[1] != string(cmd.c.key) {
				log.Fatalf("want %q, got %q", string(cmd.c.key), confirm[1])
			}
		}, GetDiscard: func(game *Game, p *Player) string {
			return send(fmt.Sprintf("%vdiscard?n=%v", host, p.n))
		}}
	game.fetch = func() []string {
		return next()[1:]
	}

	sharedTrigger := make(chan bool)
	go func() {
		for {
			<-sharedTrigger
			w := game.fetch()
			switch len(w) {
			default:
				log.Fatal("bad command ", w)
			case 1:
				game.ch <- Command{s: w[0]}
			case 2:
				game.ch <- Command{s: w[0], c: game.keyToCard(w[1][0])}
			}
		}
	}()
	p = &Player{name: "Anonymous", fun: consoleGamer{}, trigger: make(chan bool)}
	heading := ""
	pn := 0
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
			var x *Player
			if line == p.name {
				x = p
			} else {
				x = &Player{name: line, trigger: sharedTrigger}
				x.hand = make(Pile, 5, 5)
			}
			game.players = append(game.players, x)
			x.n = pn
			x.InitDeck()
			x.deck = make(Pile, len(x.manifest)-5, len(x.manifest))
			pn++
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
	game.mainloop()
}
