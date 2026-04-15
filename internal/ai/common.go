// Package ai is the umbrella for bot implementations and shared
// utilities. Concrete bots live in subpackages (easy, medium, hard).
package ai

import (
	"fmt"

	"github.com/Minalinnski/RonTama/internal/game"
	"github.com/Minalinnski/RonTama/internal/rules"
	"github.com/Minalinnski/RonTama/internal/shanten"
	"github.com/Minalinnski/RonTama/internal/tile"
)

// Difficulty enumerates the bot strength tiers.
//
// The factory that turns a Difficulty into a concrete game.Player
// lives in cmd/rontama (importing easy/medium/hard would create a
// cycle, since those packages import this one for shared helpers).
type Difficulty int

const (
	Easy Difficulty = iota
	Medium
	Hard
)

func (d Difficulty) String() string {
	switch d {
	case Easy:
		return "easy"
	case Medium:
		return "medium"
	case Hard:
		return "hard"
	default:
		return "unknown"
	}
}

// ParseDifficulty parses "easy" / "medium" / "hard" (case-sensitive).
func ParseDifficulty(s string) (Difficulty, error) {
	switch s {
	case "easy":
		return Easy, nil
	case "medium":
		return Medium, nil
	case "hard":
		return Hard, nil
	default:
		return 0, fmt.Errorf("unknown difficulty %q (want: easy|medium|hard)", s)
	}
}

// ShantenFn returns the appropriate shanten function for a rule.
func ShantenFn(r rules.RuleSet) func([tile.NumKinds]int, int) int {
	if r.RequiresDingque() {
		return shanten.OfSichuan
	}
	return shanten.Of
}

// ChooseDingqueLeastTiles picks the suit (man/pin/sou) with the fewest
// tiles in the hand. Used by every difficulty as a baseline.
func ChooseDingqueLeastTiles(view game.PlayerView) tile.Suit {
	counts := [3]int{}
	for s := 0; s < 3; s++ {
		for n := 0; n < 9; n++ {
			counts[s] += view.OwnHand.Concealed[s*9+n]
		}
	}
	best := tile.SuitMan
	bestCnt := counts[0]
	for s := 1; s < 3; s++ {
		if counts[s] < bestCnt {
			bestCnt = counts[s]
			best = tile.Suit(s)
		}
	}
	return best
}

// ComputeSeen tallies tile copies visible to the seat: own concealed,
// melds (all players'), and discards.
func ComputeSeen(view game.PlayerView) [tile.NumKinds]int {
	var seen [tile.NumKinds]int
	for i, c := range view.OwnHand.Concealed {
		seen[i] = c
	}
	for p := 0; p < game.NumPlayers; p++ {
		for _, m := range view.Melds[p] {
			for _, t := range m.Tiles {
				seen[t]++
			}
		}
		for _, t := range view.Discards[p] {
			seen[t]++
		}
	}
	return seen
}

// CanTsumo checks whether the just-drawn tile completes a winning hand
// for the seat. Returns true only when JustDrew is non-nil.
func CanTsumo(view game.PlayerView) bool {
	if view.JustDrew == nil {
		return false
	}
	hand := view.OwnHand
	concealed := hand.Concealed
	concealed[*view.JustDrew]--
	probe := tile.Hand{Concealed: concealed, Melds: hand.Melds}
	ctx := rules.WinContext{
		WinningTile: *view.JustDrew,
		Tsumo:       true,
		From:        -1,
		Seat:        view.Seat,
		Dealer:      view.Dealer,
		Dingque:     view.Dingque[view.Seat],
	}
	return view.Rule.CanWin(probe, *view.JustDrew, ctx)
}

// CountByKindInSuit returns the number of tiles in concealed of suit s.
func CountByKindInSuit(c [tile.NumKinds]int, s tile.Suit) int {
	n := 0
	for i := 0; i < tile.NumKinds; i++ {
		if c[i] > 0 && tile.Tile(i).Suit() == s {
			n += c[i]
		}
	}
	return n
}

// MustDiscardDingque returns a dingque-suit tile to discard, or
// (0, false) if the hand has none.
func MustDiscardDingque(view game.PlayerView) (tile.Tile, bool) {
	dingque := view.Dingque[view.Seat]
	if dingque == tile.SuitWind || dingque == tile.SuitDragon {
		return 0, false
	}
	for i := 0; i < tile.NumKinds; i++ {
		if view.OwnHand.Concealed[i] > 0 && tile.Tile(i).Suit() == dingque {
			return tile.Tile(i), true
		}
	}
	return 0, false
}

// PickExchange3 picks 3 tiles of one suit to pass during 换三张.
// Strategy: choose the suit with the fewest tiles in hand (still
// having at least 3); take the 3 lowest-numbered tiles of that suit.
//
// "Lowest" is a deliberate choice: it tends to dump terminals/edge
// tiles which are less flexible for runs.
func PickExchange3(view game.PlayerView) [3]tile.Tile {
	c := view.OwnHand.Concealed
	counts := [3]int{}
	for s := 0; s < 3; s++ {
		for n := 0; n < 9; n++ {
			counts[s] += c[s*9+n]
		}
	}
	bestSuit := -1
	bestCount := 99
	for s := 0; s < 3; s++ {
		if counts[s] >= 3 && counts[s] < bestCount {
			bestSuit = s
			bestCount = counts[s]
		}
	}
	if bestSuit < 0 {
		// Pigeonhole: 13 tiles in 3 suits → at least one has >= 5.
		// Fall through is defensive only.
		for s := 0; s < 3; s++ {
			if counts[s] > bestCount {
				bestSuit = s
				bestCount = counts[s]
			}
		}
	}
	var picks [3]tile.Tile
	n := 0
	for i := bestSuit * 9; i < (bestSuit+1)*9 && n < 3; i++ {
		for j := 0; j < c[i] && n < 3; j++ {
			picks[n] = tile.Tile(i)
			n++
		}
	}
	return picks
}

// AlwaysRon picks the ron call from opportunities, else returns Pass.
// Shared default for "I always take the win" bots.
func AlwaysRon(opps []game.Call) game.Call {
	for _, o := range opps {
		if o.Kind == game.CallRon {
			return o
		}
	}
	return game.Pass
}

// ShouldDeclareRiichi returns true when the bot can/should declare
// riichi this turn: rule supports it (Riichi only), hand fully
// concealed, not already riichi'd, score >= 1000, wall >= 4, and
// discarding `discard` leaves the hand at tenpai.
//
// Bots use this from OnDraw to decide the DeclareRiichi flag on a
// DrawDiscard action.
func ShouldDeclareRiichi(view game.PlayerView, discard tile.Tile) bool {
	if view.Rule.RequiresDingque() {
		return false // Sichuan
	}
	if view.HasWon[view.Seat] {
		return false
	}
	if view.Riichi[view.Seat] {
		return false // already declared
	}
	if view.Scores[view.Seat] < 1000 {
		return false
	}
	if view.WallLeft < 4 {
		return false
	}
	hand := view.OwnHand
	for _, m := range hand.Melds {
		if m.Kind != tile.ConcealedKan {
			return false // open hand
		}
	}
	// Already riichi'd? We can detect by whether the seat is locked into
	// a tenpai shape forever — but the View doesn't carry that. Bots
	// typically declare riichi only the first time they reach tenpai;
	// this simple check stays correct because the loop validates and
	// rejects double-declarations.
	probe := hand.Concealed
	if probe[discard] == 0 {
		return false
	}
	probe[discard]--
	return shanten.Of(probe, len(hand.Melds)) == 0
}
