package riichi

import (
	"strings"
	"testing"

	"github.com/Minalinnski/RonTama/internal/rules"
	"github.com/Minalinnski/RonTama/internal/tile"
)

func mustHand(s string) tile.Hand {
	return tile.NewHand(tile.MustParseHand(s))
}

func ctx(seat, dealer int, tsumo bool) rules.WinContext {
	return rules.WinContext{Seat: seat, Dealer: dealer, Tsumo: tsumo, RoundWind: tile.East}
}

func TestRule_Basics(t *testing.T) {
	r := New()
	if r.Name() != "riichi" {
		t.Errorf("Name = %q", r.Name())
	}
	if len(r.TileKinds()) != 34 {
		t.Errorf("TileKinds len = %d, want 34", len(r.TileKinds()))
	}
	if !r.AllowsChi() {
		t.Error("Riichi should allow chi")
	}
	if r.RequiresDingque() {
		t.Error("Riichi should not require dingque")
	}
}

func TestCanWin_NoYakuRejected(t *testing.T) {
	r := New()
	// 13-tile concealed hand; winning tile completes 999p triplet.
	// Has terminals (1m, 9p) so no tanyao. Without riichi/tsumo there's no yaku → invalid win.
	hand := mustHand("123m 456p 789s 11m 99p")
	c := ctx(0, 0, false)
	c.Tsumo = false
	if r.CanWin(hand, tile.Pin9, c) {
		t.Errorf("expected no-yaku rejection (no riichi, no tsumo, no tanyao)")
	}
}

func TestCanWin_RiichiOnly(t *testing.T) {
	r := New()
	hand := mustHand("123m 456p 789s 11m 99p")
	c := ctx(0, 0, false)
	c.Riichi = true
	if !r.CanWin(hand, tile.Pin9, c) {
		t.Errorf("riichi yaku should make this a valid win")
	}
}

func TestScoreWin_TanyaoTsumo(t *testing.T) {
	r := New()
	// All simples win, tsumo.
	hand := mustHand("234m 456p 678s 234p 22s") // 13 tiles concealed; win = ?
	// adjust to 13 tiles + winning tile: 234m(3)+456p(3)+678s(3)+234p(3)+22s(2) = 14, need 13
	// Let's set hand to 13 tiles waiting for 22s pair completion.
	hand = mustHand("234m 456p 678s 234p 2s")
	c := ctx(0, 0, true)
	if !r.CanWin(hand, tile.Sou2, c) {
		t.Fatalf("expected tanyao+tsumo win")
	}
	score := r.ScoreWin(hand, tile.Sou2, c)
	hasTanyao := false
	hasTsumo := false
	for _, p := range score.Patterns {
		if p == "断幺九" {
			hasTanyao = true
		}
		if strings.Contains(p, "自摸") {
			hasTsumo = true
		}
	}
	if !hasTanyao {
		t.Errorf("expected 断幺九 in patterns: %v", score.Patterns)
	}
	if !hasTsumo {
		t.Errorf("expected 自摸 yaku in patterns: %v", score.Patterns)
	}
}

func TestScoreWin_Yakuhai_Dragon(t *testing.T) {
	r := New()
	// White dragon triplet + tanyao-ish other tiles.
	hand := mustHand("234m 456p 678s 555z 6s") // 555z = white dragon triplet, win on 6s
	// Wait — 5z is White (index 27+5-1=31 = White). Yes.
	c := ctx(0, 0, true)
	if !r.CanWin(hand, tile.Sou6, c) {
		t.Fatalf("expected yakuhai win, hand: %s", hand)
	}
	score := r.ScoreWin(hand, tile.Sou6, c)
	foundYakuhai := false
	for _, p := range score.Patterns {
		if strings.HasPrefix(p, "役牌") {
			foundYakuhai = true
		}
	}
	if !foundYakuhai {
		t.Errorf("expected yakuhai in %v", score.Patterns)
	}
}

