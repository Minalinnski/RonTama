package riichi

import (
	"github.com/Minalinnski/RonTama/internal/rules"
	"github.com/Minalinnski/RonTama/internal/tile"
)

// EvalResult is the yaku/han/fu/yakuman breakdown for a hand.
type EvalResult struct {
	Yaku    []string // names of regular yaku
	Yakuman []string // names of yakuman
	Han     int      // total han (regular + dora; ignored if Yakuman set)
	Fu      int      // rounded-up fu (Riichi standard: round to next 10)
}

// evaluate runs all yaku checks against the (concealed + winning tile +
// melds) shape. Caller has already confirmed the shape is a valid win.
func (Rule) evaluate(hand tile.Hand, winTile tile.Tile, ctx rules.WinContext, standard, chiitoi, kokushi bool) EvalResult {
	combined := hand.Concealed
	combined[winTile]++
	melds := hand.Melds
	concealed := len(melds) == 0 || allConcealedKan(melds)

	res := EvalResult{}

	// --- Yakuman first (override regular han) ---
	if kokushi {
		res.Yakuman = append(res.Yakuman, "国士無双")
	}
	if standard && isSuuankou(combined, melds, ctx) {
		res.Yakuman = append(res.Yakuman, "四暗刻")
	}
	if standard && isDaisangen(combined, melds) {
		res.Yakuman = append(res.Yakuman, "大三元")
	}
	if len(res.Yakuman) > 0 {
		// Yakuman wins: don't count regular yaku/dora.
		res.Fu = 30 // unused for yakuman scoring but populate for display
		return res
	}

	// --- Regular yaku ---
	addYaku := func(name string, han int) {
		res.Yaku = append(res.Yaku, name)
		res.Han += han
	}

	if ctx.Riichi {
		addYaku("立直", 1)
	}
	if ctx.DoubleRiichi {
		addYaku("ダブル立直", 2)
	}
	if ctx.Ippatsu {
		addYaku("一発", 1)
	}
	if ctx.Tsumo && concealed {
		addYaku("門前清自摸和", 1)
	}
	if ctx.LastTile {
		if ctx.Tsumo {
			addYaku("海底摸月", 1)
		} else {
			addYaku("河底撈魚", 1)
		}
	}
	if ctx.AfterKan {
		addYaku("嶺上開花", 1)
	}
	if ctx.KanGrab {
		addYaku("槍槓", 1)
	}

	// Tanyao: all simples (2..8 of suits).
	if isTanyao(combined, melds) {
		addYaku("断幺九", 1)
	}
	// Yakuhai: dragon triplets, round wind triplet, seat wind triplet.
	if standard {
		yh := countYakuhai(combined, melds, ctx)
		if yh > 0 {
			res.Han += yh
			res.Yaku = append(res.Yaku, "役牌×"+itoa(yh))
		}
	}

	if chiitoi {
		addYaku("七対子", 2)
	}
	if standard && isToitoi(combined, melds) {
		addYaku("対々和", 2)
	}

	// Honitsu / Chinitsu (count subset).
	switch suitPurity(combined, melds) {
	case puritySingle:
		bonus := 5
		if !concealed {
			bonus = 4
		}
		addYaku("清一色", bonus)
	case puritySingleWithHonor:
		bonus := 3
		if !concealed {
			bonus = 2
		}
		addYaku("混一色", bonus)
	}

	// Ittsu (1m2m3m + 4m5m6m + 7m8m9m all in same suit).
	if standard && hasIttsu(combined, melds) {
		bonus := 2
		if !concealed {
			bonus = 1
		}
		addYaku("一気通貫", bonus)
	}
	// Sanshoku doujun (same run in all 3 suits).
	if standard && hasSanshokuDoujun(combined, melds) {
		bonus := 2
		if !concealed {
			bonus = 1
		}
		addYaku("三色同順", bonus)
	}

	// --- Dora ---
	dora := countDora(combined, melds, ctx.DoraIndicators)
	if ctx.Riichi {
		dora += countDora(combined, melds, ctx.UraDoraIndicators)
	}
	if dora > 0 {
		res.Han += dora
		res.Yaku = append(res.Yaku, "ドラ×"+itoa(dora))
	}

	// --- Fu ---
	res.Fu = computeFu(combined, melds, winTile, ctx, concealed, chiitoi)
	return res
}

