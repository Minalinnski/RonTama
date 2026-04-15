package riichi_test

import (
	"io"
	"log/slog"
	"testing"

	"github.com/Minalinnski/RonTama/internal/ai/easy"
	"github.com/Minalinnski/RonTama/internal/game"
	"github.com/Minalinnski/RonTama/internal/rules/riichi"
)

// TestRiichi_FourBots_NoPanic runs a few rounds of Riichi with 4 Easy
// bots and verifies the round completes (likely with wall exhaustion
// since Easy bots don't declare riichi → wins are rare without
// natural yaku).
func TestRiichi_FourBots_NoPanic(t *testing.T) {
	rule := riichi.New()
	players := [game.NumPlayers]game.Player{
		easy.New("a"), easy.New("b"), easy.New("c"), easy.New("d"),
	}
	silent := slog.New(slog.NewTextHandler(io.Discard, nil))
	for i := 0; i < 5; i++ {
		_, err := game.RunRound(rule, players, i%4, silent)
		if err != nil {
			t.Fatalf("round %d: %v", i, err)
		}
	}
}