func TestScoreWin_Chiitoitsu(t *testing.T) {
	r := New()
	hand := mustHand("11m 22m 33p 44p 55s 66s 7z")
	c := ctx(0, 0, true)
	if !r.CanWin(hand, tile.Red, c) {
		t.Fatalf("expected chiitoi win")
	}
	score := r.ScoreWin(hand, tile.Red, c)
	hasChiitoi := false
	for _, p := range score.Patterns {
		if p == "七対子" {
			hasChiitoi = true
		}
	}
	if !hasChiitoi {
		t.Errorf("expected 七対子 in %v", score.Patterns)
	}
}

func TestScoreWin_Kokushi(t *testing.T) {
	r := New()
	// 13 distinct yaochuu (tenpai for kokushi 13-way wait).
	hand := mustHand("19m 19p 19s 1234567z")
	c := ctx(0, 0, true)
	// Win on any yaochuu — pick 1m (already 1 in hand → 2 = pair).
	if !r.CanWin(hand, tile.Man1, c) {
		t.Fatalf("expected kokushi win")
	}
	score := r.ScoreWin(hand, tile.Man1, c)
	hasKokushi := false
	for _, p := range score.Patterns {
		if p == "国士無双" {
			hasKokushi = true
		}
	}
	if !hasKokushi {
		t.Errorf("expected 国士無双 in %v", score.Patterns)
	}
	if score.BasePts != 8000 {
		t.Errorf("yakuman base = %d, want 8000", score.BasePts)
	}
}

func TestDoraOf(t *testing.T) {
	cases := map[tile.Tile]tile.Tile{
		tile.Man1:  tile.Man2,
		tile.Man9:  tile.Man1,
		tile.Pin5:  tile.Pin6,
		tile.Sou9:  tile.Sou1,
		tile.East:  tile.South,
		tile.North: tile.East,
		tile.White: tile.Green,
		tile.Red:   tile.White,
	}
	for ind, want := range cases {
		if got := doraOf(ind); got != want {
			t.Errorf("doraOf(%s) = %s, want %s", ind, got, want)
		}
	}
}

func TestBasePoints(t *testing.T) {
	cases := []struct {
		han, fu, want int
	}{
		{1, 30, 240},  // 30 * 2^(1+2) = 240; non-dealer ron pays 4×base ≈ 1000
		{4, 30, 1920}, // 30 * 2^(4+2) = 1920 (just under mangan)
		{5, 0, 2000},  // mangan
		{6, 0, 3000},  // haneman
		{8, 0, 4000},  // baiman
		{11, 0, 6000}, // sanbaiman
		{13, 0, 8000}, // counted yakuman
	}
	for _, c := range cases {
		if got := basePoints(c.han, c.fu, nil); got != c.want {
			t.Errorf("basePoints(han=%d fu=%d) = %d, want %d", c.han, c.fu, got, c.want)
		}
	}
	if got := basePoints(0, 0, []string{"国士無双"}); got != 8000 {
		t.Errorf("yakuman basePoints = %d, want 8000", got)
	}
	if got := basePoints(0, 0, []string{"国士無双", "四暗刻"}); got != 16000 {
		t.Errorf("double yakuman = %d, want 16000", got)
	}
}

func TestComputeFu_Chiitoitsu(t *testing.T) {
	c := ctx(0, 0, true)
	// chiitoi=true bypass should return 25.
	got := computeFu([tile.NumKinds]int{}, nil, tile.Pin1, c, true, true)
	if got != 25 {
		t.Errorf("chiitoi fu = %d, want 25", got)
	}
}

func TestComputeFu_BaseRoundedUp(t *testing.T) {
	c := ctx(0, 0, true)
	// All sequences, no triplets, no kanchan -> base 20 + 2 (tsumo) = 22 -> 30
	hand := tile.Counts(tile.MustParseHand("123m 456p 789s 234m 22s"))
	got := computeFu(hand, nil, tile.Sou2, c, true, false)
	if got != 30 {
		t.Errorf("simple all-runs tsumo fu = %d, want 30", got)
	}
}
