package tui

import (
	"strings"
	"testing"

	"github.com/Minalinnski/RonTama/internal/tile"
)

func TestRenderTileCompact_AllKindsRender(t *testing.T) {
	for i := 0; i < tile.NumKinds; i++ {
		out := renderTileCompact(tile.Tile(i))
		if out == "" {
			t.Errorf("renderTileCompact(%d) = empty", i)
		}
		if !strings.Contains(out, tile.Tile(i).String()) {
			t.Errorf("renderTileCompact(%d) doesn't contain %q: %q", i, tile.Tile(i).String(), out)
		}
	}
}

func TestRenderRiver_Empty(t *testing.T) {
	got := renderRiver(nil, 10)
	if !strings.Contains(got, "empty") {
		t.Errorf("empty river render = %q, want 'empty'", got)
	}
}

func TestRenderRiver_WrapsAtMax(t *testing.T) {
	tiles := tile.MustParseHand("123456789m 1234p")
	got := renderRiver(tiles, 5)
	if !strings.Contains(got, "\n") {
		t.Errorf("expected line break in long river, got %q", got)
	}
}

func TestRenderMelds_Empty(t *testing.T) {
	got := renderMelds(nil)
	if got == "" || !strings.Contains(got, "-") {
		t.Errorf("empty melds render = %q, want '-' marker", got)
	}
}

func TestRenderMelds_Pon(t *testing.T) {
	melds := []tile.Meld{{
		Kind:  tile.Pon,
		Tiles: []tile.Tile{tile.Pin5, tile.Pin5, tile.Pin5},
	}}
	got := renderMelds(melds)
	if !strings.Contains(got, "[") || !strings.Contains(got, "5p") {
		t.Errorf("pon render = %q, want bracketed 5p", got)
	}
}

func TestRenderSuit(t *testing.T) {
	cases := map[tile.Suit]string{
		tile.SuitMan: "萬",
		tile.SuitPin: "筒",
		tile.SuitSou: "索",
	}
	for s, want := range cases {
		if got := renderSuit(s); got != want {
			t.Errorf("renderSuit(%d) = %q, want %q", s, got, want)
		}
	}
}

func TestSeatLabel(t *testing.T) {
	want := []string{"East", "South", "West", "North"}
	for i, w := range want {
		if got := seatLabel(i); got != w {
			t.Errorf("seatLabel(%d) = %q, want %q", i, got, w)
		}
	}
}
