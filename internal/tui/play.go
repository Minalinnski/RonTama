// Package tui (cont.) — the interactive play model.
package tui

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/Minalinnski/RonTama/internal/game"
	"github.com/Minalinnski/RonTama/internal/rules"
	"github.com/Minalinnski/RonTama/internal/tile"
)

// HumanSeat is fixed at seat 0 (South). The TUI human always plays
// from the bottom of the table layout.
const HumanSeat = 0

// EventMsg notifies the play model of a state change pushed by the
// game goroutine via Observer hooks.
type EventMsg struct {
	State *game.State
	Note  string // human-readable log line
}

// HumanPromptMsg asks the user to make a decision (discard / call / dingque).
type HumanPromptMsg struct {
	Kind     string      // "dingque" | "draw" | "call"
	View     game.PlayerView
	Calls    []game.Call // for "call"
	Respond  chan any    // user pushes their answer here
	Discarded tile.Tile  // for "call" — the tile being acted on
}

// RoundDoneMsg signals the round ended.
type RoundDoneMsg struct {
	Result *game.RoundResult
	Err    error
}

// PlayModel is the interactive play TUI's Bubble Tea model.
type PlayModel struct {
	rule       rules.RuleSet
	state      *game.State
	prompt     *HumanPromptMsg
	selected   int // index in own concealed hand (or 0..n-1 + drawn)
	log        []string
	width      int
	height     int
	roundDone  bool
	finalNote  string
	quitting   bool
}

// NewPlayModel constructs a fresh PlayModel.
func NewPlayModel(rule rules.RuleSet) PlayModel {
	return PlayModel{rule: rule, log: []string{"Connecting to game..."}}
}

// Init implements tea.Model.
func (m PlayModel) Init() tea.Cmd { return nil }

// Update implements tea.Model.
func (m PlayModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width, m.height = msg.Width, msg.Height
		return m, nil

	case tea.KeyMsg:
		return m.handleKey(msg)

	case EventMsg:
		if msg.State != nil {
			m.state = msg.State
		}
		if msg.Note != "" {
			m.log = appendLog(m.log, msg.Note, 12)
		}
		return m, nil

	case HumanPromptMsg:
		p := msg
		m.prompt = &p
		// Reset selected index to point at last drawn tile if any.
		if msg.Kind == "draw" && msg.View.JustDrew != nil {
			tiles := msg.View.OwnHand.ConcealedTiles()
			m.selected = len(tiles) - 1
			// If JustDrew is at the end (it's added then sorted), put cursor on it.
		} else {
			m.selected = 0
		}
		return m, nil

	case RoundDoneMsg:
		m.roundDone = true
		if msg.Err != nil {
			m.finalNote = "ERROR: " + msg.Err.Error()
		} else {
			m.finalNote = formatRoundEnd(msg.Result)
		}
		return m, nil
	}
	return m, nil
}

func (m PlayModel) handleKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	if msg.String() == "q" || msg.Type == tea.KeyCtrlC || msg.Type == tea.KeyEsc {
		m.quitting = true
		return m, tea.Quit
	}
	if m.prompt == nil {
		return m, nil
	}
	switch m.prompt.Kind {
	case "dingque":
		return m.handleDingqueKey(msg)
	case "draw":
		return m.handleDrawKey(msg)
	case "call":
		return m.handleCallKey(msg)
	}
	return m, nil
}

func (m PlayModel) handleDingqueKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	var picked tile.Suit = 255
	switch msg.String() {
	case "m", "M":
		picked = tile.SuitMan
	case "p", "P":
		picked = tile.SuitPin
	case "s", "S":
		picked = tile.SuitSou
	}
	if picked != 255 {
		m.prompt.Respond <- picked
		m.prompt = nil
	}
	return m, nil
}

