package tui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"

	"github.com/Minalinnski/RonTama/internal/tile"
)

// suitStyle returns the foreground style for a tile's suit.
func suitStyle(t tile.Tile) lipgloss.Style {
	switch t.Suit() {
	case tile.SuitMan:
		return lipgloss.NewStyle().Foreground(manColor)
	case tile.SuitPin:
		return lipgloss.NewStyle().Foreground(pinColor)
	case tile.SuitSou:
		return lipgloss.NewStyle().Foreground(souColor)
	default:
		return lipgloss.NewStyle().Foreground(honorColor)
	}
}

// renderTileCompact renders a tile as a colored 2-3 char inline token
// like "1m" or "東". Used in rivers and melds where space matters.
func renderTileCompact(t tile.Tile) string {
	return suitStyle(t).Render(t.String())
}

// renderTileBox renders a tile as a small bordered box, used in the
// concealed hand area where the player needs to "see" each tile.
//
// 3-wide box: ┌──┐
//             │1m│
//             └──┘
func renderTileBox(t tile.Tile, selected bool) string {
	body := suitStyle(t).Render(fmt.Sprintf("%-2s", t.String()))
	box := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(chromeColor).
		Padding(0, 0).
		Render(body)
	if selected {
		box = selectedTileStyle.Render(box)
	}
	return box
}

// renderHandConcealed renders the concealed hand horizontally with tile
// boxes; the optional drawn tile is shown separated by a space and a
// vertical bar.
func renderHandConcealed(tiles []tile.Tile, drawn *tile.Tile, selectedIdx int) string {
	parts := make([]string, 0, len(tiles)+2)
	for i, t := range tiles {
		parts = append(parts, renderTileBox(t, i == selectedIdx))
	}
	if drawn != nil {
		parts = append(parts, lipgloss.NewStyle().Foreground(chromeColor).Render(" │ "))
		parts = append(parts, renderTileBox(*drawn, selectedIdx == len(tiles)))
	}
	return lipgloss.JoinHorizontal(lipgloss.Top, parts...)
}

// renderRiver renders a discard pile as compact tokens with line wrap.
func renderRiver(tiles []tile.Tile, maxPerLine int) string {
	if len(tiles) == 0 {
		return chromeStyle.Render("(empty)")
	}
	var sb strings.Builder
	for i, t := range tiles {
		if i > 0 {
			if i%maxPerLine == 0 {
				sb.WriteByte('\n')
			} else {
				sb.WriteByte(' ')
			}
		}
		sb.WriteString(renderTileCompact(t))
	}
	return sb.String()
}

// renderMelds renders declared melds as bracketed groups.
func renderMelds(melds []tile.Meld) string {
	if len(melds) == 0 {
		return chromeStyle.Render("-")
	}
	var parts []string
	for _, m := range melds {
		var inner []string
		for _, t := range m.Tiles {
			inner = append(inner, renderTileCompact(t))
		}
		parts = append(parts, "["+strings.Join(inner, " ")+"]")
	}
	return strings.Join(parts, " ")
}

// renderSuit returns the suit's CJK character.
func renderSuit(s tile.Suit) string {
	switch s {
	case tile.SuitMan:
		return "萬"
	case tile.SuitPin:
		return "筒"
	case tile.SuitSou:
		return "索"
	case tile.SuitWind:
		return "(未定)"
	default:
		return "?"
	}
}

// seatLabel maps seat 0..3 to East/South/West/North.
func seatLabel(seat int) string {
	return []string{"East", "South", "West", "North"}[seat%4]
}
