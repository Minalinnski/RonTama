package tui

import (
	"context"
	"fmt"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/Minalinnski/RonTama/internal/discovery"
)

// LobbyMode is the post-lobby outcome — how the play layer should run.
type LobbyMode int

const (
	LobbyModeQuit  LobbyMode = iota // user pressed quit
	LobbyModeLocal                  // single-process play (you + 3 bots)
	LobbyModeHost                   // open LAN server, you optionally play
	LobbyModeJoin                   // dial a LAN server
)

// SeatRole describes who occupies a seat in a Local or Host game.
type SeatRole int

const (
	SeatHuman      SeatRole = iota // local human (the operator of this terminal)
	SeatRemote                     // a remote human will join via LAN
	SeatBotEasy                    // local Easy bot
	SeatBotMedium                  // local Medium bot
	SeatBotHard                    // local Hard bot
)

func (r SeatRole) String() string {
	switch r {
	case SeatHuman:
		return "You"
	case SeatRemote:
		return "Remote"
	case SeatBotEasy:
		return "Easy bot"
	case SeatBotMedium:
		return "Medium bot"
	case SeatBotHard:
		return "Hard bot"
	}
	return "?"
}

// LobbyResult is what RunLobby returns to main; main dispatches into
// the play layer accordingly.
type LobbyResult struct {
	Mode    LobbyMode
	Rule    string // "sichuan" | "riichi"
	Seats   [4]SeatRole
	JoinAt  string        // for LobbyModeJoin: target address
	Wait    time.Duration // for LobbyModeHost: how long to wait for joiners
	HostBot bool          // for LobbyModeHost: also play locally as seat 0 (true) or pure server (false)
}

// lobbyState distinguishes the lobby's screens.
type lobbyState int

const (
	stateMain lobbyState = iota
	stateNewLocal
	stateNewHost
	stateJoin
	stateJoinManual
)

// LobbyModel is the Bubble Tea model for the lobby.
type LobbyModel struct {
	state    lobbyState
	cursor   int
	width    int
	height   int
	quitting bool

	// New-Local / New-Host form state.
	rule    string     // "sichuan" | "riichi"
	seats   [4]SeatRole
	formIdx int        // which field is focused (0=rule, 1=seat1, 2=seat2, 3=seat3, 4=wait, 5=Start)
	waitS   int        // wait timeout for host (seconds)

	// Join state.
	scanning   bool
	scanResult []discovery.Found
	scanErr    error

	// Manual-address join input buffer.
	manualAddr string

	// Output.
	Result LobbyResult
}

// NewLobbyModel returns a fresh LobbyModel with sensible defaults.
func NewLobbyModel() *LobbyModel {
	return &LobbyModel{
		state: stateMain,
		rule:  "sichuan",
		seats: [4]SeatRole{SeatHuman, SeatBotEasy, SeatBotEasy, SeatBotEasy},
		waitS: 30,
	}
}

// Init implements tea.Model.
func (m *LobbyModel) Init() tea.Cmd { return nil }

// Update implements tea.Model.
func (m *LobbyModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width, m.height = msg.Width, msg.Height
		return m, nil
	case tea.KeyMsg:
		return m.handleKey(msg)
	case scanDoneMsg:
		m.scanning = false
		m.scanResult = msg.results
		m.scanErr = msg.err
		m.cursor = 0
		return m, nil
	}
	return m, nil
}

func (m *LobbyModel) handleKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	if msg.Type == tea.KeyCtrlC {
		m.quitting = true
		m.Result = LobbyResult{Mode: LobbyModeQuit}
		return m, tea.Quit
	}
	switch m.state {
	case stateMain:
		return m.handleMainKey(msg)
	case stateNewLocal, stateNewHost:
		return m.handleFormKey(msg)
	case stateJoin:
		return m.handleJoinKey(msg)
	case stateJoinManual:
		return m.handleManualKey(msg)
	}
	return m, nil
}

