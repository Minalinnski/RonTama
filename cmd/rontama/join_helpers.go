package main

import (
	"fmt"
	"log/slog"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/Minalinnski/RonTama/internal/ai/easy"
	"github.com/Minalinnski/RonTama/internal/net/client"
	"github.com/Minalinnski/RonTama/internal/rules"
	"github.com/Minalinnski/RonTama/internal/tui"
)

// joinAsHeadlessBot dials the server and runs the protocol with an
// Easy bot driving the responses.
func joinAsHeadlessBot(addr string, rule rules.RuleSet, log *slog.Logger) error {
	bot := easy.New("net-easy")
	decider := client.NewHeadlessDecider(bot, rule)

	c, err := client.Dial(addr, decider, log)
	if err != nil {
		return err
	}
	defer c.Close()
	return c.Run()
}

// joinAsTUI launches the interactive Bubble Tea play model and wires
// it to the network client. The TUI runs in the foreground; the
// network read loop runs in a goroutine and pushes events to the program.
func joinAsTUI(addr string, rule rules.RuleSet, log *slog.Logger) error {
	model := tui.NewPlayModel(rule)
	prog := tea.NewProgram(model, tea.WithAltScreen())
	decider := client.NewTUIDecider(prog, rule)
	c, err := client.Dial(addr, decider, log)
	if err != nil {
		return err
	}
	defer c.Close()

	go func() {
		if err := c.Run(); err != nil {
			prog.Send(tui.RoundDoneMsg{Err: err})
			return
		}
		// graceful disconnect — RoundEnd already triggered RoundDoneMsg
	}()

	if _, err := prog.Run(); err != nil {
		return fmt.Errorf("tui: %w", err)
	}
	return nil
}
