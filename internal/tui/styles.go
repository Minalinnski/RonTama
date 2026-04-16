package tui

import "github.com/charmbracelet/lipgloss"

// TileStyle enumerates the visual variants the player can cycle
// through with the 's' key during play.
type TileStyle int

const (
	// TileStylePlain: no background on the tile face, just the
	// rounded border + coloured character. Most compact and playable
	// on dark-themed terminals.
	TileStylePlain TileStyle = iota
	// TileStyleIvory: ivory tile-face background with coloured ink,
	// simulating a physical tile. Looks richer but some terminals
	// render the bg/border transition as a double edge.
	TileStyleIvory
	// TileStyleSolid: the ENTIRE box (border included) sits on the
	// ivory background, so there's no double-edge seam between border
	// and body. Feels like a "big card".
	TileStyleSolid

	tileStyleCount
)

// String for display in the status line.
func (s TileStyle) String() string {
	switch s {
	case TileStylePlain:
		return "plain"
	case TileStyleIvory:
		return "ivory"
	case TileStyleSolid:
		return "solid"
	}
	return "?"
}

// Next returns the next style in the cycle.
func (s TileStyle) Next() TileStyle {
	return (s + 1) % tileStyleCount
}

// Color palette: physical-mahjong look — tile faces are white, with
// ink colours per suit (萬紅 / 筒藍 / 索綠, 風黑, 中紅 / 發綠 / 白灰).
var (
	// Tile ink colours. Chosen to read well on a white tile background.
	manColor        = lipgloss.Color("#C51A1A") // 萬 — red
	pinColor        = lipgloss.Color("#1E5FB8") // 筒 — blue
	souColor        = lipgloss.Color("#1A7A3A") // 索 — green
	windColor       = lipgloss.Color("#101010") // 東南西北 — black
	redDragonColor  = lipgloss.Color("#C51A1A") // 中 — red
	greenDragonColor = lipgloss.Color("#1A7A3A") // 發 — green
	whiteDragonColor = lipgloss.Color("#7A7A7A") // 白 — grey (traditionally a blank face)

	// The tile face background.
	tileFaceColor = lipgloss.Color("#F2EFE6") // ivory white

	// Chrome / UI palette.
	dingqueColor  = lipgloss.Color("#7A7A7A")
	chromeColor   = lipgloss.Color("#9A9A9A")
	headerColor   = lipgloss.Color("#FFB000")
	selectedColor = lipgloss.Color("#FFD700")
	winColor      = lipgloss.Color("#FF77FF")
	turnColor     = lipgloss.Color("#7AE1FF")
	tableColor    = lipgloss.Color("#5A4A1F")
	dimColor      = lipgloss.Color("#4A4A4A")
)

var (
	headerStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(headerColor).
			Padding(0, 1).
			Border(lipgloss.RoundedBorder()).
			BorderForeground(headerColor)

	chromeStyle = lipgloss.NewStyle().Foreground(chromeColor)

	logStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("245")).
			Border(lipgloss.NormalBorder(), true, false, false, false).
			BorderForeground(chromeColor).
			Padding(1, 0, 0, 0)

	promptStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("#7AE1FF"))

	selectedTileStyle = lipgloss.NewStyle().
				Foreground(selectedColor).
				Bold(true).
				Underline(true)

	winBannerStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(winColor).
			Padding(0, 2)
)
