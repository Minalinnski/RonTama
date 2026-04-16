package main

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"net"
	"os"
	"os/signal"
	"strings"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/Minalinnski/RonTama/internal/ai/easy"
	"github.com/Minalinnski/RonTama/internal/ai/hard"
	"github.com/Minalinnski/RonTama/internal/ai/medium"
	"github.com/Minalinnski/RonTama/internal/discovery"
	"github.com/Minalinnski/RonTama/internal/game"
	"github.com/Minalinnski/RonTama/internal/match"
	"github.com/Minalinnski/RonTama/internal/net/client"
	"github.com/Minalinnski/RonTama/internal/net/server"
	"github.com/Minalinnski/RonTama/internal/rules"
	"github.com/Minalinnski/RonTama/internal/tui"
)

// matchConfigFor returns sensible default match parameters per rule.
// Sichuan blood-battle is traditionally single-round; Riichi is
// 東風戦 (4 rounds with dealer continuation on dealer wins).
func matchConfigFor(rule rules.RuleSet) (maxRounds int, renchan bool) {
	if rule.RequiresDingque() {
		return 1, false // sichuan
	}
	return 4, true // riichi 東風戦
}

// runLobbyFlow opens the lobby TUI, then dispatches to the chosen mode.
// If joining fails (connection refused, timeout, etc.), it loops back
// to the lobby so the user can retry or pick a different option.
func runLobbyFlow() error {
	for {
		res, err := tui.RunLobby()
		if err != nil {
			return err
		}
		switch res.Mode {
		case tui.LobbyModeQuit:
			return nil // only real exit
		case tui.LobbyModeLocal:
			_ = runLobbyLocal(res)
		case tui.LobbyModeHost:
			_ = runLobbyHost(res)
		case tui.LobbyModeJoin:
			_ = runLobbyJoin(res)
		}
		// Game/room ended (quit, disconnect, round over) → back to lobby.
	}
}

// runLobbyLocal: single-process play, you at seat 0 + bots from lobby config.
func runLobbyLocal(res tui.LobbyResult) error {
	rule, err := pickRule(res.Rule)
	if err != nil {
		return err
	}
	model := tui.NewPlayModel(rule)
	// Set up a room with all seats filled (local play starts immediately).
	model.Room = buildLocalRoom(res)
	prog := tea.NewProgram(model, tea.WithAltScreen())

	go func() {
		players := buildLocalPlayersNamed(res, prog, res.PlayerName)
		obs := tui.NewTUIObserver(prog)
		log := slog.New(slog.NewTextHandler(io.Discard, nil))
		maxRounds, renchan := matchConfigFor(rule)
		_, err := match.RunMatch(match.Config{
			Rule:          rule,
			Players:       players,
			InitialDealer: 0,
			MaxRounds:     maxRounds,
			Renchan:       renchan,
		}, log, obs)
		if err != nil {
			prog.Send(tui.RoundDoneMsg{Err: err})
		}
	}()

	if _, err := prog.Run(); err != nil {
		return fmt.Errorf("tui: %w", err)
	}
	return nil
}

