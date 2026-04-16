// Package tui (cont.) — the interactive play model.
package tui

import (
	"fmt"
	"strings"
	"time"

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
	Kind      string // "dingque" | "draw" | "call" | "exchange3"
	View      game.PlayerView
	Calls     []game.Call // for "call"
	Respond   chan any    // user pushes their answer here
	Discarded tile.Tile   // for "call" — the tile being acted on
	Deadline  time.Time   // zero = no deadline (local play); non-zero = display countdown
}

// tickMsg fires periodically while a Deadline-bearing prompt is open
// so the countdown re-renders.
type tickMsg time.Time

// RoundDoneMsg signals the round ended.
type RoundDoneMsg struct {
	Result *game.RoundResult
	Err    error
}

// PlayModel is the interactive play TUI's Bubble Tea model.
type PlayModel struct {
	rule      rules.RuleSet
	state     *game.State
	prompt    *HumanPromptMsg
	selected  int   // single-select index (draw prompt) — 0..n-1 + drawn slot
	exchSet   []int // multi-select indices into the sorted hand (exchange3 prompt)
	log       []string
	width     int
	height    int
	roundDone bool
	finalNote string
	quitting  bool

	// Banner: optional pre-game info shown while state is nil. The host
	// uses this to display the listen IP so they can tell friends what
	// to type into 'rontama join -addr ...'.
	Banner string
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
		// A new round just started — clear the "round done" banner so
		// subsequent rounds of a match re-enter normal play mode.
		if msg.Note == "Round start" {
			m.roundDone = false
			m.finalNote = ""
		}
		return m, nil

	case HumanPromptMsg:
		p := msg
		m.prompt = &p
		switch msg.Kind {
		case "draw":
			if msg.View.JustDrew != nil {
				sorted, _ := splitDrawn(msg.View.OwnHand, msg.View.JustDrew)
				m.selected = len(sorted) // default-select the drawn tile (rightmost)
			} else {
				m.selected = 0
			}
		case "exchange3":
			m.exchSet = nil
			m.selected = 0
		default:
			m.selected = 0
		}
		// If the prompt has a deadline, start the countdown tick loop.
		if !msg.Deadline.IsZero() {
			return m, countdownTick()
		}
		return m, nil

	case tickMsg:
		// Re-render so the countdown moves; reschedule if the prompt is
		// still open and the deadline hasn't expired.
		if m.prompt != nil && !m.prompt.Deadline.IsZero() && time.Now().Before(m.prompt.Deadline) {
			return m, countdownTick()
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
	case "exchange3":
		return m.handleExchange3Key(msg)
	case "dingque":
		return m.handleDingqueKey(msg)
	case "draw":
		return m.handleDrawKey(msg)
	case "call":
		return m.handleCallKey(msg)
	}
	return m, nil
}

// handleExchange3Key implements the multi-select picker for 换三张:
// digit/letter keys toggle inclusion; space confirms when exactly 3
// same-suit tiles are picked.
func (m PlayModel) handleExchange3Key(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	tiles := m.prompt.View.OwnHand.ConcealedTiles()
	s := msg.String()
	if len(s) == 1 {
		c := s[0]
		idx := -1
		switch {
		case c >= '1' && c <= '9':
			idx = int(c - '1')
		case c >= 'a' && c <= 'e':
			idx = 9 + int(c-'a')
		}
		if idx >= 0 && idx < len(tiles) {
			m.exchSet = toggle(m.exchSet, idx)
			return m, nil
		}
	}
	switch s {
	case "backspace", "esc":
		m.exchSet = nil
		return m, nil
	case " ", "enter":
		if len(m.exchSet) != 3 {
			return m, nil // need exactly 3
		}
		// Validate same suit.
		suit := tiles[m.exchSet[0]].Suit()
		for _, i := range m.exchSet {
			if tiles[i].Suit() != suit {
				return m, nil // ignore — UI status hint will explain
			}
		}
		var picks [3]tile.Tile
		for i, idx := range m.exchSet {
			picks[i] = tiles[idx]
		}
		m.prompt.Respond <- picks
		m.prompt = nil
		m.exchSet = nil
	}
	return m, nil
}

