package tui

import (
	"strings"
	"testing"

	"github.com/charmbracelet/lipgloss"

	"github.com/Minalinnski/RonTama/internal/game"
	"github.com/Minalinnski/RonTama/internal/rules/sichuan"
	"github.com/Minalinnski/RonTama/internal/tile"
)

// TestSnapshot_FullView verifies the rendered model contains the
// expected layout regions and seat labels in the right positions.
//
// Run `go test ./internal/tui/ -run Snapshot -v` to print the rendered
// view for visual inspection.
func TestSnapshot_FullView(t *testing.T) {
	rule := sichuan.New()
	st, err := game.NewState(rule, 0)
	if err != nil {
		t.Fatal(err)
	}
	for i := 0; i < game.NumPlayers; i++ {
		st.Players[i].Dingque = []tile.Suit{tile.SuitSou, tile.SuitMan, tile.SuitPin, tile.SuitMan}[i]
	}
	// Mid-game-ish discard piles with enough length to test wrapping.
	st.Discards[0] = tile.MustParseHand("1s 2s 9p 3m 7s 4p 8s")
	st.Discards[1] = tile.MustParseHand("1m 2m 9p 5s 7p 4m 9m 8m 3p")
	st.Discards[2] = tile.MustParseHand("9p 5p 8m 7s 1s 2s")
	st.Discards[3] = tile.MustParseHand("3m 4m 6p 7s 9s")
	st.Players[2].Hand.Melds = []tile.Meld{
		{Kind: tile.Pon, Tiles: []tile.Tile{tile.Sou5, tile.Sou5, tile.Sou5}, From: 1},
	}
	jd := tile.Pin5
	st.Players[0].Hand.Add(jd)
	st.Players[0].JustDrew = &jd

	m := NewPlayModel(rule)
	m.width, m.height = 120, 40
	m.state = st
	resp := make(chan any, 1)
	m.prompt = &HumanPromptMsg{Kind: "draw", View: st.View(0), Respond: resp}
	m.selected = 5

	out := m.View()
	t.Logf("\n%s", out)

	// Verify CJK honors stay 4-col tile boxes like the ASCII suit tiles.
	tiles := []tile.Tile{tile.Man1, tile.Pin5, tile.Sou9, tile.East, tile.South, tile.Red, tile.White, tile.Green}
	for _, tl := range tiles {
		box := renderTileBox(tl, false)
		// Each rendered box should have the same per-line visual width.
		lines := strings.Split(box, "\n")
		if len(lines) < 3 {
			t.Fatalf("tile box for %s has <3 lines: %q", tl, box)
		}
		w0 := lipgloss.Width(lines[0])
		w1 := lipgloss.Width(lines[1])
		w2 := lipgloss.Width(lines[2])
		if w0 != w1 || w1 != w2 {
			t.Errorf("tile %s box lines differ in width: %d / %d / %d", tl, w0, w1, w2)
		}
	}
	// All tile boxes (ASCII + CJK) should be the same width.
	widths := map[tile.Tile]int{}
	for _, tl := range tiles {
		box := renderTileBox(tl, false)
		lines := strings.Split(box, "\n")
		widths[tl] = lipgloss.Width(lines[1])
	}
	var want int
	for tl, w := range widths {
		if want == 0 {
			want = w
			continue
		}
		if w != want {
			t.Errorf("tile %s width %d differs from %d (other tiles)", tl, w, want)
		}
	}

	for _, want := range []string{"Dealer", "Wall:", "YOU (East)", "缺", "drew 5p"} {
		if !strings.Contains(out, want) {
			t.Errorf("rendered view missing %q", want)
		}
	}
}
