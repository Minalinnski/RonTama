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
// 3-wide box: ╭──╮
//             │1m│
//             ╰──╯
//
// Selected state changes the border color rather than wrapping the box in
// another style — wrapping a multi-line border with Underline/Bold breaks
// JoinHorizontal alignment in Lipgloss.
func renderTileBox(t tile.Tile, selected bool) string {
	body := suitStyle(t).Render(fmt.Sprintf("%-2s", t.String()))
	bc := chromeColor
	if selected {
		bc = selectedColor
	}
	return lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(bc).
		Padding(0, 0).
		Render(body)
}

// renderHandConcealed renders the concealed hand horizontally with tile
// boxes. drawnIdx is the index (within tiles) of the tile that was
// just drawn this turn — it gets visually separated by a space gap and
// vertical bar so the player can see what they just picked up.
//
// selectedIdx is the cursor position; if it equals an index, that tile
// is highlighted with the selected border color. -1 = no selection.
func renderHandConcealed(tiles []tile.Tile, drawnIdx, selectedIdx int) string {
	parts := make([]string, 0, len(tiles)+2)
	for i, t := range tiles {
		if i == drawnIdx && i > 0 {
			parts = append(parts, lipgloss.NewStyle().Foreground(chromeColor).Render(" "))
		}
		parts = append(parts, renderTileBox(t, i == selectedIdx))
	}
	return lipgloss.JoinHorizontal(lipgloss.Top, parts...)
}

// renderHandWithKeyHints renders the hand with single-character key
// hints below each tile (digits 1-9, then a/b/c/d/e for 10-14).
//
// Returns the multi-line string ready to print under a header.
func renderHandWithKeyHints(tiles []tile.Tile, drawnIdx, selectedIdx int) string {
	hand := renderHandConcealed(tiles, drawnIdx, selectedIdx)
	// Build the key-hint row. Each tile box is 4 chars wide; we centre a
	// single character under each.
	keys := "123456789abcde"
	var hintParts []string
	for i := range tiles {
		if i == drawnIdx && i > 0 {
			hintParts = append(hintParts, " ")
		}
		ch := byte('?')
		if i < len(keys) {
			ch = keys[i]
		}
		hintParts = append(hintParts, lipgloss.NewStyle().
			Foreground(chromeColor).
			Render(fmt.Sprintf(" %c  ", ch)))
	}
	hintRow := lipgloss.JoinHorizontal(lipgloss.Top, hintParts...)
	return lipgloss.JoinVertical(lipgloss.Left, hand, hintRow)
}

// renderHandSplit renders the concealed hand with the drawn tile
// segregated on the FAR RIGHT — separated by a small gap, not sorted
// into the rest. selectedIdx points into the combined slice
// [sorted... drawn], so selectedIdx == len(sorted) means the drawn
// tile is highlighted.
//
// Below each tile a key-hint character is rendered (1-9, a-e).
func renderHandSplit(sorted []tile.Tile, drawn *tile.Tile, selectedIdx int) string {
	keys := "123456789abcde"
	var tileParts, keyParts []string
	for i, t := range sorted {
		tileParts = append(tileParts, renderTileBox(t, i == selectedIdx))
		ch := byte('?')
		if i < len(keys) {
			ch = keys[i]
		}
		keyParts = append(keyParts, lipgloss.NewStyle().
			Foreground(chromeColor).
			Render(fmt.Sprintf(" %c  ", ch)))
	}
	if drawn != nil {
		tileParts = append(tileParts, lipgloss.NewStyle().Foreground(chromeColor).Render("  "))
		tileParts = append(tileParts, renderTileBox(*drawn, selectedIdx == len(sorted)))
		keyParts = append(keyParts, "  ")
		ch := byte('?')
		if len(sorted) < len(keys) {
			ch = keys[len(sorted)]
		}
		keyParts = append(keyParts, lipgloss.NewStyle().
			Foreground(chromeColor).
			Render(fmt.Sprintf(" %c  ", ch)))
	}
	hand := lipgloss.JoinHorizontal(lipgloss.Top, tileParts...)
	keyRow := lipgloss.JoinHorizontal(lipgloss.Top, keyParts...)
	return lipgloss.JoinVertical(lipgloss.Left, hand, keyRow)
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
