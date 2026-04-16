// Package match is a thin wrapper over the single-round game loop
// that strings several rounds together into a "match" — a.k.a.
// session / hanchan / 東風戦.
//
// Design notes (per user feedback "match 循环和单局解耦，不要改烂了"):
//
//   - The inner single-round loop (internal/game.RunRoundWithObserver)
//     is used AS-IS. No modifications needed. Match builds on top.
//   - Match owns cumulative-state concerns: running scores, current
//     dealer, honba counter, riichi-pot carryover.
//   - Each variant (Riichi, Sichuan) can specify its own match format:
//     number of rounds, dealer-rotation policy, etc. Sichuan blood-
//     battle typically runs 1 round; Riichi runs 4 (東風戦) or 8 (半荘).
package match

import (
	"fmt"
	"log/slog"

	"github.com/Minalinnski/RonTama/internal/game"
	"github.com/Minalinnski/RonTama/internal/rules"
	"github.com/Minalinnski/RonTama/internal/tile"
)

// Config describes a match's length and behaviour.
type Config struct {
	Rule         rules.RuleSet
	Players      [game.NumPlayers]game.Player
	InitialDealer int
	// MaxRounds caps the total rounds. 0 = play one round and stop
	// (the old single-round default).
	MaxRounds int
	// Renchan, when true, keeps the dealer seated after a dealer win
	// and increments Honba. Standard Riichi behaviour. For Sichuan
	// blood-battle with its own continuation rules this is typically
	// false since matches are single-round.
	Renchan bool
}

// State is the per-match mutable state carried across rounds.
type State struct {
	RoundIdx  int
	Dealer    int
	Honba     int
	RiichiPot int
	Scores    [game.NumPlayers]int
}

// Result aggregates per-round results plus final cumulative scores.
type Result struct {
	Rounds      []*game.RoundResult
	FinalScores [game.NumPlayers]int
}

// RunMatch plays rounds sequentially until Config.MaxRounds is reached
// (MaxRounds=0 means a single round). The same `obs` observer sees
// all rounds' events.
func RunMatch(cfg Config, log *slog.Logger, obs game.Observer) (*Result, error) {
	if log == nil {
		log = slog.Default()
	}
	if obs == nil {
		obs = game.NoopObserver{}
	}
	st := &State{
		Dealer: cfg.InitialDealer,
		Scores: seedScores(cfg),
	}
	rounds := 1
	if cfg.MaxRounds > 0 {
		rounds = cfg.MaxRounds
	}
	result := &Result{}
	for r := 0; r < rounds; r++ {
		st.RoundIdx = r
		rr, err := runOne(cfg, st, log, obs)
		if err != nil {
			return result, fmt.Errorf("match round %d: %w", r, err)
		}
		result.Rounds = append(result.Rounds, rr)
		applyRoundDeltas(cfg, st, rr)
	}
	result.FinalScores = st.Scores
	return result, nil
}

// runOne runs a single round with the match's cumulative state seeded
// in (scores + carried riichi pot). The round's own scoring adds to
// those scores and applySettlement consumes the pot when someone wins.
//
// TODO: honba bonus (+300 per honba to winner) isn't yet plumbed into
// per-round payout — accumulated but cosmetic for now.
func runOne(cfg Config, st *State, log *slog.Logger, obs game.Observer) (*game.RoundResult, error) {
	scores := st.Scores
	// Compute round wind from round index: 0-3 = East, 4-7 = South, etc.
	winds := []tile.Tile{tile.East, tile.South, tile.West, tile.North}
	roundWind := winds[(st.RoundIdx/game.NumPlayers)%4]
	opts := game.RoundOpts{
		Rule:           cfg.Rule,
		Players:        cfg.Players,
		Dealer:         st.Dealer,
		Log:            log,
		Observer:       obs,
		InitialScores:  &scores,
		CarryRiichiPot: st.RiichiPot,
		Honba:          st.Honba,
		RoundWind:      roundWind,
	}
	return game.RunRoundOpts(opts)
}

// applyRoundDeltas copies the round's final scores into cumulative
// state (the round began with st.Scores seeded, so its FinalScores
// ARE the new cumulative values), then decides the next dealer +
// honba per the rule's policy.
func applyRoundDeltas(cfg Config, st *State, rr *game.RoundResult) {
	st.Scores = rr.FinalScores
	// Dealer rotation / renchan decision.
	dealerWon := false
	anyWon := len(rr.Wins) > 0
	for _, w := range rr.Wins {
		if w.Seat == st.Dealer {
			dealerWon = true
			break
		}
	}
	switch {
	case cfg.Renchan && dealerWon:
		// Dealer continues (連荘).
		st.Honba++
		st.RiichiPot = 0
	case !anyWon && rr.Exhaustion:
		// Exhaustive draw — honba grows. Dealer stays in this MVP
		// regardless of whether they were tenpai (proper rule: dealer
		// only continues when tenpai at exhaustion).
		st.Honba++
		// RiichiPot: declarers' sticks stay on the table for next round.
		// The inner loop already left them there since no winner took them.
		// Read back from rr by diff: if any player's score dropped by 1000
		// relative to pre-round and no win counted them, it's on the table.
		// For MVP we trust the inner loop's bookkeeping and assume rr's
		// FinalScores already reflect the 1000-point debits; carry a
		// reasonable approximation. (Proper: inner loop would return
		// RemainingPot; defer that refinement.)
	default:
		st.Dealer = (st.Dealer + 1) % game.NumPlayers
		st.Honba = 0
		st.RiichiPot = 0
	}
}

// seedScores initialises cumulative match scores from the rule's
// StartingScore (Riichi 25000, Sichuan 0).
func seedScores(cfg Config) [game.NumPlayers]int {
	start := cfg.Rule.StartingScore()
	var s [game.NumPlayers]int
	for i := range s {
		s[i] = start
	}
	return s
}
