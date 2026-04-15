package game_test

import (
	"io"
	"log/slog"
	"testing"

	"github.com/Minalinnski/RonTama/internal/ai/easy"
	"github.com/Minalinnski/RonTama/internal/game"
	"github.com/Minalinnski/RonTama/internal/rules/sichuan"
)

func TestRunRound_Sichuan_NoPanic(t *testing.T) {
	rule := sichuan.New()
	players := [game.NumPlayers]game.Player{
		easy.New("a"), easy.New("b"), easy.New("c"), easy.New("d"),
	}
	silent := slog.New(slog.NewTextHandler(io.Discard, nil))
	for i := 0; i < 20; i++ {
		_, err := game.RunRound(rule, players, i%4, silent)
		if err != nil {
			t.Fatalf("round %d: %v", i, err)
		}
	}
}

func TestRunRound_BloodBattle_AtMostThreeWins(t *testing.T) {
	rule := sichuan.New()
	players := [game.NumPlayers]game.Player{
		easy.New("a"), easy.New("b"), easy.New("c"), easy.New("d"),
	}
	silent := slog.New(slog.NewTextHandler(io.Discard, nil))
	for i := 0; i < 50; i++ {
		res, err := game.RunRound(rule, players, 0, silent)
		if err != nil {
			t.Fatal(err)
		}
		if len(res.Wins) > 3 {
			t.Errorf("round %d: %d wins, blood-battle caps at 3", i, len(res.Wins))
		}
	}
}

func TestRunRound_ScoresZeroSum(t *testing.T) {
	rule := sichuan.New()
	players := [game.NumPlayers]game.Player{
		easy.New("a"), easy.New("b"), easy.New("c"), easy.New("d"),
	}
	silent := slog.New(slog.NewTextHandler(io.Discard, nil))
	for i := 0; i < 30; i++ {
		res, err := game.RunRound(rule, players, i%4, silent)
		if err != nil {
			t.Fatal(err)
		}
		sum := 0
		for _, s := range res.FinalScores {
			sum += s
		}
		if sum != 0 {
			t.Errorf("round %d scores not zero-sum: %v (sum=%d)", i, res.FinalScores, sum)
		}
	}
}
