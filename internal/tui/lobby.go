package tui

import (
	"context"
	"fmt"
	"os"
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
	Mode       LobbyMode
	Rule       string // "sichuan" | "riichi"
	Seats      [4]SeatRole
	JoinAt     string        // for LobbyModeJoin: target address
	Wait       time.Duration // for LobbyModeHost: how long to wait for joiners
	HostBot    bool          // for LobbyModeHost: also play locally as seat 0 (true) or pure server (false)
	PlayerName string        // user's display name (carries through to local seats and network registration)
}

// lobbyState distinguishes the lobby's screens.
type lobbyState int

const (
	stateMain lobbyState = iota
	stateNewLocal
	stateNewHost
	stateJoin
	stateJoinManual
	stateEditName
)

// LobbyModel is the Bubble Tea model for the lobby.
type LobbyModel struct {
	state    lobbyState
	cursor   int
	width    int
	height   int
	quitting bool

	// Player name shown to other players. Defaults to $USER, editable
	// from a dedicated lobby screen.
	playerName string

	// New-Local / New-Host form state.
	rule           string // "sichuan" | "riichi"
	seats          [4]SeatRole
	formIdx        int    // focused field; layout differs per state — see handleFormKey
	waitS          int    // wait timeout for host (seconds)
	hostServerOnly bool   // host: when true, seat 0 is also remote (you join from another terminal)

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
	name := os.Getenv("USER")
	if name == "" {
		name = os.Getenv("USERNAME") // Windows
	}
	if name == "" {
		name = "Player"
	}
	return &LobbyModel{
		state:      stateMain,
		rule:       "sichuan",
		seats:      [4]SeatRole{SeatHuman, SeatBotEasy, SeatBotEasy, SeatBotEasy},
		waitS:      30,
		playerName: name,
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
	case stateEditName:
		return m.handleNameKey(msg)
	}
	return m, nil
}

// handleNameKey edits m.playerName via the same character-buffer
// approach as the manual-IP screen.
func (m *LobbyModel) handleNameKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "esc", "enter":
		m.state = stateMain
		m.cursor = 0
		return m, nil
	case "backspace":
		if len(m.playerName) > 0 {
			m.playerName = m.playerName[:len(m.playerName)-1]
		}
		return m, nil
	}
	s := msg.String()
	if len(s) == 1 {
		c := s[0]
		if c >= 0x20 && c < 0x7f && len(m.playerName) < 20 {
			m.playerName += string(c)
		}
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
		if !strings.Contains(addr, ":") {
			addr = addr + ":7777"
		}
		m.Result = LobbyResult{
			Mode:       LobbyModeJoin,
			JoinAt:     addr,
			Rule:       "sichuan",
			PlayerName: m.playerName,
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
	{"Join by IP address", stateJoinManual},
	{"Join LAN (auto-discover mDNS)", stateJoin},
	{"Edit your name", stateEditName},
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
			m.hostServerOnly = false
		case stateJoin:
			m.scanning = true
			return m, scanCmd()
		case stateJoinManual:
			m.manualAddr = ""
		case stateEditName:
			// nothing to init; just enter editing mode
		}
	case "q":
		m.quitting = true
		m.Result = LobbyResult{Mode: LobbyModeQuit}
		return m, tea.Quit
	}
	return m, nil
}

// ---------------- New game form ----------------
//
// Field index layout:
//   stateNewLocal: 0=Rule, 1-3=Seat1..3, 4=Start              (maxIdx=4)
//   stateNewHost:  0=Rule, 1=ServerOnly, 2-4=Seat1..3,
//                  5=Wait, 6=Start                            (maxIdx=6)

func (m *LobbyModel) formMaxIdx() int {
	if m.state == stateNewHost {
		return 6
	}
	return 4
}

