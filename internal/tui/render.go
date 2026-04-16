package tui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/mattn/go-runewidth"

	"github.com/Minalinnski/RonTama/internal/tile"
)

// suitStyle returns the foreground style for a tile — used for the
// face (character) rendering. Dragons split to their own colours so
// 中/發/白 match the real tile faces.
func suitStyle(t tile.Tile) lipgloss.Style {
	s := lipgloss.NewStyle()
	switch t.Suit() {
	case tile.SuitMan:
		return s.Foreground(manColor)
	case tile.SuitPin:
		return s.Foreground(pinColor)
	case tile.SuitSou:
		return s.Foreground(souColor)
	case tile.SuitWind:
		return s.Foreground(windColor)
	case tile.SuitDragon:
		switch t {
		case tile.Red:
			return s.Foreground(redDragonColor)
		case tile.Green:
			return s.Foreground(greenDragonColor)
		case tile.White:
			return s.Foreground(whiteDragonColor)
		}
	}
	return s.Foreground(chromeColor)
}

// tileFaceStyle returns the style used inside a tile box: the suit
// foreground, with a background determined by the active TileStyle.
func tileFaceStyle(t tile.Tile) lipgloss.Style {
	base := suitStyle(t)
	switch currentTileStyle {
	case TileStyleIvory, TileStyleSolid:
		return base.Background(tileFaceColor)
	}
	return base
}

// currentTileStyle is the globally-active tile render mode. Set by
// PlayModel on every 's' key press. Package-level to avoid threading
// a style argument through every render helper; TUI is inherently
// single-threaded (one tea.Program per process) so contention isn't
// a concern.
var currentTileStyle TileStyle = TileStylePlain

// renderTileCompact renders a tile as a colored 2-3 char inline token
// like "1m" or "東". Used in rivers and melds where space matters.
func renderTileCompact(t tile.Tile) string {
	return suitStyle(t).Render(t.String())
}

// renderTileBox renders a tile as a small bordered box with an ivory
// face and a suit-coloured character: looks like a real mahjong tile.
//
// 3-wide box: ╭──╮
//             │1m│    ← ivory face, red/blue/green ink
//             ╰──╯
//
// Body is always exactly 2 terminal columns — "1m" is 2 ASCII cols,
// "東" is 1 rune × 2 CJK cols. Using %-2s pads by RUNE count, which
// adds an extra space after single-rune CJK (東 → "東 ") and makes
// wind/dragon tiles a column wider than suit tiles. We pad using
// runewidth.StringWidth instead, which measures visual columns.
//
// Selected state switches the border colour to gold instead of
// re-styling the multi-line body (which breaks JoinHorizontal alignment
// in Lipgloss).
func renderTileBox(t tile.Tile, selected bool) string {
	body := tileFaceStyle(t).Render(padToCols(t.String(), 2))
	bc := chromeColor
	if selected {
		bc = selectedColor
	}
	style := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(bc).
		Padding(0, 0)
	// Solid style: extend the ivory background over the entire box
	// (border characters included) to remove the bg/border seam that
	// looks like a double edge on some terminals.
	if currentTileStyle == TileStyleSolid {
		style = style.
			BorderBackground(tileFaceColor).
			Background(tileFaceColor)
	}
	return style.Render(body)
}

// padToCols right-pads s with spaces so its visual terminal width is
// exactly `cols`. Measures visual width with go-runewidth (CJK chars
// count as 2 columns each).
func padToCols(s string, cols int) string {
	w := runewidth.StringWidth(s)
	if w >= cols {
		return s
	}
	return s + strings.Repeat(" ", cols-w)
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

// renderHandMulti renders the hand with multiple tiles highlighted
// (used by 换三张 selection). selectedIdxs lists the tile-indices to mark.
func renderHandMulti(tiles []tile.Tile, selectedIdxs []int) string {
	picked := map[int]bool{}
	for _, i := range selectedIdxs {
		picked[i] = true
	}
	keys := "123456789abcde"
	var tileParts, keyParts []string
	for i, t := range tiles {
		tileParts = append(tileParts, renderTileBox(t, picked[i]))
		ch := byte('?')
		if i < len(keys) {
			ch = keys[i]
		}
		keyParts = append(keyParts, lipgloss.NewStyle().
			Foreground(chromeColor).
			Render(fmt.Sprintf(" %c  ", ch)))
	}
	hand := lipgloss.JoinHorizontal(lipgloss.Top, tileParts...)
	keyRow := lipgloss.JoinHorizontal(lipgloss.Top, keyParts...)
	return lipgloss.JoinVertical(lipgloss.Left, hand, keyRow)
}

// renderHandRiichiSelect is like renderHandSplit but shows tiles in 3
// visual states: selected (gold border), riichi-valid (pink border),
// and invalid (dimmed border). Used during the riichi selection sub-mode.
func renderHandRiichiSelect(sorted []tile.Tile, drawn *tile.Tile, selectedIdx int, valid []bool) string {
	keys := "123456789abcde"
	var tileParts, keyParts []string
	isValid := func(i int) bool { return i < len(valid) && valid[i] }
	for i, t := range sorted {
		bc := dimColor // invalid
		if i == selectedIdx {
			bc = selectedColor
		} else if isValid(i) {
			bc = winColor // pink = valid riichi candidate
		}
		tileParts = append(tileParts, renderTileBoxColor(t, bc))
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
		idx := len(sorted)
		bc := dimColor
		if idx == selectedIdx {
			bc = selectedColor
		} else if isValid(idx) {
			bc = winColor
		}
		tileParts = append(tileParts, renderTileBoxColor(*drawn, bc))
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

// renderTileBoxColor renders a tile box with a caller-specified border colour
// (used by riichi-selection mode for valid/invalid/selected states).
func renderTileBoxColor(t tile.Tile, bc lipgloss.Color) string {
	body := tileFaceStyle(t).Render(padToCols(t.String(), 2))
	style := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(bc).
		Padding(0, 0)
	if currentTileStyle == TileStyleSolid {
		style = style.
			BorderBackground(tileFaceColor).
			Background(tileFaceColor)
	}
	return style.Render(body)
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
