package main

import (
	"fmt"
	"io/ioutil"
	"log"
	"math/rand"
	"net/http"
	"regexp"
	"strings"
	"time"
)

func client(host string) {
	p := &Player{fun: consoleGamer{}, herald: make(chan Event), trigger: make(chan bool)}
	rand.Seed(time.Now().Unix())
	for i := 0; i < 3; i++ {
		p.name = p.name + string('A'+rand.Intn(26))
	}
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
			body := send(host + "poll?id=" + p.name)
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
	if err := send(host + "reg?name=" + p.name); err != p.name {
		log.Fatalf("registration: " + err)
	}

	game := &Game{
		ch: make(chan Command),
		sendCmd: func(game *Game, other *Player, cmd *Command) {
			if p != other {
				return
			}
			v := next()
			if v[0] != "go" {
				log.Fatalf("want 'go', got %q", v[0])
			}
			u := host + "cmd?id=" + p.name + "&s=" + cmd.s
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
		},
		fetch: func() []string { return next()[1:] },
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
	go p.fun.start(game, p)

	for {
		game.Reset()
		game.players = nil
		p.hand = nil
		p.discard = nil
		heading := ""
		pn := 0
		var v []string
		for {
			v = strings.Split(next()[0], "\n")
			if v[0] == "new" {
				break
			}
		}
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
		game.dump()
		game.mainloop()
	}
}
