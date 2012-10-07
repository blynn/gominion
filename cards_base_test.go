package main

import "testing"

func TestBureaucratWitchMoat(t *testing.T) {
	players := Setup(t, `
= Alice =
hand:Bureaucrat
deck:Copper
= Bob =
hand:Province,Duchy,Estate
deck:Copper
discard:Moat
= Carol =
hand:Silver,Copper,Witch
deck:Copper
discard:Gold
= Dave =
hand:Copper,Estate,Estate,Estate
= Eve =
hand:Moat
`)
	game := &Game{ch: make(chan Command), isServer: true,
		sendCmd:    func(game *Game, p *Player, cmd *Command) {},
		GetDiscard: func(game *Game, p *Player) string { return p.discard[len(p.discard)-1].name },
		players:    players,
	}
	game.NewGame()
	// Alice plays Bureaucrat.
	GetCard("Silver").supply = 8
	game.StartTurn(0)
	game.phase = phAction
	done := make(chan bool)
	go func() {
		// Bob chooses to deck a Duchy.
		<-players[1].trigger
		game.ch <- Command{s: "pick", c: GetCard("Duchy")}
		// Carol: no Victory cards, so she reveals hand.
		// Dave: choice is forced (Estate).
		// Eve reveals Moat to stop attack.
		<-players[4].trigger
		game.ch <- Command{s: "pick", c: GetCard("Moat")}
		// One can reveal the same Reaction multiple times.
		<-players[4].trigger
		game.ch <- Command{s: "pick", c: GetCard("Moat")}
		<-players[4].trigger
		game.ch <- Command{s: "done"}
		done <- true
	}()
	game.Play(GetCard("Bureaucrat"))
	<-done
	CheckPiles(t, players, `
= Alice =
played:Bureaucrat
deck:Silver,Copper
= Bob =
hand:Province,Estate
deck:Duchy,Copper
discard:Moat
= Carol =
hand:Silver,Copper,Witch
deck:Copper
discard:Gold
= Dave =
hand:Copper,Estate,Estate
deck:Estate
= Eve =
hand:Moat
`)
	// Carol plays Witch.
	GetCard("Curse").supply = 3
	game.p = players[2]
	go func() {
		// Eve abstains from revealing Moat(!)
		<-players[4].trigger
		game.ch <- Command{s: "done"}
		done <- true
	}()
	game.Play(GetCard("Witch"))
	<-done
	// Curses are given starting from Carol's left, until they run out.
	CheckPiles(t, players, `
= Alice =
played:Bureaucrat
deck:Silver,Copper
discard:Curse
= Bob =
hand:Province,Estate
deck:Duchy,Copper
discard:Moat
= Carol =
hand:Silver,Copper,Copper,Gold
played:Witch
deck:
discard:
= Dave =
hand:Copper,Estate,Estate
deck:Estate
discard:Curse
= Eve =
hand:Moat
discard:Curse
`)
}

func TestThroneRoomFeast(t *testing.T) {
	players := Setup(t, `
= Alice =
hand:Throne Room,Throne Room,Feast,Feast,Spy
`)
	game := &Game{ch: make(chan Command), isServer: true,
		sendCmd:    func(game *Game, p *Player, cmd *Command) {},
		GetDiscard: func(game *Game, p *Player) string { return p.discard[len(p.discard)-1].name },
		players:    players,
	}
	game.NewGame()
	// Alice plays Throne Room -> Throne Room -> Feast, Feast.
	game.StartTurn(0)
	game.phase = phAction
	done := make(chan bool)
	game.suplist = append(game.suplist, GetCard("Village"), GetCard("Militia"), GetCard("Market"), GetCard("Adventurer"))
	GetCard("Village").supply = 1
	GetCard("Militia").supply = 1
	GetCard("Market").supply = 10
	GetCard("Adventurer").supply = 10
	go func() {
		<-players[0].trigger
		game.ch <- Command{s: "pick", c: GetCard("Throne Room")}
		<-players[0].trigger
		game.ch <- Command{s: "pick", c: GetCard("Feast")}
		<-players[0].trigger
		game.ch <- Command{s: "pick", c: GetCard("Market")}
		<-players[0].trigger
		game.ch <- Command{s: "pick", c: GetCard("Village")}
		<-players[0].trigger
		game.ch <- Command{s: "pick", c: GetCard("Feast")}
		<-players[0].trigger
		game.ch <- Command{s: "pick", c: GetCard("Militia")}
		// No more choices. The last Feast must be used to gain a Market, the only
		// remaining card costing 5 or less.
		done <- true
	}()
	game.Play(GetCard("Throne Room"))
	<-done
	CheckPiles(t, players, `
= Alice =
hand:Spy
deck:
played:Throne Room,Throne Room
discard:Market,Village,Militia,Market
`)
	if msg := ComparePiles(game.trash, ParsePile("Feast,Feast")); msg != "" {
		t.Errorf(msg)
	}
}