// allConcealedKan returns true if every meld is a concealed kan (the
// hand is still considered concealed for menzen yaku purposes).
func allConcealedKan(melds []tile.Meld) bool {
	for _, m := range melds {
		if m.Kind != tile.ConcealedKan {
			return false
		}
	}
	return true
}

// isTanyao: no terminals (1/9 of suits) or honors anywhere.
func isTanyao(c [tile.NumKinds]int, melds []tile.Meld) bool {
	for i := 0; i < tile.NumKinds; i++ {
		if c[i] > 0 && tile.Tile(i).IsTerminal() {
			return false
		}
	}
	for _, m := range melds {
		for _, t := range m.Tiles {
			if t.IsTerminal() {
				return false
			}
		}
	}
	return true
}

// countYakuhai: 1 han per dragon triplet + 1 per matching wind triplet.
func countYakuhai(c [tile.NumKinds]int, melds []tile.Meld, ctx rules.WinContext) int {
	count := 0
	check := func(t tile.Tile) {
		if c[t] >= 3 {
			count++
		}
		for _, m := range melds {
			if (m.Kind == tile.Pon || m.Kind == tile.Kan || m.Kind == tile.ConcealedKan) &&
				len(m.Tiles) >= 3 && m.Tiles[0] == t {
				count++
			}
		}
	}
	check(tile.White)
	check(tile.Green)
	check(tile.Red)
	if ctx.RoundWind != 0 || ctx.RoundWind == tile.East {
		// Default to East round if unset.
		rw := ctx.RoundWind
		if rw < tile.East || rw > tile.North {
			rw = tile.East
		}
		check(rw)
	}
	check(ctx.SeatWind())
	return count
}

// puritySingle*: helpers to detect chinitsu/honitsu.
type purity int

const (
	purityMixed purity = iota
	puritySingleWithHonor
	puritySingle
)

func suitPurity(c [tile.NumKinds]int, melds []tile.Meld) purity {
	suit := tile.Suit(255)
	hasHonor := false
	mark := func(t tile.Tile) bool {
		if t.IsHonor() {
			hasHonor = true
			return true
		}
		s := t.Suit()
		if suit == 255 {
			suit = s
			return true
		}
		return suit == s
	}
	for i := 0; i < tile.NumKinds; i++ {
		if c[i] > 0 && !mark(tile.Tile(i)) {
			return purityMixed
		}
	}
	for _, m := range melds {
		for _, t := range m.Tiles {
			if !mark(t) {
				return purityMixed
			}
		}
	}
	if suit == 255 {
		return purityMixed // pure honors (nothing fits standard chinitsu)
	}
	if hasHonor {
		return puritySingleWithHonor
	}
	return puritySingle
}

// isToitoi: 4 triplets/kans + 1 pair. Sequences disqualify.
func isToitoi(c [tile.NumKinds]int, melds []tile.Meld) bool {
	for _, m := range melds {
		if m.Kind == tile.Chi {
			return false
		}
	}
	pair, trip := 0, 0
	for _, n := range c {
		switch n {
		case 0:
		case 2:
			pair++
		case 3:
			trip++
		default:
			return false
		}
	}
	return pair == 1 && trip+len(melds) == 4
}

// hasIttsu: 1-2-3 + 4-5-6 + 7-8-9 of same suit, considering both
// concealed and meld'd runs.
func hasIttsu(c [tile.NumKinds]int, melds []tile.Meld) bool {
	for suit := 0; suit < 3; suit++ {
		// Build a suit count vector that includes melds.
		var s [9]int
		for n := 0; n < 9; n++ {
			s[n] = c[suit*9+n]
		}
		for _, m := range melds {
			if m.Kind != tile.Chi || len(m.Tiles) < 3 {
				continue
			}
			if int(m.Tiles[0])/9 != suit {
				continue
			}
			// add the run tiles
			for _, t := range m.Tiles {
				s[int(t)%9]++
			}
		}
		// Need to be able to consume 123 + 456 + 789 from s.
		ok := s[0] > 0 && s[1] > 0 && s[2] > 0 &&
			s[3] > 0 && s[4] > 0 && s[5] > 0 &&
			s[6] > 0 && s[7] > 0 && s[8] > 0
		if ok {
			return true
		}
	}
	return false
}