// toggle adds/removes idx from the slice, preserving order.
func toggle(set []int, idx int) []int {
	for i, v := range set {
		if v == idx {
			return append(set[:i], set[i+1:]...)
		}
	}
	return append(set, idx)
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
	sorted, drawn := splitDrawn(m.prompt.View.OwnHand, m.prompt.View.JustDrew)
	maxIdx := len(sorted) - 1
	if drawn != nil {
		maxIdx++
	}

	s := msg.String()
	if len(s) == 1 {
		c := s[0]
		idx := -1
		switch {
		case c >= '1' && c <= '9':
			idx = int(c - '1')
		case c >= 'a' && c <= 'e':
			idx = 9 + int(c-'a')
		}
		if idx >= 0 && idx <= maxIdx {
			m.selected = idx
			return m, nil
		}
	}

	switch s {
	case "left", "h":
		if m.selected > 0 {
			m.selected--
		}
	case "right", "l":
		if m.selected < maxIdx {
			m.selected++
		}
	case "t":
		// Only let the player tsumo when the rule actually accepts it
		// (e.g. Riichi requires a yaku). Silently swallow otherwise.
		if !canTsumoNow(m.prompt.View) {
			return m, nil
		}
		m.prompt.Respond <- game.DrawAction{Kind: game.DrawTsumo}
		m.prompt = nil
	case "r":
		// Riichi declaration: discard the currently-selected tile
		// AND set DeclareRiichi=true. The game loop validates eligibility
		// (concealed, score>=1000, wall>=4, post-discard tenpai) and
		// rejects with an error if invalid; bots/humans should self-check.
		if m.selected < 0 || m.selected > maxIdx {
			break
		}
		var discard tile.Tile
		if m.selected < len(sorted) {
			discard = sorted[m.selected]
		} else if drawn != nil {
			discard = *drawn
		}
		m.prompt.Respond <- game.DrawAction{
			Kind: game.DrawDiscard, Discard: discard, DeclareRiichi: true,
		}
		m.prompt = nil
	case " ", "enter":
		if m.selected < 0 || m.selected > maxIdx {
			break
		}
		var discard tile.Tile
		if m.selected < len(sorted) {
			discard = sorted[m.selected]
		} else if drawn != nil {
			discard = *drawn
		}
		m.prompt.Respond <- game.DrawAction{Kind: game.DrawDiscard, Discard: discard}
		m.prompt = nil
	}
	return m, nil
}

func (m PlayModel) handleCallKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	// Map a key letter → desired CallKind. We then iterate the offered
	// calls and take the first one that matches.
	wantByKey := map[string]game.CallKind{
		"r": game.CallRon,
		"k": game.CallKan,
		"p": game.CallPon,
		"c": game.CallChi,
	}
	if want, ok := wantByKey[msg.String()]; ok {
		for _, c := range m.prompt.Calls {
			if c.Kind == want {
				m.prompt.Respond <- c
				m.prompt = nil
				return m, nil
			}
		}
		// Key matched a real call kind but none was offered — ignore.
	}
	if msg.String() == "n" {
		m.prompt.Respond <- game.Pass
		m.prompt = nil
	}
	return m, nil
}

// canTsumoNow returns true when the current draw makes a winning hand
// under the rule. Used by the TUI to gate the 't' key and the prompt
// hint — we don't want to OFFER tsumo if the game would reject it
// (Riichi specifically requires a yaku, so 4-sets+1-pair without any
// yaku-bearing condition is invalid).
func canTsumoNow(view game.PlayerView) bool {
	if view.JustDrew == nil {
		return false
	}
	hand := view.OwnHand
	concealed := hand.Concealed
	concealed[*view.JustDrew]--
	probe := tile.Hand{Concealed: concealed, Melds: hand.Melds}
	ctx := rules.WinContext{
		WinningTile: *view.JustDrew,
		Tsumo:       true,
		From:        -1,
		Seat:        view.Seat,
		Dealer:      view.Dealer,
		Dingque:     view.Dingque[view.Seat],
		Riichi:      view.Riichi[view.Seat],
		RoundWind:   tile.East, // Phase 6 simplification — round wind is always East
	}
	return view.Rule.CanWin(probe, *view.JustDrew, ctx)
}

// countdownTick schedules a one-shot tea.Cmd that fires after 250ms.
// PlayModel.Update reschedules another tick while a deadline-bearing
// prompt remains open.
func countdownTick() tea.Cmd {
	return tea.Tick(250*time.Millisecond, func(t time.Time) tea.Msg {
		return tickMsg(t)
	})
}

