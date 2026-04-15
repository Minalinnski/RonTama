package tui

import (
	"strings"
	"testing"

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
	st.Discards[0] = tile.MustParseHand("1s 2s 9p")
	st.Discards[1] = tile.MustParseHand("1m 2m")
	st.Discards[2] = tile.MustParseHand("9p")
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

	for _, want := range []string{"Dealer", "Wall:", "You (East)", "缺", "drew 5p"} {
		if !strings.Contains(out, want) {
			t.Errorf("rendered view missing %q", want)
		}
	}
}