// handleManualKey accepts typed characters into m.manualAddr and dials on enter.
func (m *LobbyModel) handleManualKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "esc":
		m.state = stateMain
		m.cursor = 0
		return m, nil
	case "backspace":
		if len(m.manualAddr) > 0 {
			m.manualAddr = m.manualAddr[:len(m.manualAddr)-1]
		}
		return m, nil
	case "enter":
		if m.manualAddr == "" {
			return m, nil
		}
		addr := m.manualAddr
		// If the user typed "host" without a port, default 7777.
		if !strings.Contains(addr, ":") {
			addr = addr + ":7777"
		}
		m.Result = LobbyResult{
			Mode:   LobbyModeJoin,
			JoinAt: addr,
			Rule:   "sichuan",
		}
		return m, tea.Quit
	}
	// Treat any 1-char printable key as a typed character.
	s := msg.String()
	if len(s) == 1 {
		c := s[0]
		if c >= 0x20 && c < 0x7f {
			m.manualAddr += string(c)
		}
	}
	return m, nil
}

// ---------------- Main menu ----------------

var mainOptions = []struct {
	label string
	state lobbyState
}{
	{"New Local Game (you + 3 bots)", stateNewLocal},
	{"Host LAN Game (open a room for friends)", stateNewHost},
	{"Join LAN Game (auto-discover via mDNS)", stateJoin},
	{"Join by IP address (manual)", stateJoinManual},
}

func (m *LobbyModel) handleMainKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "up", "k":
		if m.cursor > 0 {
			m.cursor--
		}
	case "down", "j":
		if m.cursor < len(mainOptions) {
			m.cursor++
		}
	case "enter", " ":
		if m.cursor == len(mainOptions) {
			m.quitting = true
			m.Result = LobbyResult{Mode: LobbyModeQuit}
			return m, tea.Quit
		}
		opt := mainOptions[m.cursor]
		m.state = opt.state
		m.cursor = 0
		m.formIdx = 0
		switch m.state {
		case stateNewLocal:
			m.seats = [4]SeatRole{SeatHuman, SeatBotEasy, SeatBotEasy, SeatBotEasy}
		case stateNewHost:
			m.seats = [4]SeatRole{SeatHuman, SeatRemote, SeatRemote, SeatRemote}
		case stateJoin:
			m.scanning = true
			return m, scanCmd()
		case stateJoinManual:
			m.manualAddr = ""
		}
	case "q":
		m.quitting = true
		m.Result = LobbyResult{Mode: LobbyModeQuit}
		return m, tea.Quit
	}
	return m, nil
}

// ---------------- New game form ----------------

func (m *LobbyModel) handleFormKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	maxIdx := 4 // 0=rule, 1..3=seats[1..3] (seat 0 is always You for local; for host: cycle through), wait field for host
	if m.state == stateNewHost {
		maxIdx++ // wait timeout
	}
	switch msg.String() {
	case "esc", "b":
		m.state = stateMain
		m.cursor = 0
		return m, nil
	case "up", "k":
		if m.formIdx > 0 {
			m.formIdx--
		}
	case "down", "j":
		if m.formIdx < maxIdx {
			m.formIdx++
		}
	case "left", "h":
		m.adjustForm(-1)
	case "right", "l":
		m.adjustForm(+1)
	case "enter", " ":
		if m.formIdx == maxIdx {
			m.startGame()
			return m, tea.Quit
		}
		m.adjustForm(+1)
	}
	return m, nil
}

func (m *LobbyModel) adjustForm(dir int) {
	switch m.formIdx {
	case 0:
		// rule toggle
		if m.rule == "sichuan" {
			m.rule = "riichi"
		} else {
			m.rule = "sichuan"
		}
	case 1, 2, 3:
		seatIdx := m.formIdx
		options := []SeatRole{SeatBotEasy, SeatBotMedium, SeatBotHard}
		if m.state == stateNewHost {
			options = []SeatRole{SeatRemote, SeatBotEasy, SeatBotMedium, SeatBotHard}
		}
		curIdx := 0
		for i, r := range options {
			if r == m.seats[seatIdx] {
				curIdx = i
				break
			}
		}
		curIdx = (curIdx + dir + len(options)) % len(options)
		m.seats[seatIdx] = options[curIdx]
	case 4:
		if m.state == stateNewHost {
			step := 5 * dir
			if dir > 0 {
				step = 5
			} else {
				step = -5
			}
			m.waitS += step
			if m.waitS < 5 {
				m.waitS = 5
			}
			if m.waitS > 600 {
				m.waitS = 600
			}
		}
	}
}

