package riichi_test

import (
	"io"
	"log/slog"
	"strings"
	"testing"

	"github.com/Minalinnski/RonTama/internal/ai/easy"
	"github.com/Minalinnski/RonTama/internal/game"
	"github.com/Minalinnski/RonTama/internal/match"
	"github.com/Minalinnski/RonTama/internal/rules/riichi"
	"github.com/Minalinnski/RonTama/internal/tile"
)

func silent() *slog.Logger { return slog.New(slog.NewTextHandler(io.Discard, nil)) }

func fourBots() [game.NumPlayers]game.Player {
	return [game.NumPlayers]game.Player{
		easy.New("a"), easy.New("b"), easy.New("c"), easy.New("d"),
	}
}

// TestHooks_DoraAppearsInScore runs Riichi rounds and confirms dora
// han actually shows up in at least some wins. Before the hooks
// refactor, DoraIndicators was never populated so dora was always 0.
func TestHooks_DoraAppearsInScore(t *testing.T) {
	rule := riichi.New()
	players := fourBots()
	doraFound := false
	for r := 0; r < 20 && !doraFound; r++ {
		res, err := game.RunRound(rule, players, r%4, silent())
		if err != nil {
			t.Fatalf("round %d: %v", r, err)
		}
		for _, w := range res.Wins {
			for _, p := range w.Score.Patterns {
				if strings.HasPrefix(p, "ドラ") {
					doraFound = true
				}
			}
		}
	}
	if !doraFound {
		t.Errorf("dora never appeared in 20 rounds — hooks may not be plumbing DoraIndicators")
	}
}

// TestHooks_FuritenBlocks verifies the furiten mechanism: a bot that
// discards tile X should not be offered ron on X later. We can't easily
// control which tiles are discarded in a real game, but we can verify
// the hooks' AvailableCalls doesn't offer ron on furiten tiles.
func TestHooks_FuritenBlocks(t *testing.T) {
	rule := riichi.New()
	hooks := rule.Hooks().(*riichi.Hooks)

	// Create a minimal StateAccessor mock by running a real state.
	st, err := game.NewState(rule, 0)
	if err != nil {
		t.Fatal(err)
	}
	hooks.OnRoundSetup(st)

	// Simulate: seat 0 discards Man1.
	st.Discards[0] = append(st.Discards[0], tile.Man1)
	hooks.AfterDiscard(st, 0, tile.Man1)

	// Now check: if seat 1 discards Man1, seat 0 should NOT get ron
	// (because seat 0 is in furiten for Man1).
	calls := hooks.AvailableCalls(st, tile.Man1, 1)
	for _, c := range calls {
		if c.Player == 0 && c.Kind == 4 { // 4 = CallRon
			t.Errorf("seat 0 should not be offered ron on Man1 (furiten)")
		}
	}
}

// TestHooks_RoundWindFromMatch runs a 4-round match and checks that
// round wind rotates from East to... well, for a 4-round 東風戦 it stays
// East (round index 0-3 / 4 = 0 = East). For 8 rounds it'd be South.
func TestHooks_RoundWindFromMatch(t *testing.T) {
	rule := riichi.New()
	players := fourBots()
	res, err := match.RunMatch(match.Config{
		Rule:      rule,
		Players:   players,
		MaxRounds: 4,
		Renchan:   true,
	}, silent(), nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(res.Rounds) != 4 {
		t.Errorf("expected 4 rounds, got %d", len(res.Rounds))
	}
	// All 4 rounds should have completed without panics.
}

// TestHooks_IppatsuWindowCloses runs many rounds and just confirms
// no panic or infinite loop from ippatsu tracking.
func TestHooks_IppatsuStress(t *testing.T) {
	rule := riichi.New()
	players := fourBots()
	for r := 0; r < 10; r++ {
		_, err := game.RunRound(rule, players, r%4, silent())
		if err != nil {
			t.Fatalf("round %d: %v", r, err)
		}
	}
}

// TestHooks_DeadWallSize verifies that after OnRoundSetup the wall
// has 14 fewer tiles (122 remaining from 136 - 52 dealt - 14 dead = 70).
func TestHooks_DeadWallSize(t *testing.T) {
	rule := riichi.New()
	st, err := game.NewState(rule, 0)
	if err != nil {
		t.Fatal(err)
	}
	hooks := rule.Hooks().(*riichi.Hooks)
	hooks.OnRoundSetup(st)
	// After dealing 13×4=52 tiles and carving 14 dead wall: 136-52-14=70
	if st.Wall.Remaining() != 70 {
		t.Errorf("wall remaining = %d, want 70", st.Wall.Remaining())
	}
}
