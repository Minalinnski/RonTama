package tui

import "github.com/charmbracelet/lipgloss"

// Color palette: suits get distinct hues, neutral grays for chrome.
var (
	manColor     = lipgloss.Color("#E45757") // 萬 - red
	pinColor     = lipgloss.Color("#3A8FE6") // 筒 - blue
	souColor     = lipgloss.Color("#3FB76C") // 索 - green
	honorColor   = lipgloss.Color("#E0E0E0") // 字 - white
	dingqueColor = lipgloss.Color("#7A7A7A") // dimmed for renounced suit

	chromeColor   = lipgloss.Color("#9A9A9A")
	headerColor   = lipgloss.Color("#FFB000")
	selectedColor = lipgloss.Color("#FFD700")
	winColor      = lipgloss.Color("#FF77FF")
	turnColor     = lipgloss.Color("#7AE1FF") // current-player highlight
	tableColor    = lipgloss.Color("#5A4A1F") // center "table" panel border
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