// runLobbyHost: open a LAN server using the lobby seat plan.
func runLobbyHost(res tui.LobbyResult) error {
	rule, err := pickRule(res.Rule)
	if err != nil {
		return err
	}
	// All server logs go to Discard — no stderr leaks behind TUI.
	log := slog.New(slog.NewTextHandler(io.Discard, nil))

	// 1. Bind listener FIRST (OS picks port). This triggers macOS
	//    firewall dialog on the normal terminal before alt-screen.
	ln, port, err := listenFreePort()
	if err != nil {
		return fmt.Errorf("listen: %w", err)
	}
	defer ln.Close()

	// 2. mDNS announce with the ACTUAL port.
	if closer, dErr := discovery.Announce("", port, []string{"rule=" + res.Rule}); dErr == nil {
		defer closer.Close()
	}

	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt)
	defer cancel()

	if !res.HostBot {
		// Pure server (no TUI).
		players := buildServerSeatsNamed(res, nil, res.PlayerName)
		cfg := server.Config{
			Addr:        fmt.Sprintf(":%d", port),
			JoinTimeout: res.Wait,
			Log:         log,
			Rule:        rule,
			Players:     players,
			Listener:    ln,
		}
		return server.Run(ctx, cfg)
	}

	// Host + play: TUI with pre-bound listener.
	startCh := make(chan struct{}, 1)
	model := tui.NewPlayModel(rule)
	model.StartChan = startCh
	room := buildHostRoom(res)
	room.Port = port
	model.Room = room
	prog := tea.NewProgram(model, tea.WithAltScreen())
	players := buildServerSeatsNamed(res, prog, res.PlayerName)
	obs := tui.NewTUIObserver(prog)

	joinCh := make(chan server.JoinEvent, 16)
	go func() {
		for ev := range joinCh {
			prog.Send(tui.JoinUpdateMsg{
				Seats:      ev.Seats,
				Filled:     ev.Filled,
				Total:      ev.Total,
				Done:       ev.Done,
				ListenAddr: ev.ListenAddr,
			})
		}
	}()

	go func() {
		cfg := server.Config{
			Addr:          ln.Addr().String(),
			JoinTimeout:   res.Wait,
			Log:           log,
			Rule:          rule,
			Players:       players,
			ExtraObserver: obs,
			JoinChan:      joinCh,
			StartChan:     startCh,
			Listener:      ln, // pre-bound listener — no race!
		}
		if err := server.Run(ctx, cfg); err != nil {
			prog.Send(tui.RoundDoneMsg{Err: err})
		}
	}()

	if _, err := prog.Run(); err != nil {
		return fmt.Errorf("tui: %w", err)
	}
	return nil
}

// runLobbyJoin: dial selected server, launch TUI client.
func runLobbyJoin(res tui.LobbyResult) error {
	rule, err := pickRule(res.Rule)
	if err != nil {
		return err
	}
	log := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelInfo}))
	return joinAsTUINamed(res.JoinAt, rule, res.PlayerName, log)
}

// buildLocalPlayersNamed turns lobby seat config into a [4]game.Player
// for local play, with the human at seat 0 carrying the lobby-supplied name.
func buildLocalPlayersNamed(res tui.LobbyResult, prog *tea.Program, humanName string) [game.NumPlayers]game.Player {
	var players [game.NumPlayers]game.Player
	for i, role := range res.Seats {
		players[i] = playerFromRoleNamed(role, i, prog, humanName)
		if players[i] == nil {
			players[i] = easy.New(fmt.Sprintf("seat%d-fallback", i))
		}
	}
	return players
}

// buildServerSeatsNamed turns lobby seat config into the Players[4] for
// server.Config (nil entries become remote slots).
func buildServerSeatsNamed(res tui.LobbyResult, prog *tea.Program, humanName string) [game.NumPlayers]game.Player {
	var players [game.NumPlayers]game.Player
	for i, role := range res.Seats {
		switch role {
		case tui.SeatRemote:
			players[i] = nil
		case tui.SeatHuman:
			if prog != nil {
				players[i] = tui.NewHumanPlayer(humanName, prog)
			}
		default:
			players[i] = playerFromRoleNamed(role, i, prog, humanName)
		}
	}
	return players
}

// playerFromRoleNamed maps a single lobby SeatRole to a concrete game.Player.
func playerFromRoleNamed(role tui.SeatRole, seat int, prog *tea.Program, humanName string) game.Player {
	switch role {
	case tui.SeatHuman:
		if prog != nil {
			return tui.NewHumanPlayer(humanName, prog)
		}
		return easy.New(humanName + "-fallback")
	case tui.SeatBotEasy:
		return easy.New(fmt.Sprintf("Easy bot %d", seat))
	case tui.SeatBotMedium:
		return medium.New(fmt.Sprintf("Medium bot %d", seat))
	case tui.SeatBotHard:
		return hard.New(fmt.Sprintf("Hard bot %d", seat))
	}
	return nil
}

