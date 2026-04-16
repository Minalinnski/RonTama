package match_test

import (
	"io"
	"log/slog"
	"testing"

	"github.com/Minalinnski/RonTama/internal/ai/easy"
	"github.com/Minalinnski/RonTama/internal/game"
	"github.com/Minalinnski/RonTama/internal/match"
	"github.com/Minalinnski/RonTama/internal/rules/riichi"
	"github.com/Minalinnski/RonTama/internal/rules/sichuan"
)

func TestRunMatch_Sichuan_SingleRound(t *testing.T) {
	rule := sichuan.New()
	players := [game.NumPlayers]game.Player{
		easy.New("a"), easy.New("b"), easy.New("c"), easy.New("d"),
	}
	silent := slog.New(slog.NewTextHandler(io.Discard, nil))
	res, err := match.RunMatch(match.Config{
		Rule:      rule,
		Players:   players,
		MaxRounds: 1,
	}, silent, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(res.Rounds) != 1 {
		t.Errorf("expected 1 round, got %d", len(res.Rounds))
	}
}

func TestRunMatch_Riichi_FourRounds(t *testing.T) {
	rule := riichi.New()
	players := [game.NumPlayers]game.Player{
		easy.New("a"), easy.New("b"), easy.New("c"), easy.New("d"),
	}
	silent := slog.New(slog.NewTextHandler(io.Discard, nil))
	res, err := match.RunMatch(match.Config{
		Rule:      rule,
		Players:   players,
		MaxRounds: 4,
		Renchan:   true,
	}, silent, nil)
	if err != nil {
		t.Fatalf("match: %v", err)
	}
	if len(res.Rounds) != 4 {
		t.Errorf("expected 4 rounds, got %d", len(res.Rounds))
	}
	// Scores should carry across rounds — total should remain 100000
	// (Riichi starting 25000 × 4), possibly minus lingering riichi pot.
	total := 0
	for _, s := range res.FinalScores {
		total += s
	}
	start := 4 * rule.StartingScore()
	if total > start || total < start-20000 {
		t.Errorf("final score sum %d implausible vs start %d", total, start)
	}
}

func TestRunMatch_Riichi_DealerRotation(t *testing.T) {
	// Run 4 Riichi rounds and verify each round's dealer was different
	// (exactly one full 東風戦 rotation when Renchan triggers rarely).
	// We can't deterministically assert which dealer where without
	// controlled win outcomes; just sanity-check at least one rotation
	// happened (dealer != 0 at some point).
	rule := riichi.New()
	players := [game.NumPlayers]game.Player{
		easy.New("a"), easy.New("b"), easy.New("c"), easy.New("d"),
	}
	silent := slog.New(slog.NewTextHandler(io.Discard, nil))
	obs := &dealerRecorder{}
	_, err := match.RunMatch(match.Config{
		Rule:      rule,
		Players:   players,
		MaxRounds: 4,
		Renchan:   true,
	}, silent, obs)
	if err != nil {
		t.Fatal(err)
	}
	if len(obs.dealers) != 4 {
		t.Fatalf("expected 4 OnRoundStart events, got %d", len(obs.dealers))
	}
	distinct := map[int]bool{}
	for _, d := range obs.dealers {
		distinct[d] = true
	}
	if len(distinct) < 2 {
		t.Errorf("no dealer rotation observed across 4 rounds: %v", obs.dealers)
	}
}

type dealerRecorder struct {
	game.NoopObserver
	dealers []int
}

func (r *dealerRecorder) OnRoundStart(s *game.State) {
	r.dealers = append(r.dealers, s.Dealer)
}