func (m *LobbyModel) handleFormKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	maxIdx := m.formMaxIdx()
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
	if m.state == stateNewLocal {
		switch m.formIdx {
		case 0:
			m.toggleRule()
		case 1, 2, 3:
			m.cycleSeat(m.formIdx, dir, false)
		}
		return
	}
	// stateNewHost
	switch m.formIdx {
	case 0:
		m.toggleRule()
	case 1:
		m.hostServerOnly = !m.hostServerOnly
		if m.hostServerOnly {
			m.seats[0] = SeatRemote
		} else {
			m.seats[0] = SeatHuman
		}
	case 2, 3, 4:
		m.cycleSeat(m.formIdx-1, dir, true)
	case 5:
		step := 5
		if dir < 0 {
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

func (m *LobbyModel) toggleRule() {
	if m.rule == "sichuan" {
		m.rule = "riichi"
	} else {
		m.rule = "sichuan"
	}
}

// cycleSeat advances the role for seats[seatIdx]. host=true allows
// SeatRemote in the cycle.
func (m *LobbyModel) cycleSeat(seatIdx, dir int, host bool) {
	options := []SeatRole{SeatBotEasy, SeatBotMedium, SeatBotHard}
	if host {
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
}

func (m *LobbyModel) startGame() {
	switch m.state {
	case stateNewLocal:
		m.Result = LobbyResult{
			Mode:       LobbyModeLocal,
			Rule:       m.rule,
			Seats:      m.seats,
			PlayerName: m.playerName,
		}
	case stateNewHost:
		m.Result = LobbyResult{
			Mode:       LobbyModeHost,
			Rule:       m.rule,
			Seats:      m.seats,
			Wait:       time.Duration(m.waitS) * time.Second,
			HostBot:    !m.hostServerOnly,
			PlayerName: m.playerName,
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
			Mode:       LobbyModeJoin,
			JoinAt:     m.scanResult[m.cursor].Addr,
			Rule:       "sichuan",
			PlayerName: m.playerName,
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
	case stateEditName:
		body = m.viewEditName()
	}

	footer := chromeStyle.Render("ctrl+c = quit  ·  arrows/hjkl = move  ·  enter = select  ·  esc/b = back")

	nameLine := chromeStyle.Render("Your name: ") +
		lipgloss.NewStyle().Foreground(turnColor).Render(m.playerName)

	return lipgloss.JoinVertical(lipgloss.Center,
		"",
		title,
		nameLine,
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
	maxIdx := m.formMaxIdx()
	startLabel := "[ Start ]"

	field := func(idx int, label, value string) string {
		marker := "  "
		val := value
		if idx == m.formIdx {
			marker = "▸ "
			val = lipgloss.NewStyle().Foreground(turnColor).Bold(true).Render("‹ " + value + " ›")
		}
		return fmt.Sprintf("%s%-14s %s", marker, label+":", val)
	}

	var lines []string
	lines = append(lines, lipgloss.NewStyle().Bold(true).Render(title))
	lines = append(lines, "")

	if m.state == stateNewLocal {
		lines = append(lines, field(0, "Rule", m.rule))
		lines = append(lines, field(1, "Seat 1", m.seats[1].String()))
		lines = append(lines, field(2, "Seat 2", m.seats[2].String()))
		lines = append(lines, field(3, "Seat 3", m.seats[3].String()))
		lines = append(lines, "")
		lines = append(lines, chromeStyle.Render("Seat 0 is YOU. left/right cycle values."))
	} else {
		// stateNewHost
		soVal := "No (you play here)"
		if m.hostServerOnly {
			soVal = "Yes (join from another terminal)"
		}
		lines = append(lines, field(0, "Rule", m.rule))
		lines = append(lines, field(1, "Server only", soVal))
		lines = append(lines, field(2, "Seat 1", m.seats[1].String()))
		lines = append(lines, field(3, "Seat 2", m.seats[2].String()))
		lines = append(lines, field(4, "Seat 3", m.seats[3].String()))
		lines = append(lines, field(5, "Wait", fmt.Sprintf("%ds", m.waitS)))
		lines = append(lines, "")
		hint := "Server only = No  → you play seat 0 in this terminal."
		if m.hostServerOnly {
			hint = "Server only = Yes → run a separate `rontama` and Join LAN to take seat 0."
		}
		lines = append(lines, chromeStyle.Render(hint))
	}

	lines = append(lines, "")
	startMarker := "  "
	if m.formIdx == maxIdx {
		startMarker = "▸ "
		lines = append(lines, startMarker+lipgloss.NewStyle().Foreground(headerColor).Bold(true).Render(startLabel))
	} else {
		lines = append(lines, startMarker+startLabel)
	}

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

func (m *LobbyModel) viewEditName() string {
	var lines []string
	lines = append(lines, lipgloss.NewStyle().Bold(true).Render("Edit your name"))
	lines = append(lines, "")
	lines = append(lines, chromeStyle.Render("Other players see this in the table panels."))
	lines = append(lines, "")
	cursor := lipgloss.NewStyle().Foreground(turnColor).Render("█")
	lines = append(lines, "  > "+m.playerName+cursor)
	lines = append(lines, "")
	lines = append(lines, chromeStyle.Render("enter / esc = save and back  ·  backspace = delete"))
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
