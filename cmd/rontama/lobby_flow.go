package main

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"os"
	"os/signal"

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
		players := buildLocalPlayers(res, prog)
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
		players := buildServerSeats(res, nil)
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
	prog := tea.NewProgram(model, tea.WithAltScreen())
	players := buildServerSeats(res, prog)
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
	return joinAsTUI(res.JoinAt, rule, log)
}

// buildLocalPlayers turns lobby seat config into a [4]game.Player for local play.
func buildLocalPlayers(res tui.LobbyResult, prog *tea.Program) [game.NumPlayers]game.Player {
	var players [game.NumPlayers]game.Player
	for i, role := range res.Seats {
		players[i] = playerFromRole(role, i, prog)
		if players[i] == nil {
			// SeatRemote shouldn't appear in local games; treat as Easy.
			players[i] = easy.New(fmt.Sprintf("seat%d-fallback", i))
		}
	}
	return players
}

// buildServerSeats turns lobby seat config into the Players[4] for server.Config:
// SeatRemote → nil (server fills from connection)
// SeatHuman + prog != nil → HumanPlayer (host plays via TUI)
// SeatBot* → corresponding bot
func buildServerSeats(res tui.LobbyResult, prog *tea.Program) [game.NumPlayers]game.Player {
	var players [game.NumPlayers]game.Player
	for i, role := range res.Seats {
		switch role {
		case tui.SeatRemote:
			players[i] = nil
		case tui.SeatHuman:
			if prog != nil {
				players[i] = tui.NewHumanPlayer("you", prog)
			}
		default:
			players[i] = playerFromRole(role, i, prog)
		}
	}
	return players
}

// playerFromRole maps a single lobby SeatRole to a concrete game.Player.
func playerFromRole(role tui.SeatRole, seat int, prog *tea.Program) game.Player {
	switch role {
	case tui.SeatHuman:
		if prog != nil {
			return tui.NewHumanPlayer("you", prog)
		}
		return easy.New("you-fallback")
	case tui.SeatBotEasy:
		return easy.New(fmt.Sprintf("E%d", seat))
	case tui.SeatBotMedium:
		return medium.New(fmt.Sprintf("M%d", seat))
	case tui.SeatBotHard:
		return hard.New(fmt.Sprintf("H%d", seat))
	}
	return nil
}

// _ = client kept for potential future direct use of the client package
// in this file (e.g. reusing ParseRule or shared join helpers).
var _ = client.NewHeadlessDecider
var _ rules.RuleSet
