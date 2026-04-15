package riichi

import (
	"github.com/Minalinnski/RonTama/internal/rules"
	"github.com/Minalinnski/RonTama/internal/tile"
)

// computeFu computes the Riichi fu (符) for the hand.
//
// Standard formula (simplified MVP — full pinfu / kanchan-wait detail
// would require structural decomposition; we apply the broad strokes):
//
//	chiitoitsu:                 25 fu (fixed)
//	base:                        20 fu
//	  + tsumo:                   +2 fu (unless pinfu — we don't detect pinfu here)
//	  + ron concealed:           +10 fu
//	  + each minkou (open trip): +2 (terminal/honor: +4)
//	  + each ankou (concealed):  +4 (terminal/honor: +8)
//	  + each minkan:             +8 (terminal/honor: +16)
//	  + each ankan:              +16 (terminal/honor: +32)
//	  + yakuhai pair:            +2
//	  + (waits ignored — kanchan/penchan/tanki bonus +2 omitted)
//
// Result is rounded UP to the next multiple of 10.
func computeFu(c [tile.NumKinds]int, melds []tile.Meld, winTile tile.Tile, ctx rules.WinContext, concealed bool, chiitoi bool) int {
	if chiitoi {
		return 25
	}

	fu := 20
	if ctx.Tsumo {
		fu += 2
	}
	if !ctx.Tsumo && concealed {
		fu += 10
	}

	// Concealed triplets / pairs / honor pair bonuses from c (post-win).
	for i := 0; i < tile.NumKinds; i++ {
		t := tile.Tile(i)
		switch c[i] {
		case 3:
			// Concealed triplet (ankou). Treated as concealed unless this is
			// the winning tile and it was a ron — then the triplet was
			// completed by the discard, so it counts as minkou for fu.
			if !ctx.Tsumo && t == winTile {
				fu += koukouFu(t, false)
			} else {
				fu += koukouFu(t, true)
			}
		case 2:
			if t.IsDragon() || t == ctx.RoundWind || t == ctx.SeatWind() {
				fu += 2 // yakuhai pair
			}
		}
	}

	// Meld bonuses.
	for _, m := range melds {
		bonus := 0
		isAnkou := false
		switch m.Kind {
		case tile.Pon:
			bonus = 2
		case tile.Kan:
			bonus = 8
		case tile.ConcealedKan:
			bonus = 16
			isAnkou = true
		case tile.AddedKan:
			bonus = 8 // promoted from open pon
		case tile.Chi:
			bonus = 0
		}
		if bonus > 0 && len(m.Tiles) > 0 && m.Tiles[0].IsTerminal() {
			bonus *= 2
		}
		if isAnkou && len(m.Tiles) > 0 {
			// already doubled if terminal above
		}
		fu += bonus
	}

	return roundUp10(fu)
}

// koukouFu: fu for a concealed triplet by tile rank.
//
//	concealed simple:   +4
//	concealed terminal: +8
//	open simple:        +2
//	open terminal:      +4
func koukouFu(t tile.Tile, concealed bool) int {
	base := 2
	if concealed {
		base = 4
	}
	if t.IsTerminal() {
		base *= 2
	}
	return base
}

// roundUp10 rounds n up to the next multiple of 10.
func roundUp10(n int) int {
	if n%10 == 0 {
		return n
	}
	return n + (10 - n%10)
}
