package sichuan

import (
	"testing"

	"github.com/Minalinnski/RonTama/internal/rules"
	"github.com/Minalinnski/RonTama/internal/tile"
)

func mustHand(s string) tile.Hand {
	return tile.NewHand(tile.MustParseHand(s))
}

func TestRule_Basics(t *testing.T) {
	r := New()
	if r.Name() == "" {
		t.Error("Name empty")
	}
	if len(r.TileKinds()) != 27 {
		t.Errorf("TileKinds len=%d, want 27", len(r.TileKinds()))
	}
	if !r.RequiresDingque() {
		t.Error("Sichuan should require dingque")
	}
	if r.AllowsChi() {
		t.Error("Sichuan should not allow chi")
	}
}

func TestCanWin_PlainHu(t *testing.T) {
	r := New()
	// 13 concealed tiles, won on the 14th.
	hand := mustHand("123m 456m 789m 123p 1p")
	winT := tile.Pin1
	ctx := rules.WinContext{WinningTile: winT, Tsumo: true, Seat: 0, Dingque: tile.SuitSou}
	if !r.CanWin(hand, winT, ctx) {
		t.Errorf("expected win on plain hand")
	}
}

func TestCanWin_DingqueViolation(t *testing.T) {
	r := New()
	// Hand has sou but dingque is sou — can't win.
	hand := mustHand("123m 456m 789m 11p 1s")
	winT := tile.Pin1
	ctx := rules.WinContext{WinningTile: winT, Tsumo: true, Seat: 0, Dingque: tile.SuitSou}
	if r.CanWin(hand, winT, ctx) {
		t.Errorf("should reject win with dingque violation")
	}
}

func TestCanWin_SevenPairs(t *testing.T) {
	r := New()
	hand := mustHand("11m 22m 33m 44m 55m 66m 7m") // 13 tiles, 6 pairs + 1 single
	winT := tile.Man7
	ctx := rules.WinContext{WinningTile: winT, Tsumo: true, Seat: 0, Dingque: tile.SuitSou}
	if !r.CanWin(hand, winT, ctx) {
		t.Errorf("expected seven-pairs win")
	}
}

func TestScoreWin_Patterns(t *testing.T) {
	r := New()
	// 七对: all single suit -> 清七对
	hand := mustHand("11m 22m 33m 44m 55m 66m 7m")
	score := r.ScoreWin(hand, tile.Man7, rules.WinContext{
		WinningTile: tile.Man7, Tsumo: true, Seat: 0, Dingque: tile.SuitSou,
	})
	want := "清七对"
	found := false
	for _, p := range score.Patterns {
		if p == want {
			found = true
		}
	}
	if !found {
		t.Errorf("expected pattern %q in %v", want, score.Patterns)
	}
	if score.Fan < 8 {
		t.Errorf("清七对 should be at least 8 fan, got %d", score.Fan)
	}
}

func TestScoreWin_PingHu(t *testing.T) {
	r := New()
	// plain win, ron from another player
	hand := mustHand("123m 456m 789m 123p 1p")
	score := r.ScoreWin(hand, tile.Pin1, rules.WinContext{
		WinningTile: tile.Pin1, Tsumo: false, Seat: 0, Dingque: tile.SuitSou,
	})
	if score.Fan < 1 {
		t.Errorf("Ping hu should be >= 1 fan, got %d", score.Fan)
	}
	hasPingHu := false
	for _, p := range score.Patterns {
		if p == "平胡" {
			hasPingHu = true
		}
	}
	if !hasPingHu {
		t.Errorf("expected 平胡 in patterns, got %v", score.Patterns)
	}
}

func TestScoreWin_TsumoBonus(t *testing.T) {
	r := New()
	hand := mustHand("123m 456m 789m 123p 1p")
	scoreRon := r.ScoreWin(hand, tile.Pin1, rules.WinContext{
		WinningTile: tile.Pin1, Tsumo: false, Seat: 0, Dingque: tile.SuitSou,
	})
	scoreTsumo := r.ScoreWin(hand, tile.Pin1, rules.WinContext{
		WinningTile: tile.Pin1, Tsumo: true, Seat: 0, Dingque: tile.SuitSou,
	})
	if scoreTsumo.Fan != scoreRon.Fan+1 {
		t.Errorf("tsumo should add +1 fan: ron=%d tsumo=%d", scoreRon.Fan, scoreTsumo.Fan)
	}
}
