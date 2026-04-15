// Package sichuan implements the Sichuan "Blood Battle" (血战到底)
// mahjong variant.
//
// Highlights:
//   - 108 suited tiles (no honors, no flowers)
//   - Each player declares 定缺 (renounced suit) before play; the
//     player must discard all tiles of that suit before they can win
//   - Chi is not allowed; only pon, kan, ron/tsumo
//   - Game continues after a player wins ("blood battle"): the winner
//     drops out and the remaining players play on until 3 have won or
//     the wall is exhausted
//
// Scoring uses a doubling 番 system. Patterns implemented in this
// initial pass:
//   - 平胡       (1 fan)  basic 4 sets + 1 pair
//   - 大对子     (2 fan)  all triplets + pair (碰碰胡)
//   - 七对       (2 fan)  seven distinct pairs
//   - 龙七对     (4 fan)  seven pairs with one quad
//   - 清一色     (4 fan)  single suit
//   - 清碰       (8 fan)  清一色 + 大对子 stacked multiplicatively
//   - 清七对     (8 fan)  清一色 + 七对
//   - 清龙七对   (16 fan) 清一色 + 龙七对
//
// Bonuses (additive fan):
//   - 自摸 (+1)   self-drawn win
//   - 杠上花 (+1) win on the replacement tile after a kan
//   - 抢杠胡 (+1) ron on a tile being added to a pon for kan
//   - 海底 (+1)   win on the last drawn tile
package sichuan

import (
	"github.com/Minalinnski/RonTama/internal/rules"
	"github.com/Minalinnski/RonTama/internal/shanten"
	"github.com/Minalinnski/RonTama/internal/tile"
)

// Rule is the Sichuan ruleset (血战到底 variant).
type Rule struct{}

// New returns the Sichuan ruleset.
func New() *Rule { return &Rule{} }

// Name implements rules.RuleSet.
func (Rule) Name() string { return "sichuan-bloodbattle" }

// TileKinds returns the 27 suited tile kinds.
func (Rule) TileKinds() []tile.Tile { return tile.SuitedKinds() }

// CopiesPerTile is 4.
func (Rule) CopiesPerTile() int { return 4 }

// HandSize is 13.
func (Rule) HandSize() int { return 13 }

// StartingScore is 0; Sichuan rounds report deltas, not absolute scores.
func (Rule) StartingScore() int { return 0 }

// AllowsChi is false in Sichuan.
func (Rule) AllowsChi() bool { return false }

// RequiresDingque is true.
func (Rule) RequiresDingque() bool { return true }

// CanWin checks Sichuan agari validity.
//
// Requirements:
//   - Concealed + winning tile forms a valid 4-sets-1-pair OR seven-pairs shape
//   - No tile of the player's dingque suit remains in the hand after the win
func (Rule) CanWin(hand tile.Hand, winTile tile.Tile, ctx rules.WinContext) bool {
	// Add the winning tile to the count vector, then check shanten == -1.
	combined := hand.Concealed
	combined[winTile]++
	melds := len(hand.Melds)

	// Dingque check: no tiles of renounced suit allowed.
	if hasSuit(combined, ctx.Dingque) {
		return false
	}
	// Also melds must not contain dingque suit (should be filtered earlier
	// but defend against bugs).
	for _, m := range hand.Melds {
		for _, t := range m.Tiles {
			if t.Suit() == ctx.Dingque {
				return false
			}
		}
	}

	// Standard form
	if shanten.OfStandard(combined, melds) == -1 {
		return true
	}
	// Seven pairs (only if no melds)
	if melds == 0 && shanten.OfSevenPairs(combined) == -1 {
		return true
	}
	return false
}

// hasSuit reports whether any tile of suit s has count > 0.
func hasSuit(c [tile.NumKinds]int, s tile.Suit) bool {
	for i := 0; i < tile.NumKinds; i++ {
		if c[i] > 0 && tile.Tile(i).Suit() == s {
			return true
		}
	}
	return false
}

// Settle implements rules.RuleSet.
//
// Sichuan: tsumo → each live (not-already-won) non-winner pays BasePts;
// ron → discarder pays 2× BasePts.
func (Rule) Settle(dealer, winner int, ctx rules.WinContext, score rules.Score, hasWon [4]bool) [4]int {
	var d [4]int
	if ctx.Tsumo {
		for i := 0; i < 4; i++ {
			if i == winner || hasWon[i] {
				continue
			}
			d[i] -= score.BasePts
			d[winner] += score.BasePts
		}
	} else {
		pay := score.BasePts * 2
		d[ctx.From] -= pay
		d[winner] += pay
	}
	return d
}

