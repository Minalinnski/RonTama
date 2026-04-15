package riichi

import (
	"testing"

	"github.com/Minalinnski/RonTama/internal/rules"
	"github.com/Minalinnski/RonTama/internal/tile"
)

func TestPinfu_Detected_Ryanmen(t *testing.T) {
	r := New()
	// 13-tile hand: 234m + 567p + 234s + 78s + 22p = 3+3+3+2+2 = 13.
	// Win on 6s completes 678s (run start=5, winNum=5 → left-edge ryanmen).
	hand := mustHand("234m 567p 234s 78s 22p")
	c := ctx(0, 0, false)
	winTile := tile.Sou6
	if !r.CanWin(hand, winTile, c) {
		t.Fatalf("expected valid win, got rejection")
	}
	score := r.ScoreWin(hand, winTile, c)
	hasPinfu := false
	for _, p := range score.Patterns {
		if p == "平和" {
			hasPinfu = true
		}
	}
	if !hasPinfu {
		t.Errorf("expected 平和 in patterns: %v", score.Patterns)
	}
}

func TestPinfu_Rejected_YakuhaiPair(t *testing.T) {
	r := New()
	// Pair is white dragon → pinfu rejected even though the rest fits.
	// Tsumo so menzen tsumo gives a valid yaku.
	hand := mustHand("234m 567p 234s 78s 55z") // 5z = White
	c := ctx(0, 0, true)                       // tsumo for valid yaku
	winTile := tile.Sou6
	if !r.CanWin(hand, winTile, c) {
		t.Fatalf("hand should be a valid win with menzen tsumo")
	}
	score := r.ScoreWin(hand, winTile, c)
	for _, p := range score.Patterns {
		if p == "平和" {
			t.Errorf("yakuhai-pair shouldn't be pinfu: %v", score.Patterns)
		}
	}
}

func TestPinfu_Rejected_Toitoi(t *testing.T) {
	r := New()
	// All triplets → toitoi, not pinfu.
	hand := mustHand("111m 222m 333p 444p 5s") // tanki wait on 5s
	c := ctx(0, 0, true)
	if !r.CanWin(hand, tile.Sou5, c) {
		t.Skip("not a valid win — skipping")
	}
	score := r.ScoreWin(hand, tile.Sou5, c)
	for _, p := range score.Patterns {
		if p == "平和" {
			t.Errorf("toitoi shouldn't be pinfu: %v", score.Patterns)
		}
	}
}

func TestPinfu_FuOverride(t *testing.T) {
	// pinfu tsumo = 20 fu fixed
	got := computeFu([tile.NumKinds]int{}, nil, tile.Pin1, rules.WinContext{Tsumo: true}, true, false, true)
	if got != 20 {
		t.Errorf("pinfu tsumo fu = %d, want 20", got)
	}
	// pinfu ron = 30 fu fixed
	got = computeFu([tile.NumKinds]int{}, nil, tile.Pin1, rules.WinContext{Tsumo: false}, true, false, true)
	if got != 30 {
		t.Errorf("pinfu ron fu = %d, want 30", got)
	}
}

func TestPinfu_Penchan_NotRyanmen(t *testing.T) {
	r := New()
	// 13-tile hand: 12m + 567p + 234s + 678s + 22p = 2+3+3+3+2 = 13.
	// Win on Man3 completes 123m. start=0, winNum=2 → "right end".
	// Ryanmen requires start >= 1, but start=0 → penchan. Not pinfu.
	// Use tsumo so menzen tsumo gives a valid yaku.
	hand := mustHand("12m 567p 234s 678s 22p")
	c := ctx(0, 0, true)
	if !r.CanWin(hand, tile.Man3, c) {
		t.Fatalf("hand should be a valid win with menzen tsumo")
	}
	score := r.ScoreWin(hand, tile.Man3, c)
	for _, p := range score.Patterns {
		if p == "平和" {
			t.Errorf("penchan completion shouldn't be pinfu: %v", score.Patterns)
		}
	}
}
