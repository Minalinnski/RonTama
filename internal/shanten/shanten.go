// Package shanten computes the "shanten number" of a mahjong hand:
// the minimum number of useful tile draws still needed to reach
// tenpai (one tile from winning).
//
// Conventions:
//
//	-1 = winning hand (agari)
//	 0 = tenpai (waiting on one tile)
//	 N = N tiles away from tenpai
//
// All three winning forms are evaluated and the minimum is returned:
//
//   - Standard:    4 sets + 1 pair
//   - Seven pairs: 7 distinct pairs (Riichi only)
//   - Kokushi:     13 different yaochuu + a pair (Riichi only)
//
// Sichuan callers should use OfStandard directly (chiitoitsu/kokushi
// don't exist in Sichuan).
package shanten

import "github.com/Minalinnski/RonTama/internal/tile"

// Of returns min(standard, seven-pairs, kokushi) for a Riichi-style
// hand. Pass the concealed-tile count vector and the number of
// already-declared melds.
func Of(hand [tile.NumKinds]int, melds int) int {
	best := OfStandard(hand, melds)
	if melds == 0 {
		// Seven pairs / kokushi only valid with a fully concealed hand.
		if v := OfSevenPairs(hand); v < best {
			best = v
		}
		if v := OfKokushi(hand); v < best {
			best = v
		}
	}
	return best
}

// OfSichuan returns the standard-form shanten only (Sichuan rules
// do not allow chiitoitsu or kokushi).
func OfSichuan(hand [tile.NumKinds]int, melds int) int {
	return OfStandard(hand, melds)
}

// OfStandard returns shanten for the 4-sets + 1-pair winning form.
func OfStandard(hand [tile.NumKinds]int, melds int) int {
	// Worst case: every possible tile must become useful.
	best := 8 - 2*melds
	work := hand

	// Branch 1: try each tile kind as the dedicated pair.
	for i := 0; i < tile.NumKinds; i++ {
		if work[i] >= 2 {
			work[i] -= 2
			s, p := bestDecompose(&work)
			work[i] += 2
			capP := 4 - melds - s
			if p > capP {
				p = capP
			}
			if p < 0 {
				p = 0
			}
			sh := 8 - 2*(s+melds) - p - 1
			if sh < best {
				best = sh
			}
		}
	}

	// Branch 2: no pair extracted; an implicit partial may become the pair.
	s, p := bestDecompose(&work)
	capP := 5 - melds - s
	if p > capP {
		p = capP
	}
	if p < 0 {
		p = 0
	}
	sh := 8 - 2*(s+melds) - p
	if sh < best {
		best = sh
	}
	return best
}

// OfSevenPairs returns shanten for chiitoitsu form. Only valid for a
// fully concealed Riichi hand (callers handle the precondition).
func OfSevenPairs(hand [tile.NumKinds]int) int {
	pairs, kinds := 0, 0
	for _, c := range hand {
		if c >= 2 {
			pairs++
		}
		if c >= 1 {
			kinds++
		}
	}
	// We need 7 pairs, all distinct kinds. If we have P pairs and K
	// distinct kinds, we still need (7-P) more pairs from the remaining
	// (K-P) singles plus brand-new draws.
	shortage := 7 - kinds
	if shortage < 0 {
		shortage = 0
	}
	return 6 - pairs + shortage
}

// IsWinningStandard reports whether `hand` (the count vector of
// concealed tiles, including any winning tile already added) plus
// `melds` declared melds forms an EXACT 4-sets + 1-pair winning shape.
//
// This is stricter than `OfStandard(...) == -1`: shanten conflates
// pair-partials and run-partials (XY waiting on Z) since both reduce
// the tile count needed by 1 — that's correct for shanten counting
// but wrong for "is this a complete win" because a run-partial
// without a real pair cannot satisfy the 4-sets + 1-pair invariant.
//
// Used by Rule.CanWin to avoid offering invalid ron/tsumo opportunities.
func IsWinningStandard(hand [tile.NumKinds]int, melds int) bool {
	need := 4 - melds
	if need < 0 {
		return false
	}
	for i := 0; i < tile.NumKinds; i++ {
		if hand[i] >= 2 {
			hand[i] -= 2
			ok := canFormSetsExact(&hand, 0, need)
			hand[i] += 2
			if ok {
				return true
			}
		}
	}
	return false
}

// canFormSetsExact returns true iff `c` from index `start` can be
// fully consumed by exactly `need` sets (triplets or runs).
func canFormSetsExact(c *[tile.NumKinds]int, start, need int) bool {
	for start < tile.NumKinds && (*c)[start] == 0 {
		start++
	}
	if start >= tile.NumKinds {
		return need == 0
	}
	if need == 0 {
		// tiles remain but no more sets allowed
		return false
	}
	// Triplet
	if (*c)[start] >= 3 {
		(*c)[start] -= 3
		if canFormSetsExact(c, start, need-1) {
			(*c)[start] += 3
			return true
		}
		(*c)[start] += 3
	}
	// Run (suit tiles only; need start in 0..6 within suit)
	if start < 27 && start%9 <= 6 && (*c)[start+1] > 0 && (*c)[start+2] > 0 {
		(*c)[start]--
		(*c)[start+1]--
		(*c)[start+2]--
		if canFormSetsExact(c, start, need-1) {
			(*c)[start]++
			(*c)[start+1]++
			(*c)[start+2]++
			return true
		}
		(*c)[start]++
		(*c)[start+1]++
		(*c)[start+2]++
	}
	return false
}