// ScoreWin computes Sichuan score. See package doc for patterns.
func (Rule) ScoreWin(hand tile.Hand, winTile tile.Tile, ctx rules.WinContext) rules.Score {
	combined := hand.Concealed
	combined[winTile]++
	melds := hand.Melds

	patterns := []string{}
	fan := 0

	// Detect base patterns (mutually exclusive ish — pick the highest pair-form).
	isSevenPairs := len(melds) == 0 && shanten.OfSevenPairs(combined) == -1
	isStandard := shanten.OfStandard(combined, len(melds)) == -1

	allTriplets := isStandard && allTripletForm(combined, melds)
	singleSuit := singleSuitOnly(combined, melds)
	hasKan := false
	for _, m := range melds {
		if m.IsKan() {
			hasKan = true
			break
		}
	}
	dragonSevenPairs := isSevenPairs && hasFourOfAKind(combined)

	switch {
	case singleSuit && dragonSevenPairs:
		patterns = append(patterns, "清龙七对")
		fan = 16
	case singleSuit && allTriplets:
		patterns = append(patterns, "清碰")
		fan = 8
	case singleSuit && isSevenPairs:
		patterns = append(patterns, "清七对")
		fan = 8
	case dragonSevenPairs:
		patterns = append(patterns, "龙七对")
		fan = 4
	case singleSuit:
		patterns = append(patterns, "清一色")
		fan = 4
	case allTriplets:
		patterns = append(patterns, "大对子")
		fan = 2
	case isSevenPairs:
		patterns = append(patterns, "七对")
		fan = 2
	default:
		patterns = append(patterns, "平胡")
		fan = 1
	}

	// Additive situation bonuses.
	if ctx.Tsumo {
		patterns = append(patterns, "自摸")
		fan++
	}
	if ctx.AfterKan {
		patterns = append(patterns, "杠上花")
		fan++
	}
	if ctx.KanGrab {
		patterns = append(patterns, "抢杠胡")
		fan++
	}
	if ctx.LastTile {
		patterns = append(patterns, "海底")
		fan++
	}
	if hasKan && !isSevenPairs {
		// Kan presence isn't a pattern by itself but generally is settled
		// with separate kan payments (杠分). Track it for callers to read.
		patterns = append(patterns, "带杠")
	}

	// Base points: 1 unit doubled per fan beyond 1.
	base := 1
	for i := 1; i < fan; i++ {
		base *= 2
	}

	return rules.Score{
		Patterns: patterns,
		Fan:      fan,
		BasePts:  base,
	}
}

// allTripletForm reports whether the hand decomposes into 4 triplets/kans + 1 pair.
// Concealed counts is the post-win count vector; melds are declared melds.
func allTripletForm(c [tile.NumKinds]int, melds []tile.Meld) bool {
	for _, m := range melds {
		// Kans count as triplets; chi (runs) would disqualify.
		if m.Kind == tile.Chi {
			return false
		}
	}
	pairs, triplets := 0, 0
	for _, n := range c {
		switch n {
		case 0:
			// nothing
		case 2:
			pairs++
		case 3:
			triplets++
		default:
			// Any other count breaks the all-triplet shape.
			return false
		}
	}
	requiredTriplets := 4 - len(melds)
	return triplets == requiredTriplets && pairs == 1
}

// singleSuitOnly reports whether the entire hand (concealed + melds) lives
// in a single suit. 清一色 is suit-only (no honors); since Sichuan has no
// honors, "single suit" naturally means one of man/pin/sou.
func singleSuitOnly(c [tile.NumKinds]int, melds []tile.Meld) bool {
	suit := tile.Suit(255)
	check := func(t tile.Tile) bool {
		s := t.Suit()
		if suit == 255 {
			suit = s
			return true
		}
		return suit == s
	}
	for i := 0; i < tile.NumKinds; i++ {
		if c[i] > 0 {
			if !check(tile.Tile(i)) {
				return false
			}
		}
	}
	for _, m := range melds {
		for _, t := range m.Tiles {
			if !check(t) {
				return false
			}
		}
	}
	return suit != 255
}

// hasFourOfAKind reports whether any tile kind appears exactly 4 times in the
// concealed counts (for 龙七对 detection).
func hasFourOfAKind(c [tile.NumKinds]int) bool {
	for _, n := range c {
		if n == 4 {
			return true
		}
	}
	return false
}
