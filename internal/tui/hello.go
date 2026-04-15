// Package tui hosts the Bubble Tea models and views for RonTama.
//
// hello.go is a Phase 0 placeholder: it proves the Bubble Tea + Lipgloss
// stack works end-to-end. Real game UI lives in views/ in later phases.
package tui

import (
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

var (
	titleStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("#FFB000")).
			Padding(1, 4).
			Border(lipgloss.RoundedBorder()).
			BorderForeground(lipgloss.Color("#FFB000"))

	hintStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("240")).
			Italic(true).
			MarginTop(1)
)

type helloModel struct {
	width  int
	height int
}

// NewHello returns the Phase 0 placeholder Bubble Tea program.
func NewHello() *tea.Program {
	return tea.NewProgram(helloModel{}, tea.WithAltScreen())
}

func (m helloModel) Init() tea.Cmd {
	return nil
}

func (m helloModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width, m.height = msg.Width, msg.Height
	case tea.KeyMsg:
		switch msg.String() {
		case "q", "ctrl+c", "esc":
			return m, tea.Quit
		}
	}
	return m, nil
}

func (m helloModel) View() string {
	title := titleStyle.Render("🀄  RonTama")
	hint := hintStyle.Render("Phase 0 scaffold — press q to quit")
	body := lipgloss.JoinVertical(lipgloss.Center, title, hint)

	if m.width == 0 || m.height == 0 {
		return body
	}
	return lipgloss.Place(m.width, m.height, lipgloss.Center, lipgloss.Center, body)
}

// compile-time check
var _ tea.Model = helloModel{}
