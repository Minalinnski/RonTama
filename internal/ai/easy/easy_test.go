package easy_test

import (
	"testing"

	"github.com/Minalinnski/RonTama/internal/ai/easy"
	"github.com/Minalinnski/RonTama/internal/game"
	"github.com/Minalinnski/RonTama/internal/rules/sichuan"
	"github.com/Minalinnski/RonTama/internal/tile"
)

func TestEasy_DiscardsDingqueFirst(t *testing.T) {
	bot := easy.New("test")
	view := game.PlayerView{
		Rule:    sichuan.New(),
		Seat:    0,
		OwnHand: tile.NewHand(tile.MustParseHand("123m 456p 789s 11p 1s")),
		Dingque: [game.NumPlayers]tile.Suit{tile.SuitSou, tile.SuitMan, tile.SuitMan, tile.SuitMan},
	}
	view.JustDrew = nil
	action := bot.OnDraw(view)
	if action.Kind != game.DrawDiscard {
		t.Fatalf("expected DrawDiscard, got %v", action.Kind)
	}
	if action.Discard.Suit() != tile.SuitSou {
		t.Errorf("expected sou-suit discard (dingque), got %s", action.Discard)
	}
}

func TestEasy_ChoosesDingqueLeast(t *testing.T) {
	bot := easy.New("test")
	view := game.PlayerView{
		Rule: sichuan.New(),
		Seat: 0,
		// Hand: 6m + 4p + 3s -> sou is least, should pick sou.
		OwnHand: tile.NewHand(tile.MustParseHand("123456m 1234p 123s")),
	}
	dq := bot.ChooseDingque(view)
	if dq != tile.SuitSou {
		t.Errorf("expected SuitSou (fewest), got %d", dq)
	}
}
