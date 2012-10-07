package main

import "testing"

func TestCoppersmith(t *testing.T) {
	players := Setup(t, `
= Alice =
hand:Coppersmith,Copper,Silver,Gold,Copper
`)
	game := &Game{ch: make(chan Command), isServer: true,
		sendCmd:    func(game *Game, p *Player, cmd *Command) {},
		GetDiscard: func(game *Game, p *Player) string { return p.discard[len(p.discard)-1].name },
		players:    players,
	}
	game.NewGame()
	game.StartTurn(0)
	game.phase = phAction
	game.Play(GetCard("Coppersmith"))
	game.phase = phBuy
	game.Play(GetCard("Copper"))
	game.Play(GetCard("Copper"))
	if game.c != 4 {
		t.Errorf("want %v, got %v", 4, game.c)
	}
	game.Play(GetCard("Silver"))
	game.Play(GetCard("Gold"))
	if game.c != 9 {
		t.Errorf("want %v, got %v", 9, game.c)
	}
}