// listenFreePort binds on :0 (OS picks a free port). Always succeeds
// unless the system is truly out of ports. Returns the listener and
// the actual assigned port.
func listenFreePort() (net.Listener, int, error) {
	ln, err := net.Listen("tcp", ":0")
	if err != nil {
		return nil, 0, err
	}
	port := ln.Addr().(*net.TCPAddr).Port
	return ln, port, nil
}

// buildLocalRoom creates the initial room for local play (all seats filled).
func buildLocalRoom(res tui.LobbyResult) *tui.RoomState {
	room := &tui.RoomState{Rule: res.Rule}
	for i, role := range res.Seats {
		switch role {
		case tui.SeatHuman:
			room.Seats[i] = tui.SeatInfo{Name: res.PlayerName, Status: tui.SeatStatusYou}
		case tui.SeatBotEasy:
			room.Seats[i] = tui.SeatInfo{Name: "Easy bot", Status: tui.SeatStatusBot}
		case tui.SeatBotMedium:
			room.Seats[i] = tui.SeatInfo{Name: "Medium bot", Status: tui.SeatStatusBot}
		case tui.SeatBotHard:
			room.Seats[i] = tui.SeatInfo{Name: "Hard bot", Status: tui.SeatStatusBot}
		}
		room.Filled++
	}
	room.Total = 4
	return room
}

// buildHostRoom creates the initial room for hosting (some seats waiting).
func buildHostRoom(res tui.LobbyResult) *tui.RoomState {
	room := &tui.RoomState{
		Rule:     res.Rule,
		CanStart: true,
		ShowIPs:  true,
	}
	remoteCount := 0
	for i, role := range res.Seats {
		switch role {
		case tui.SeatHuman:
			room.Seats[i] = tui.SeatInfo{Name: res.PlayerName, Status: tui.SeatStatusYou}
		case tui.SeatRemote:
			room.Seats[i] = tui.SeatInfo{Status: tui.SeatStatusWaiting}
			remoteCount++
		case tui.SeatBotEasy:
			room.Seats[i] = tui.SeatInfo{Name: "Easy bot", Status: tui.SeatStatusBot}
		case tui.SeatBotMedium:
			room.Seats[i] = tui.SeatInfo{Name: "Medium bot", Status: tui.SeatStatusBot}
		case tui.SeatBotHard:
			room.Seats[i] = tui.SeatInfo{Name: "Hard bot", Status: tui.SeatStatusBot}
		}
	}
	room.Total = remoteCount
	return room
}

// formatHostBanner builds the "waiting for friends" banner shown in the
// host's TUI before joiners arrive. It includes the listen IPs so the
// host can read them off to friends ("type 192.168.x.x:7777").
func formatHostBanner(res tui.LobbyResult, port int) string {
	addrs := discovery.LocalIPv4Addrs()
	var b strings.Builder
	b.WriteString("🀄  HOSTING — waiting for friends to join\n\n")
	b.WriteString(fmt.Sprintf("Rule:    %s\n", res.Rule))
	roles := make([]string, 4)
	for i, r := range res.Seats {
		roles[i] = fmt.Sprintf("seat%d=%s", i, r.String())
	}
	b.WriteString(fmt.Sprintf("Seats:   %s\n", strings.Join(roles, ", ")))
	b.WriteString(fmt.Sprintf("Wait:    %s before any unfilled remote seats become bots\n\n", res.Wait))
	b.WriteString("Tell friends:\n")
	b.WriteString("  1. They run `rontama` → Join LAN Game (mDNS auto-discover), OR\n")
	if len(addrs) > 0 {
		b.WriteString("  2. They run `rontama` → Join by IP address → type one of:\n")
		for _, a := range addrs {
			b.WriteString(fmt.Sprintf("       %s:%d\n", a, port))
		}
	} else {
		b.WriteString("  2. (no LAN IPs detected — only loopback joining will work)\n")
	}
	b.WriteString("\nThis screen will replace itself with the table once everyone is in.\n")
	b.WriteString("Press q to abort.")
	return b.String()
}

// _ = client kept for potential future direct use of the client package
// in this file (e.g. reusing ParseRule or shared join helpers).
var _ = client.NewHeadlessDecider
var _ rules.RuleSet