func (m PlayModel) handleDrawKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	tiles := m.prompt.View.OwnHand.ConcealedTiles()
	maxIdx := len(tiles) - 1

	switch msg.String() {
	case "left", "h":
		if m.selected > 0 {
			m.selected--
		}
	case "right", "l":
		if m.selected < maxIdx {
			m.selected++
		}
	case "t":
		// Tsumo
		m.prompt.Respond <- game.DrawAction{Kind: game.DrawTsumo}
		m.prompt = nil
	case " ", "enter":
		if m.selected >= 0 && m.selected <= maxIdx {
			discard := tiles[m.selected]
			m.prompt.Respond <- game.DrawAction{Kind: game.DrawDiscard, Discard: discard}
			m.prompt = nil
		}
	}
	return m, nil
}

func (m PlayModel) handleCallKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	for _, c := range m.prompt.Calls {
		switch msg.String() {
		case "r":
			if c.Kind == game.CallRon {
				m.prompt.Respond <- c
				m.prompt = nil
				return m, nil
			}
		case "p":
			if c.Kind == game.CallPon {
				m.prompt.Respond <- c
				m.prompt = nil
				return m, nil
			}
		case "k":
			if c.Kind == game.CallKan {
				m.prompt.Respond <- c
				m.prompt = nil
				return m, nil
			}
		}
	}
	if msg.String() == "n" {
		m.prompt.Respond <- game.Pass
		m.prompt = nil
	}
	return m, nil
}

// View implements tea.Model.
func (m PlayModel) View() string {
	if m.state == nil {
		return "Loading..."
	}
	header := m.renderHeader()
	tableTop := m.renderSeatHeader(2) // North
	tableMid := lipgloss.JoinHorizontal(lipgloss.Top,
		m.renderSeatBlock(1, "left"), // West
		strings.Repeat(" ", 4),
		m.renderCenterRivers(),
		strings.Repeat(" ", 4),
		m.renderSeatBlock(3, "right"), // East
	)
	tableBot := m.renderSelfBlock()
	logBlock := m.renderLog()
	prompt := m.renderPrompt()

	body := lipgloss.JoinVertical(lipgloss.Left,
		header,
		"",
		lipgloss.PlaceHorizontal(m.maxWidth(), lipgloss.Center, tableTop),
		"",
		tableMid,
		"",
		tableBot,
		"",
		prompt,
		"",
		logBlock,
	)
	if m.roundDone {
		body = lipgloss.JoinVertical(lipgloss.Left,
			body,
			"",
			winBannerStyle.Render(m.finalNote),
			chromeStyle.Render("press q to quit"),
		)
	}
	return body
}

func (m PlayModel) maxWidth() int {
	if m.width > 0 {
		return m.width
	}
	return 100
}

func (m PlayModel) renderHeader() string {
	st := m.state
	parts := []string{
		fmt.Sprintf("Dealer:%s", seatLabel(st.Dealer)),
		fmt.Sprintf("Wall:%d", st.Wall.Remaining()),
		fmt.Sprintf("Turn:%d", st.TurnsTaken),
	}
	scores := make([]string, 0, 4)
	for i := 0; i < game.NumPlayers; i++ {
		mark := ""
		if st.Players[i].HasWon {
			mark = "✓"
		}
		scores = append(scores, fmt.Sprintf("%s%s:%+d", seatLabel(i)[:1], mark, st.Players[i].Score))
	}
	return headerStyle.Render(strings.Join(parts, " | ") + "  Scores: " + strings.Join(scores, " "))
}

// renderSeatHeader summarises a remote seat in 1-2 lines (used for North).
func (m PlayModel) renderSeatHeader(seat int) string {
	st := m.state
	p := st.Players[seat]
	dq := renderSuit(p.Dingque)
	if p.Dingque == tile.SuitWind {
		dq = "?"
	}
	mark := ""
	if p.HasWon {
		mark = " ✓"
	}
	hand := fmt.Sprintf("Hand: %d", p.Hand.ConcealedCount())
	return chromeStyle.Render(fmt.Sprintf("%s%s   %s   缺%s   %s",
		seatLabel(seat), mark, hand, dq, renderMelds(p.Hand.Melds)))
}