func (m *LobbyModel) startGame() {
	switch m.state {
	case stateNewLocal:
		m.Result = LobbyResult{
			Mode:  LobbyModeLocal,
			Rule:  m.rule,
			Seats: m.seats,
		}
	case stateNewHost:
		m.Result = LobbyResult{
			Mode:    LobbyModeHost,
			Rule:    m.rule,
			Seats:   m.seats,
			Wait:    time.Duration(m.waitS) * time.Second,
			HostBot: m.seats[0] == SeatHuman,
		}
	}
}

// ---------------- Join form ----------------

func (m *LobbyModel) handleJoinKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "esc", "b":
		m.state = stateMain
		m.cursor = 0
		return m, nil
	case "r":
		// rescan
		m.scanning = true
		return m, scanCmd()
	case "up", "k":
		if m.cursor > 0 {
			m.cursor--
		}
	case "down", "j":
		if m.cursor < len(m.scanResult)-1 {
			m.cursor++
		}
	case "enter", " ":
		if len(m.scanResult) == 0 {
			return m, nil
		}
		m.Result = LobbyResult{
			Mode:   LobbyModeJoin,
			JoinAt: m.scanResult[m.cursor].Addr,
			Rule:   "sichuan", // join doesn't know yet; player can pass -rule
		}
		return m, tea.Quit
	}
	return m, nil
}

type scanDoneMsg struct {
	results []discovery.Found
	err     error
}

func scanCmd() tea.Cmd {
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 4*time.Second)
		defer cancel()
		found, err := discovery.Browse(ctx, 3*time.Second)
		return scanDoneMsg{results: found, err: err}
	}
}

// ---------------- View ----------------

func (m *LobbyModel) View() string {
	if m.quitting {
		return ""
	}
	title := lipgloss.NewStyle().
		Bold(true).
		Foreground(headerColor).
		Padding(0, 2).
		Border(lipgloss.RoundedBorder()).
		BorderForeground(headerColor).
		Render("🀄 RonTama Lobby")

	var body string
	switch m.state {
	case stateMain:
		body = m.viewMain()
	case stateNewLocal:
		body = m.viewForm("New Local Game")
	case stateNewHost:
		body = m.viewForm("Host LAN Game")
	case stateJoin:
		body = m.viewJoin()
	case stateJoinManual:
		body = m.viewJoinManual()
	}

	footer := chromeStyle.Render("ctrl+c = quit  ·  arrows/hjkl = move  ·  enter = select  ·  esc/b = back")

	return lipgloss.JoinVertical(lipgloss.Center,
		"",
		title,
		"",
		body,
		"",
		footer,
	)
}

func (m *LobbyModel) viewMain() string {
	var lines []string
	for i, opt := range mainOptions {
		marker := "  "
		label := opt.label
		if i == m.cursor {
			marker = "▸ "
			label = lipgloss.NewStyle().Foreground(turnColor).Bold(true).Render(label)
		}
		lines = append(lines, marker+label)
	}
	// Quit option
	marker := "  "
	label := "Quit"
	if m.cursor == len(mainOptions) {
		marker = "▸ "
		label = lipgloss.NewStyle().Foreground(turnColor).Bold(true).Render(label)
	}
	lines = append(lines, marker+label)

	body := strings.Join(lines, "\n")
	return lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(chromeColor).
		Padding(1, 4).
		Render(body)
}