// View implements tea.Model.
//
// Layout: 3-row × 3-column grid where each cell is a bordered panel.
//
//	┌─────┐  ┌──────────┐  ┌─────┐
//	│     │  │  N seat  │  │     │
//	└─────┘  └──────────┘  └─────┘
//	┌──────┐ ┌──────────┐  ┌──────┐
//	│ W    │ │  TABLE   │  │  E   │
//	│ seat │ │  (wall)  │  │ seat │
//	└──────┘ └──────────┘  └──────┘
//	┌─────┐  ┌──────────┐  ┌─────┐
//	│     │  │  YOU     │  │     │
//	└─────┘  └──────────┘  └─────┘
//
// Corners are blank spacers. The current-turn seat's panel border is
// highlighted in cyan.
func (m PlayModel) View() string {
	if m.state == nil {
		if m.Banner != "" {
			return lipgloss.NewStyle().
				Border(lipgloss.RoundedBorder()).
				BorderForeground(headerColor).
				Padding(1, 3).
				Render(m.Banner)
		}
		return "Loading..."
	}
	header := m.renderHeader()

	// Compute panel widths so the grid lines up.
	const sideW = 24
	const centerW = 30
	const seatPanelW = 50

	corner := lipgloss.NewStyle().Width(sideW).Render("")
	topRow := lipgloss.JoinHorizontal(lipgloss.Top,
		corner,
		lipgloss.PlaceHorizontal(centerW, lipgloss.Center, m.renderHorizSeatPanel(2, seatPanelW)),
		corner,
	)
	midRow := lipgloss.JoinHorizontal(lipgloss.Top,
		m.renderVertSeatPanel(3, sideW),
		lipgloss.PlaceHorizontal(centerW, lipgloss.Center, m.renderTablePanel(centerW-2)),
		m.renderVertSeatPanel(1, sideW),
	)
	botRow := lipgloss.JoinHorizontal(lipgloss.Top,
		corner,
		lipgloss.PlaceHorizontal(centerW, lipgloss.Center, m.renderSelfPanel(seatPanelW)),
		corner,
	)
	table := lipgloss.JoinVertical(lipgloss.Left, topRow, "", midRow, "", botRow)
	tableCentered := lipgloss.PlaceHorizontal(m.maxWidth(), lipgloss.Center, table)

	hand := m.renderHandRow()
	prompt := m.renderPrompt()
	logBlock := m.renderLog()

	body := lipgloss.JoinVertical(lipgloss.Left,
		header,
		"",
		tableCentered,
		"",
		hand,
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

// panelStyle returns the bordered style for a seat panel. If `current`
// is true, uses the cyan turn-highlight border.
func panelStyle(width int, current bool) lipgloss.Style {
	bc := chromeColor
	if current {
		bc = turnColor
	}
	return lipgloss.NewStyle().
		Width(width).
		Border(lipgloss.RoundedBorder()).
		BorderForeground(bc).
		Padding(0, 1)
}

// renderHorizSeatPanel renders the across-table seat (top of grid) as
// a wide bordered panel with horizontal river.
func (m PlayModel) renderHorizSeatPanel(seat, width int) string {
	st := m.state
	p := st.Players[seat]
	current := st.Current == seat && !p.HasWon
	titleColor := chromeColor
	if current {
		titleColor = turnColor
	}
	label := seatLabel(seat)
	if p.Name != "" {
		label = fmt.Sprintf("%s [%s]", seatLabel(seat), p.Name)
	}
	title := lipgloss.NewStyle().Bold(true).Foreground(titleColor).Render(
		fmt.Sprintf("● %s%s", label, winMark(p.HasWon)))
	if !current {
		title = lipgloss.NewStyle().Foreground(titleColor).Render(
			fmt.Sprintf("  %s%s", label, winMark(p.HasWon)))
	}
	info := chromeStyle.Render(fmt.Sprintf("Hand:%d  缺:%s  Score:%+d",
		p.Hand.ConcealedCount(), dingqueLabel(p.Dingque), p.Score))
	melds := chromeStyle.Render("Melds: ") + renderMelds(p.Hand.Melds)
	riichi := ""
	if p.Hand.Concealed[0] >= 0 && st.Riichi[seat] {
		riichi = lipgloss.NewStyle().Foreground(winColor).Render(" 立")
	}
	if riichi != "" {
		title = title + riichi
	}
	river := chromeStyle.Render("River: ") + renderRiver(st.Discards[seat], 16)
	body := lipgloss.JoinVertical(lipgloss.Left, title, info, melds, river)
	return panelStyle(width, current).Render(body)
}

// renderVertSeatPanel renders a side seat (left or right column) as a
// narrow bordered panel with vertical river.
func (m PlayModel) renderVertSeatPanel(seat, width int) string {
	st := m.state
	p := st.Players[seat]
	current := st.Current == seat && !p.HasWon
	titleColor := chromeColor
	if current {
		titleColor = turnColor
	}
	bullet := "  "
	if current {
		bullet = "● "
	}
	label := seatLabel(seat)
	if p.Name != "" {
		// Side seats have limited width; show one-line "Name · Pos" if
		// there's room, else just Name.
		label = p.Name
	}
	title := lipgloss.NewStyle().Bold(current).Foreground(titleColor).Render(
		fmt.Sprintf("%s%s%s", bullet, label, winMark(p.HasWon)))
	if st.Riichi[seat] {
		title = title + lipgloss.NewStyle().Foreground(winColor).Render(" 立")
	}
	info := chromeStyle.Render(fmt.Sprintf("Hand:%d  缺:%s\nScore:%+d",
		p.Hand.ConcealedCount(), dingqueLabel(p.Dingque), p.Score))
	melds := chromeStyle.Render("Melds: ") + renderMelds(p.Hand.Melds)
	river := renderVerticalRiver(st.Discards[seat])
	body := lipgloss.JoinVertical(lipgloss.Left, title, info, melds, "", river)
	return panelStyle(width, current).Render(body)
}

// renderTablePanel is the centre "table" panel showing wall / round /
// turn / pot information. Visually marked as the table to avoid being
// mistaken for a seat.
func (m PlayModel) renderTablePanel(width int) string {
	st := m.state
	rw := "東" // round wind (Phase 6 keeps it East)
	header := lipgloss.NewStyle().Bold(true).Foreground(headerColor).Render("≡ TABLE ≡")
	lines := []string{
		header,
		chromeStyle.Render(fmt.Sprintf("Wall:  %d", st.Wall.Remaining())),
		chromeStyle.Render(fmt.Sprintf("Round: %s", rw)),
		chromeStyle.Render(fmt.Sprintf("Turn:  %d", st.TurnsTaken)),
	}
	if st.RiichiPot > 0 {
		lines = append(lines, lipgloss.NewStyle().Foreground(winColor).Render(
			fmt.Sprintf("Pot:   %d", st.RiichiPot)))
	}
	body := lipgloss.JoinVertical(lipgloss.Center, lines...)
	return lipgloss.NewStyle().
		Width(width).
		Border(lipgloss.RoundedBorder()).
		BorderForeground(tableColor).
		Padding(0, 1).
		Render(body)
}

// renderSelfPanel renders the YOU panel (seat 0): just info + river.
// The actual hand boxes are rendered separately below the table
// (renderHandRow) so they can stretch full-width.
func (m PlayModel) renderSelfPanel(width int) string {
	st := m.state
	p := st.Players[HumanSeat]
	current := st.Current == HumanSeat && !p.HasWon
	titleColor := chromeColor
	if current {
		titleColor = turnColor
	}
	bullet := "  "
	if current {
		bullet = "● "
	}
	youName := "YOU"
	if p.Name != "" {
		youName = fmt.Sprintf("YOU [%s]", p.Name)
	}
	title := lipgloss.NewStyle().Bold(current).Foreground(titleColor).Render(
		fmt.Sprintf("%s%s (%s)%s", bullet, youName, seatLabel(HumanSeat), winMark(p.HasWon)))
	if st.Riichi[HumanSeat] {
		title = title + lipgloss.NewStyle().Foreground(winColor).Render(" 立")
	}
	info := chromeStyle.Render(fmt.Sprintf("缺:%s  Melds:%s  Score:%+d",
		dingqueLabel(p.Dingque), renderMelds(p.Hand.Melds), p.Score))
	river := chromeStyle.Render("River: ") + renderRiver(st.Discards[HumanSeat], 16)
	body := lipgloss.JoinVertical(lipgloss.Left, title, info, river)
	return panelStyle(width, current).Render(body)
}

// renderHandRow renders the human's concealed hand as boxed tiles
// below the table. The just-drawn tile is shown on the FAR RIGHT,
// separated by a gap, and never auto-sorted into the rest.
//
// Exchange-three mode: hand is rendered with multi-select highlighting
// of m.exchSet entries; the drawn-tile slot is suppressed (no draws
// happen during the exchange phase).
func (m PlayModel) renderHandRow() string {
	st := m.state
	p := st.Players[HumanSeat]

	if m.prompt != nil && m.prompt.Kind == "exchange3" {
		tiles := m.prompt.View.OwnHand.ConcealedTiles()
		hand := renderHandMulti(tiles, m.exchSet)
		return lipgloss.PlaceHorizontal(m.maxWidth(), lipgloss.Center, hand)
	}

	sorted, drawn := splitDrawn(p.Hand, p.JustDrew)
	sel := -1
	if m.prompt != nil && m.prompt.Kind == "draw" {
		sel = m.selected
	}
	hand := renderHandSplit(sorted, drawn, sel)
	return lipgloss.PlaceHorizontal(m.maxWidth(), lipgloss.Center, hand)
}

// splitDrawn partitions the hand into (sorted-without-drawn, drawn-tile).
// If JustDrew is nil, returns (sorted-full-hand, nil).
func splitDrawn(hand tile.Hand, drew *tile.Tile) (sorted []tile.Tile, drawn *tile.Tile) {
	all := hand.ConcealedTiles()
	if drew == nil {
		return all, nil
	}
	for i := len(all) - 1; i >= 0; i-- {
		if all[i] == *drew {
			sorted = append([]tile.Tile{}, all[:i]...)
			sorted = append(sorted, all[i+1:]...)
			d := *drew
			return sorted, &d
		}
	}
	return all, nil
}

// renderVerticalRiver lays out a discard pile as one or more stacked
// columns. The "River:" label only appears on the first column; later
// columns are unlabeled to avoid the "River:River:" doubling that
// happens when wrap kicks in.
func renderVerticalRiver(tiles []tile.Tile) string {
	if len(tiles) == 0 {
		return chromeStyle.Render("River: -")
	}
	const colHeight = 10
	cols := (len(tiles) + colHeight - 1) / colHeight
	colStrs := make([]string, cols)
	for c := 0; c < cols; c++ {
		start := c * colHeight
		end := start + colHeight
		if end > len(tiles) {
			end = len(tiles)
		}
		var rows []string
		if c == 0 {
			rows = append(rows, chromeStyle.Render("River:"))
		} else {
			rows = append(rows, lipgloss.NewStyle().Foreground(dimColor).Render("  ··"))
		}
		for _, t := range tiles[start:end] {
			rows = append(rows, renderTileCompact(t))
		}
		colStrs[c] = lipgloss.JoinVertical(lipgloss.Left, rows...)
	}
	// Add a single-space separator between columns.
	parts := []string{colStrs[0]}
	for i := 1; i < len(colStrs); i++ {
		parts = append(parts, " ", colStrs[i])
	}
	return lipgloss.JoinHorizontal(lipgloss.Top, parts...)
}

// renderCenterPanel shows wall-info / dora-indicator / round-wind in
// the middle of the table.
func (m PlayModel) renderCenterPanel() string {
	st := m.state
	rw := "東"
	lines := []string{
		chromeStyle.Render(fmt.Sprintf("Wall: %d", st.Wall.Remaining())),
		chromeStyle.Render(fmt.Sprintf("Round: %s", rw)),
		chromeStyle.Render(fmt.Sprintf("Turn: %d", st.TurnsTaken)),
	}
	if st.RiichiPot > 0 {
		lines = append(lines, chromeStyle.Render(fmt.Sprintf("Pot: %d", st.RiichiPot)))
	}
	return lipgloss.JoinVertical(lipgloss.Center, lines...)
}

// dingqueLabel renders the chosen dingque suit ("?" if not yet picked).
func dingqueLabel(s tile.Suit) string {
	if s == tile.SuitWind || s == tile.SuitDragon {
		return "?"
	}
	return renderSuit(s)
}

func winMark(hasWon bool) string {
	if hasWon {
		return " ✓"
	}
	return ""
}

func (m PlayModel) renderSelfBlock() string {
	st := m.state
	p := st.Players[HumanSeat]
	tiles := p.Hand.ConcealedTiles()

	header := chromeStyle.Render(fmt.Sprintf("You (%s)   缺:%s   Melds: %s   Score: %+d",
		seatLabel(HumanSeat), dingqueLabel(p.Dingque), renderMelds(p.Hand.Melds), p.Score))
	myRiver := chromeStyle.Render("Your river: ") + renderRiver(m.state.Discards[HumanSeat], 18)

	// Find the drawn tile's position in the sorted tiles list so we
	// can render it visually separated. -1 = no draw to highlight.
	drawnIdx := -1
	if p.JustDrew != nil {
		jd := *p.JustDrew
		// The hand was sorted; find the LAST occurrence of jd to point at
		// the freshly-drawn copy (in case the player already had copies).
		for i := len(tiles) - 1; i >= 0; i-- {
			if tiles[i] == jd {
				drawnIdx = i
				break
			}
		}
	}

	sel := -1
	if m.prompt != nil && m.prompt.Kind == "draw" {
		sel = m.selected
	}
	hand := renderHandWithKeyHints(tiles, drawnIdx, sel)
	return lipgloss.JoinVertical(lipgloss.Left, header, myRiver, hand)
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
		return chromeStyle.Render("(waiting…  press q to quit)")
	}
	countdown := m.renderCountdown()
	switch m.prompt.Kind {
	case "exchange3":
		tiles := m.prompt.View.OwnHand.ConcealedTiles()
		picked := len(m.exchSet)
		hint := ""
		if picked == 3 {
			suit := tiles[m.exchSet[0]].Suit()
			ok := true
			for _, i := range m.exchSet {
				if tiles[i].Suit() != suit {
					ok = false
					break
				}
			}
			if ok {
				hint = "  ✓ press space to confirm"
			} else {
				hint = "  ✗ all 3 must be the same suit"
			}
		}
		return promptStyle.Render(fmt.Sprintf(
			"换三张: pick 3 tiles of ONE suit (1-9/a-e to toggle, esc to clear)  selected: %d/3%s%s",
			picked, hint, countdown))
	case "dingque":
		return promptStyle.Render("Choose 缺 suit:  m=萬  p=筒  s=索" + countdown)
	case "draw":
		drewLabel := "no draw (post-call discard)"
		if m.prompt.View.JustDrew != nil {
			drewLabel = "drew " + m.prompt.View.JustDrew.String()
		}
		tsumoHint := ""
		if canTsumoNow(m.prompt.View) {
			tsumoHint = "  t=自摸"
		}
		// Show the riichi hint only for Riichi rules (not Sichuan).
		riichiHint := ""
		if !m.prompt.View.Rule.RequiresDingque() {
			riichiHint = "  r=立直"
		}
		return promptStyle.Render(fmt.Sprintf(
			"Your turn — %s.  ←/→ or 1-9/a-e select  space=discard%s%s  q=quit%s",
			drewLabel, tsumoHint, riichiHint, countdown,
		))
	case "call":
		// Build a deduped option list (multiple chi patterns appear as one
		// 'c=Chi' choice — first matching pattern wins on press).
		seen := map[game.CallKind]bool{}
		var ron, kan, pon, chi bool
		for _, c := range m.prompt.Calls {
			if seen[c.Kind] {
				continue
			}
			seen[c.Kind] = true
			switch c.Kind {
			case game.CallRon:
				ron = true
			case game.CallKan:
				kan = true
			case game.CallPon:
				pon = true
			case game.CallChi:
				chi = true
			}
		}
		opts := []string{}
		if ron {
			opts = append(opts, "r=Ron")
		}
		if kan {
			opts = append(opts, "k=Kan")
		}
		if pon {
			opts = append(opts, "p=Pon")
		}
		if chi {
			opts = append(opts, "c=Chi")
		}
		opts = append(opts, "n=pass")
		return promptStyle.Render(fmt.Sprintf("Call on %s? %s%s",
			m.prompt.Discarded, strings.Join(opts, " "), countdown))
	}
	return ""
}

// renderCountdown returns "  ⏳ Ns" when the prompt has an active deadline.
func (m PlayModel) renderCountdown() string {
	if m.prompt == nil || m.prompt.Deadline.IsZero() {
		return ""
	}
	left := time.Until(m.prompt.Deadline)
	if left < 0 {
		left = 0
	}
	color := turnColor
	if left < 5*time.Second {
		color = winColor // pink/red as time runs out
	}
	return lipgloss.NewStyle().Foreground(color).Bold(true).Render(
		fmt.Sprintf("  ⏳ %ds", int(left.Seconds()+0.5)))
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