// IsWinningSevenPairs reports whether the concealed-only hand forms
// 7 distinct pairs (chiitoitsu).
func IsWinningSevenPairs(hand [tile.NumKinds]int) bool {
	pairs, total := 0, 0
	for _, c := range hand {
		total += c
		if c == 2 {
			pairs++
		} else if c != 0 {
			// 3 or 4 of a kind disqualifies chiitoi (each pair must be distinct)
			return false
		}
	}
	return pairs == 7 && total == 14
}

// IsWinningKokushi reports whether the concealed hand is the 13
// distinct yaochuu plus a pair from those 13.
func IsWinningKokushi(hand [tile.NumKinds]int) bool {
	return OfKokushi(hand) == -1
}

// OfKokushi returns shanten for kokushi musou (thirteen orphans).
func OfKokushi(hand [tile.NumKinds]int) int {
	yaochuu := []tile.Tile{
		tile.Man1, tile.Man9,
		tile.Pin1, tile.Pin9,
		tile.Sou1, tile.Sou9,
		tile.East, tile.South, tile.West, tile.North,
		tile.White, tile.Green, tile.Red,
	}
	distinct, hasPair := 0, 0
	for _, t := range yaochuu {
		if hand[t] >= 1 {
			distinct++
		}
		if hand[t] >= 2 {
			hasPair = 1
		}
	}
	return 13 - distinct - hasPair
}

// bestDecompose returns the (sets, partials) decomposition of hand
// that maximises 2*sets + partials. Suits are independent, so we
// process each suit individually plus honors.
func bestDecompose(hand *[tile.NumKinds]int) (sets, partials int) {
	// Suited tiles: per-suit recursive search.
	for suit := 0; suit < 3; suit++ {
		var s9 [9]int
		copy(s9[:], hand[suit*9:suit*9+9])
		ss, pp := suitDecompose(&s9, 0)
		sets += ss
		partials += pp
	}
	// Honors: only triplets and pairs (no runs).
	for i := 27; i < tile.NumKinds; i++ {
		c := hand[i]
		switch {
		case c >= 3:
			sets++
			if c == 4 {
				// 4-of-a-kind: triplet + nothing useful for shanten
				// (the 4th would be a kan but kan is a meld declaration).
			}
		case c == 2:
			partials++
		}
	}
	return
}

// suitDecompose finds the (sets, partials) decomposition of a single
// 9-tile suit vector that maximises 2*sets + partials.
func suitDecompose(s *[9]int, start int) (int, int) {
	for start < 9 && s[start] == 0 {
		start++
	}
	if start >= 9 {
		return 0, 0
	}

	bestS, bestP := 0, 0
	score := func(a, b int) int { return 2*a + b }
	consider := func(a, b int) {
		if score(a, b) > score(bestS, bestP) {
			bestS, bestP = a, b
		}
	}

	// Triplet at start.
	if s[start] >= 3 {
		s[start] -= 3
		a, b := suitDecompose(s, start)
		s[start] += 3
		consider(1+a, b)
	}
	// Run start, start+1, start+2.
	if start <= 6 && s[start+1] > 0 && s[start+2] > 0 {
		s[start]--
		s[start+1]--
		s[start+2]--
		a, b := suitDecompose(s, start)
		s[start]++
		s[start+1]++
		s[start+2]++
		consider(1+a, b)
	}
	// Partial: pair (could become triplet).
	if s[start] >= 2 {
		s[start] -= 2
		a, b := suitDecompose(s, start)
		s[start] += 2
		consider(a, 1+b)
	}
	// Partial: ryanmen / penchan (start, start+1).
	if start <= 7 && s[start+1] > 0 {
		s[start]--
		s[start+1]--
		a, b := suitDecompose(s, start)
		s[start]++
		s[start+1]++
		consider(a, 1+b)
	}
	// Partial: kanchan (start, start+2).
	if start <= 6 && s[start+2] > 0 {
		s[start]--
		s[start+2]--
		a, b := suitDecompose(s, start)
		s[start]++
		s[start+2]++
		consider(a, 1+b)
	}
	// Float: drop one tile at start.
	s[start]--
	a, b := suitDecompose(s, start)
	s[start]++
	consider(a, b)

	return bestS, bestP
}

// AdvancingTiles returns the list of tile kinds that, if drawn, would
// strictly reduce the shanten of `hand` (subject to per-kind 4-tile
// limit minus visible counts).
//
// `seen` is the count of each tile kind already visible to the player
// (own hand, river, melds, dora indicators) — drawing a kind that
// already has 4 copies visible can't help.
//
// shantenFn picks the rule (Of for Riichi, OfSichuan for Sichuan).
func AdvancingTiles(
	hand [tile.NumKinds]int,
	melds int,
	seen [tile.NumKinds]int,
	shantenFn func([tile.NumKinds]int, int) int,
) []tile.Tile {
	current := shantenFn(hand, melds)
	var out []tile.Tile
	for i := 0; i < tile.NumKinds; i++ {
		if seen[i] >= 4 {
			continue
		}
		hand[i]++
		next := shantenFn(hand, melds)
		hand[i]--
		if next < current {
			out = append(out, tile.Tile(i))
		}
	}
	return out
}
