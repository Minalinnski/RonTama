package main

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"os"
	"os/signal"
	"strings"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/Minalinnski/RonTama/internal/ai/easy"
	"github.com/Minalinnski/RonTama/internal/ai/hard"
	"github.com/Minalinnski/RonTama/internal/ai/medium"
	"github.com/Minalinnski/RonTama/internal/discovery"
	"github.com/Minalinnski/RonTama/internal/game"
	"github.com/Minalinnski/RonTama/internal/net/client"
	"github.com/Minalinnski/RonTama/internal/net/server"
	"github.com/Minalinnski/RonTama/internal/rules"
	"github.com/Minalinnski/RonTama/internal/tui"
)

// runLobbyFlow opens the lobby TUI, then dispatches to the chosen mode.
func runLobbyFlow() error {
	res, err := tui.RunLobby()
	if err != nil {
		return err
	}
	switch res.Mode {
	case tui.LobbyModeQuit:
		return nil
	case tui.LobbyModeLocal:
		return runLobbyLocal(res)
	case tui.LobbyModeHost:
		return runLobbyHost(res)
	case tui.LobbyModeJoin:
		return runLobbyJoin(res)
	}
	return nil
}

// runLobbyLocal: single-process play, you at seat 0 + bots from lobby config.
func runLobbyLocal(res tui.LobbyResult) error {
	rule, err := pickRule(res.Rule)
	if err != nil {
		return err
	}
	model := tui.NewPlayModel(rule)
	prog := tea.NewProgram(model, tea.WithAltScreen())

	go func() {
		players := buildLocalPlayersNamed(res, prog, res.PlayerName)
		obs := tui.NewTUIObserver(prog)
		log := slog.New(slog.NewTextHandler(io.Discard, nil))
		_, err := game.RunRoundWithObserver(rule, players, 0, log, obs)
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
	log := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelInfo}))

	// Announce via mDNS so joiners can find us.
	if closer, dErr := discovery.Announce("", 7777, []string{"rule=" + res.Rule}); dErr == nil {
		defer closer.Close()
		log.Info("mDNS announced", "service", discovery.ServiceType, "port", 7777)
	}

	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt)
	defer cancel()

	if !res.HostBot {
		// Pure server (you don't play). Fill non-remote seats with bots
		// per lobby choice; remote seats wait for clients.
		players := buildServerSeatsNamed(res, nil, res.PlayerName)
		cfg := server.Config{
			Addr:        ":7777",
			JoinTimeout: res.Wait,
			Log:         log,
			Rule:        rule,
			Players:     players,
		}
		return server.Run(ctx, cfg)
	}

	// You ARE playing seat 0 — launch TUI + run server in goroutine.
	model := tui.NewPlayModel(rule)
	model.Banner = formatHostBanner(res, 7777)
	prog := tea.NewProgram(model, tea.WithAltScreen())
	players := buildServerSeatsNamed(res, prog, res.PlayerName)
	obs := tui.NewTUIObserver(prog)

	go func() {
		cfg := server.Config{
			Addr:          ":7777",
			JoinTimeout:   res.Wait,
			Log:           log,
			Rule:          rule,
			Players:       players,
			ExtraObserver: obs,
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