// renderSeatBlock summarises a side seat (West/East) vertically.
func (m PlayModel) renderSeatBlock(seat int, _ string) string {
	st := m.state
	p := st.Players[seat]
	dq := renderSuit(p.Dingque)
	if p.Dingque == tile.SuitWind {
		dq = "?"
	}
	mark := ""
	if p.HasWon {
		mark = " ✓"
	}
	return chromeStyle.Render(fmt.Sprintf("%s%s\nHand: %d\n缺: %s\nMelds: %s",
		seatLabel(seat), mark, p.Hand.ConcealedCount(), dq, renderMelds(p.Hand.Melds)))
}

// renderCenterRivers shows all four rivers stacked.
func (m PlayModel) renderCenterRivers() string {
	st := m.state
	var lines []string
	for i := 0; i < game.NumPlayers; i++ {
		river := renderRiver(st.Discards[i], 14)
		lines = append(lines, fmt.Sprintf("%s ► %s", seatLabel(i)[:1], river))
	}
	return strings.Join(lines, "\n")
}

func (m PlayModel) renderSelfBlock() string {
	st := m.state
	p := st.Players[HumanSeat]
	tiles := p.Hand.ConcealedTiles()

	dq := renderSuit(p.Dingque)
	if p.Dingque == tile.SuitWind {
		dq = "?"
	}
	header := chromeStyle.Render(fmt.Sprintf("You (%s)  缺%s   Melds: %s   Score: %+d",
		seatLabel(HumanSeat), dq, renderMelds(p.Hand.Melds), p.Score))

	var hand string
	if m.prompt != nil && m.prompt.Kind == "draw" {
		hand = renderHandConcealed(tiles, nil, m.selected)
	} else {
		hand = renderHandConcealed(tiles, nil, -1)
	}
	return lipgloss.JoinVertical(lipgloss.Left, header, hand)
}

func (m PlayModel) renderLog() string {
	if len(m.log) == 0 {
		return ""
	}
	tail := m.log
	if len(tail) > 8 {
		tail = tail[len(tail)-8:]
	}
	return logStyle.Render(strings.Join(tail, "\n"))
}

func (m PlayModel) renderPrompt() string {
	if m.prompt == nil {
		if m.roundDone {
			return ""
		}
		return chromeStyle.Render("(bots playing… press q to quit)")
	}
	switch m.prompt.Kind {
	case "dingque":
		return promptStyle.Render("Choose 缺 suit:  m=萬  p=筒  s=索")
	case "draw":
		return promptStyle.Render(fmt.Sprintf(
			"Your turn — drew %s.  ←/→ select  space/enter discard  t=tsumo  q=quit",
			func() string {
				if m.prompt.View.JustDrew != nil {
					return m.prompt.View.JustDrew.String()
				}
				return "?"
			}(),
		))
	case "call":
		opts := []string{"n=pass"}
		for _, c := range m.prompt.Calls {
			switch c.Kind {
			case game.CallRon:
				opts = append([]string{"r=Ron"}, opts...)
			case game.CallPon:
				opts = append(opts, "p=Pon")
			case game.CallKan:
				opts = append(opts, "k=Kan")
			}
		}
		return promptStyle.Render(fmt.Sprintf("Call on %s? %s", m.prompt.Discarded, strings.Join(opts, " ")))
	}
	return ""
}

func appendLog(log []string, line string, max int) []string {
	log = append(log, line)
	if len(log) > max {
		log = log[len(log)-max:]
	}
	return log
}

func formatRoundEnd(r *game.RoundResult) string {
	if r == nil {
		return "Round ended."
	}
	var sb strings.Builder
	sb.WriteString("Round end. ")
	if len(r.Wins) == 0 {
		sb.WriteString("Wall exhausted, no wins.")
	} else {
		for i, w := range r.Wins {
			if i > 0 {
				sb.WriteString("  ")
			}
			how := "tsumo"
			if !w.Tsumo {
				how = fmt.Sprintf("ron from %s", seatLabel(w.From))
			}
			sb.WriteString(fmt.Sprintf("%s wins via %s [%s]",
				seatLabel(w.Seat), how, strings.Join(w.Score.Patterns, "+")))
		}
	}
	sb.WriteString(fmt.Sprintf("  Final: %+d/%+d/%+d/%+d",
		r.FinalScores[0], r.FinalScores[1], r.FinalScores[2], r.FinalScores[3]))
	return sb.String()
}