// hasSanshokuDoujun: same run number in man + pin + sou.
func hasSanshokuDoujun(c [tile.NumKinds]int, melds []tile.Meld) bool {
	suitVecs := [3][9]int{}
	for suit := 0; suit < 3; suit++ {
		for n := 0; n < 9; n++ {
			suitVecs[suit][n] = c[suit*9+n]
		}
	}
	for _, m := range melds {
		if m.Kind != tile.Chi || len(m.Tiles) < 3 {
			continue
		}
		suit := int(m.Tiles[0]) / 9
		for _, t := range m.Tiles {
			suitVecs[suit][int(t)%9]++
		}
	}
	for start := 0; start <= 6; start++ {
		ok := suitVecs[0][start] > 0 && suitVecs[0][start+1] > 0 && suitVecs[0][start+2] > 0 &&
			suitVecs[1][start] > 0 && suitVecs[1][start+1] > 0 && suitVecs[1][start+2] > 0 &&
			suitVecs[2][start] > 0 && suitVecs[2][start+1] > 0 && suitVecs[2][start+2] > 0
		if ok {
			return true
		}
	}
	return false
}

// isSuuankou: 4 concealed triplets/kans + pair. Tsumo required for
// pure suuankou; ron variant only counts as 4-han (toitoi+sanankou) —
// but for MVP we treat ron as still yakuman if shape qualifies (loose).
func isSuuankou(c [tile.NumKinds]int, melds []tile.Meld, ctx rules.WinContext) bool {
	// All declared melds must be concealed kans.
	for _, m := range melds {
		if m.Kind != tile.ConcealedKan {
			return false
		}
	}
	pair, trip := 0, 0
	for _, n := range c {
		switch n {
		case 0:
		case 2:
			pair++
		case 3:
			trip++
		default:
			return false
		}
	}
	return pair == 1 && trip+len(melds) == 4 && ctx.Tsumo
}

// isDaisangen: triplets/kans of all three dragons.
func isDaisangen(c [tile.NumKinds]int, melds []tile.Meld) bool {
	check := func(t tile.Tile) bool {
		if c[t] >= 3 {
			return true
		}
		for _, m := range melds {
			if (m.Kind == tile.Pon || m.IsKan()) && len(m.Tiles) >= 3 && m.Tiles[0] == t {
				return true
			}
		}
		return false
	}
	return check(tile.White) && check(tile.Green) && check(tile.Red)
}

// countDora: each dora indicator points to a dora tile (next in
// suit/wind/dragon cycle); count how many of those appear in the hand.
func countDora(c [tile.NumKinds]int, melds []tile.Meld, indicators []tile.Tile) int {
	dora := map[tile.Tile]int{}
	for _, ind := range indicators {
		dora[doraOf(ind)]++
	}
	if len(dora) == 0 {
		return 0
	}
	count := 0
	for tile, mult := range dora {
		count += c[tile] * mult
		for _, m := range melds {
			for _, t := range m.Tiles {
				if t == tile {
					count += mult
				}
			}
		}
	}
	return count
}

// doraOf: indicator -> dora tile (next in cycle).
//
//	1m..9m: 9m → 1m, others +1
//	pins / sou: same
//	winds (E S W N): cycle E→S→W→N→E
//	dragons (Wh Gr Rd): cycle Wh→Gr→Rd→Wh
func doraOf(ind tile.Tile) tile.Tile {
	switch {
	case ind.IsSuit():
		base := int(ind) - int(ind)%9
		next := (int(ind) - base + 1) % 9
		return tile.Tile(base + next)
	case ind == tile.East:
		return tile.South
	case ind == tile.South:
		return tile.West
	case ind == tile.West:
		return tile.North
	case ind == tile.North:
		return tile.East
	case ind == tile.White:
		return tile.Green
	case ind == tile.Green:
		return tile.Red
	case ind == tile.Red:
		return tile.White
	}
	return ind
}

// itoa is a tiny inlinable int-to-string for yaku names; standard
// strconv is fine but adds an import for one-character output.
func itoa(n int) string {
	if n < 10 {
		return string(rune('0' + n))
	}
	return "?"
}
