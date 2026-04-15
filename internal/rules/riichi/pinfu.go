package riichi

import (
	"github.com/Minalinnski/RonTama/internal/rules"
	"github.com/Minalinnski/RonTama/internal/tile"
)

// isPinfu reports whether the winning hand qualifies for 平和.
//
// Conditions (Riichi standard):
//   - Hand is fully concealed (no melds at all — even ankans disqualify pinfu)
//   - Decomposable into 4 sequences + 1 pair (no triplets / kans)
//   - The pair is NOT a yakuhai (not a dragon, not the round wind, not the seat wind)
//   - The winning tile fills a ryanmen wait — i.e., the run containing
//     the winning tile is open at both ends; not kanchan, penchan,
//     tanki, or shanpon
//
// pinfu is valid for both ron and tsumo. The yaku is +1 han either way;
// the fu calculation differs (pinfu tsumo = 20 fu; pinfu ron concealed
// would be 30 fu — handled as a special case in computeFu).
func isPinfu(c [tile.NumKinds]int, winTile tile.Tile, melds []tile.Meld, ctx rules.WinContext) bool {
	if len(melds) > 0 {
		return false
	}
	if winTile.IsHonor() {
		return false
	}
	work := c
	for pair := 0; pair < tile.NumKinds; pair++ {
		if work[pair] < 2 {
			continue
		}
		t := tile.Tile(pair)
		if t.IsDragon() || t == ctx.RoundWind || t == ctx.SeatWind() {
			continue
		}
		work[pair] -= 2
		if otherSuitsRunnable(work, int(winTile)/9) && winSuitHasRyanmen(work, winTile) {
			work[pair] += 2
			return true
		}
		work[pair] += 2
	}
	return false
}

// otherSuitsRunnable: every honor count is 0, and every suit other than
// `winSuit` decomposes exactly into 3-tile runs.
func otherSuitsRunnable(c [tile.NumKinds]int, winSuit int) bool {
	for i := 27; i < tile.NumKinds; i++ {
		if c[i] != 0 {
			return false
		}
	}
	for s := 0; s < 3; s++ {
		if s == winSuit {
			continue
		}
		var v [9]int
		copy(v[:], c[s*9:s*9+9])
		if !exactRuns(v, 0) {
			return false
		}
	}
	return true
}

// winSuitHasRyanmen: in the suit containing winTile, find a run-only
// decomposition where the run that consumes winTile has it at a
// ryanmen (two-sided) position.
func winSuitHasRyanmen(c [tile.NumKinds]int, winTile tile.Tile) bool {
	suit := int(winTile) / 9
	if suit >= 3 {
		return false
	}
	var v [9]int
	copy(v[:], c[suit*9:suit*9+9])
	return tryRyanmenDecomposition(&v, 0, int(winTile)%9, false)
}

// tryRyanmenDecomposition attempts to consume v entirely with 3-tile
// runs where the run containing winNum is at a ryanmen position.
//
// foundRyanmen tracks whether the run containing winNum (already
// consumed) was ryanmen. Returns true once we've fully decomposed v
// AND foundRyanmen is true.
func tryRyanmenDecomposition(v *[9]int, start, winNum int, foundRyanmen bool) bool {
	for start < 9 && v[start] == 0 {
		start++
	}
	if start >= 9 {
		return foundRyanmen
	}
	if start > 6 {
		return false
	}
	if v[start+1] == 0 || v[start+2] == 0 {
		return false
	}
	v[start]--
	v[start+1]--
	v[start+2]--
	defer func() {
		v[start]++
		v[start+1]++
		v[start+2]++
	}()

	// Does this run contain winNum, and where?
	containsWin := winNum == start || winNum == start+1 || winNum == start+2
	if containsWin {
		// Pinfu requires this run to be ryanmen w.r.t. winNum.
		ryanmen := false
		switch {
		case winNum == start && start <= 5:
			// "left end" win: completed run a, a+1, a+2 by drawing a;
			// wait was (a+1, a+2), other valid completion was a+3
			// (valid iff a+3 is in 0..8, i.e., start <= 5).
			ryanmen = true
		case winNum == start+2 && start >= 1:
			// "right end" win: drew a+2, wait was (a, a+1), other side
			// would be a-1 (valid iff a-1 is in 0..8, i.e., start >= 1).
			ryanmen = true
		}
		// If it's not ryanmen here, this decomposition is poisoned
		// because winNum is consumed in a non-ryanmen run.
		if !ryanmen {
			return false
		}
		return tryRyanmenDecomposition(v, start, winNum, true)
	}
	return tryRyanmenDecomposition(v, start, winNum, foundRyanmen)
}

// exactRuns returns true if v can be exactly partitioned into 3-tile
// consecutive runs starting from index 0..6 each.
func exactRuns(v [9]int, start int) bool {
	for start < 9 && v[start] == 0 {
		start++
	}
	if start >= 9 {
		return true
	}
	if start > 6 {
		return false
	}
	if v[start+1] == 0 || v[start+2] == 0 {
		return false
	}
	v[start]--
	v[start+1]--
	v[start+2]--
	if exactRuns(v, start) {
		return true
	}
	v[start]++
	v[start+1]++
	v[start+2]++
	return false
}
