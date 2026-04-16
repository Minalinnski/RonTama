// Package riichi implements Japanese Riichi mahjong.
//
// Phase 6 scope (intentionally MVP — full Riichi rules are a multi-week
// undertaking):
//
//	tiles:    full 136 (4 of each of the 34 kinds)
//	wins:     standard 4-sets+pair, chiitoitsu, kokushi musou
//	yaku:     立直, 一発, 門前清自摸和, 断幺九, 役牌 (dragons + round/seat winds),
//	          一気通貫, 三色同順, 対々和, 七対子, 混一色, 清一色, 国士無双
//	fu:       base 20 + meld/wait/win-method bonuses (round to 10)
//	score:    standard formula with mangan/haneman/baiman/sanbaiman/yakuman caps
//	dora:     +1 han per dora tile (uradora only on riichi wins)
//	furiten:  basic — own-discard furiten only (no temp/eternal distinction)
//
// Not yet implemented (intentional):
//   - 平和 (pinfu) — needs full structural decomposition; deferred
//   - 三色同刻, 三槓子, 小三元 etc. — uncommon yaku
//   - Riichi declaration mechanics (declared via Player interface but
//     scored opportunistically when WinContext.Riichi is set)
//   - 振聴 across temporary windows; double riichi mostly relies on
//     Player setting WinContext.DoubleRiichi
//
// Caller passes any Riichi-specific state in WinContext (Riichi flag,
// dora indicators, etc.); the Rule itself is stateless.
package riichi

import (
	"github.com/Minalinnski/RonTama/internal/rules"
	"github.com/Minalinnski/RonTama/internal/shanten"
	"github.com/Minalinnski/RonTama/internal/tile"
)

// Rule is the Riichi mahjong ruleset. Uses pointer receiver so
// Hooks() can capture a back-reference.
type Rule struct{}

// New returns the Riichi ruleset. Use *Rule (pointer receiver) so
// Hooks() can return a &Hooks{rule: r} back-reference.
func New() *Rule { return &Rule{} }

// Name implements rules.RuleSet.
func (Rule) Name() string { return "riichi" }

// TileKinds returns all 34 kinds.
func (Rule) TileKinds() []tile.Tile { return tile.AllKinds() }

// CopiesPerTile is 4.
func (Rule) CopiesPerTile() int { return 4 }

// HandSize is 13.
func (Rule) HandSize() int { return 13 }

// StartingScore is 25000 (standard Riichi opening stack).
func (Rule) StartingScore() int { return 25000 }

// AllowsChi is true in Riichi.
func (Rule) AllowsChi() bool { return true }

// RequiresDingque is false.
func (Rule) RequiresDingque() bool { return false }

// RequiresExchange3 is false.
func (Rule) RequiresExchange3() bool { return false }

// Hooks returns the Riichi-specific lifecycle hooks.
func (r *Rule) Hooks() rules.RuleHooks {
	return NewHooks(r, tile.East) // round wind default; overridden by match via RuleState
}

// CanWin returns true if hand+winningTile is a standard, chiitoi, or
// kokushi shape AND has at least one yaku (otherwise no-yaku → no-win).
func (r Rule) CanWin(hand tile.Hand, winTile tile.Tile, ctx rules.WinContext) bool {
	combined := hand.Concealed
	combined[winTile]++
	melds := len(hand.Melds)

	// Shape check (strict — see shanten.IsWinningStandard godoc)
	standard := shanten.IsWinningStandard(combined, melds)
	chiitoi := melds == 0 && shanten.IsWinningSevenPairs(combined)
	kokushi := melds == 0 && shanten.IsWinningKokushi(combined)
	if !standard && !chiitoi && !kokushi {
		return false
	}

	// Need at least one yaku (Riichi requires yaku for valid win,
	// "no-yaku" hands cannot be declared).
	res := r.evaluate(hand, winTile, ctx, standard, chiitoi, kokushi)
	return res.Han > 0 || len(res.Yakuman) > 0
}

// Settle implements the standard Riichi payment matrix.
//
//	dealer ron     → discarder pays base × 6
//	non-dealer ron → discarder pays base × 4
//	dealer tsumo   → each of 3 non-dealers pays base × 2
//	non-dealer tsumo → dealer pays base × 2; the two other non-dealers each pay base × 1
//
// Each payment is rounded up to the next 100.
func (Rule) Settle(dealer, winner int, ctx rules.WinContext, score rules.Score, _ [4]bool) [4]int {
	var d [4]int
	base := score.BasePts
	dealerWin := winner == dealer
	if ctx.Tsumo {
		for i := 0; i < 4; i++ {
			if i == winner {
				continue
			}
			mult := 1
			if dealerWin || i == dealer {
				mult = 2
			}
			pay := roundUp100(base * mult)
			d[i] -= pay
			d[winner] += pay
		}
	} else {
		mult := 4
		if dealerWin {
			mult = 6
		}
		pay := roundUp100(base * mult)
		d[ctx.From] -= pay
		d[winner] += pay
	}
	return d
}

func roundUp100(n int) int {
	if n%100 == 0 {
		return n
	}
	return n + (100 - n%100)
}

// ScoreWin computes the final score breakdown.
func (r Rule) ScoreWin(hand tile.Hand, winTile tile.Tile, ctx rules.WinContext) rules.Score {
	combined := hand.Concealed
	combined[winTile]++
	melds := len(hand.Melds)
	standard := shanten.IsWinningStandard(combined, melds)
	chiitoi := melds == 0 && shanten.IsWinningSevenPairs(combined)
	kokushi := melds == 0 && shanten.IsWinningKokushi(combined)

	res := r.evaluate(hand, winTile, ctx, standard, chiitoi, kokushi)

	patterns := append([]string{}, res.Yaku...)
	for _, name := range res.Yakuman {
		patterns = append(patterns, name)
	}

	base := basePoints(res.Han, res.Fu, res.Yakuman)
	return rules.Score{
		Patterns: patterns,
		Fan:      res.Han,
		BasePts:  base,
	}
}