func (m *LobbyModel) viewForm(title string) string {
	maxIdx := 4
	if m.state == stateNewHost {
		maxIdx = 5
	}
	startLabel := "[ Start ]"

	field := func(idx int, label, value string) string {
		marker := "  "
		val := value
		if idx == m.formIdx {
			marker = "▸ "
			val = lipgloss.NewStyle().Foreground(turnColor).Bold(true).Render("‹ " + value + " ›")
		}
		return fmt.Sprintf("%s%-12s %s", marker, label+":", val)
	}

	var lines []string
	lines = append(lines, lipgloss.NewStyle().Bold(true).Render(title))
	lines = append(lines, "")
	lines = append(lines, field(0, "Rule", m.rule))
	lines = append(lines, field(1, "Seat 1", m.seats[1].String()))
	lines = append(lines, field(2, "Seat 2", m.seats[2].String()))
	lines = append(lines, field(3, "Seat 3", m.seats[3].String()))
	if m.state == stateNewHost {
		lines = append(lines, field(4, "Wait", fmt.Sprintf("%ds", m.waitS)))
	}
	lines = append(lines, "")
	startMarker := "  "
	if m.formIdx == maxIdx {
		startMarker = "▸ "
		lines = append(lines, startMarker+lipgloss.NewStyle().Foreground(headerColor).Bold(true).Render(startLabel))
	} else {
		lines = append(lines, startMarker+startLabel)
	}
	lines = append(lines, "")
	lines = append(lines, chromeStyle.Render("Seat 0 is YOU. left/right cycle values."))

	body := strings.Join(lines, "\n")
	return lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(chromeColor).
		Padding(1, 4).
		Render(body)
}

func (m *LobbyModel) viewJoin() string {
	var lines []string
	lines = append(lines, lipgloss.NewStyle().Bold(true).Render("Join LAN Game"))
	lines = append(lines, "")
	switch {
	case m.scanning:
		lines = append(lines, chromeStyle.Render("Scanning mDNS for RonTama servers..."))
	case m.scanErr != nil:
		lines = append(lines, lipgloss.NewStyle().Foreground(lipgloss.Color("#FF7777")).Render("scan error: "+m.scanErr.Error()))
	case len(m.scanResult) == 0:
		lines = append(lines, chromeStyle.Render("No servers found. r=rescan, esc=back"))
	default:
		for i, f := range m.scanResult {
			marker := "  "
			label := fmt.Sprintf("%-30s  %s", f.Name, f.Addr)
			if i == m.cursor {
				marker = "▸ "
				label = lipgloss.NewStyle().Foreground(turnColor).Bold(true).Render(label)
			}
			lines = append(lines, marker+label)
		}
		lines = append(lines, "")
		lines = append(lines, chromeStyle.Render("enter = connect  ·  r = rescan"))
	}
	body := strings.Join(lines, "\n")
	return lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(chromeColor).
		Padding(1, 4).
		Render(body)
}

func (m *LobbyModel) viewJoinManual() string {
	var lines []string
	lines = append(lines, lipgloss.NewStyle().Bold(true).Render("Join by IP address"))
	lines = append(lines, "")
	lines = append(lines, chromeStyle.Render("Type the host's address (e.g. 192.168.1.5 or 192.168.1.5:7777)."))
	lines = append(lines, "")
	cursor := lipgloss.NewStyle().Foreground(turnColor).Render("█")
	lines = append(lines, "  > "+m.manualAddr+cursor)
	lines = append(lines, "")
	lines = append(lines, chromeStyle.Render("enter = connect  ·  esc = back  ·  default port 7777"))
	body := strings.Join(lines, "\n")
	return lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(chromeColor).
		Padding(1, 4).
		Render(body)
}

// RunLobby launches the lobby Bubble Tea program and returns the user's choice.
func RunLobby() (LobbyResult, error) {
	m := NewLobbyModel()
	prog := tea.NewProgram(m, tea.WithAltScreen())
	if _, err := prog.Run(); err != nil {
		return LobbyResult{}, err
	}
	return m.Result, nil
}
