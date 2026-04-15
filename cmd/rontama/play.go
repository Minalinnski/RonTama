package main

import (
	"flag"
	"fmt"
	"io"
	"log/slog"
	"os"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/Minalinnski/RonTama/internal/ai/easy"
	"github.com/Minalinnski/RonTama/internal/game"
	"github.com/Minalinnski/RonTama/internal/rules/sichuan"
	"github.com/Minalinnski/RonTama/internal/tui"
)

// runPlay drives Sichuan rounds. -tui launches the interactive Bubble
// Tea UI with the human at seat 0; otherwise prints round results to
// stdout (4-bot only).
func runPlay(args []string) error {
	fs := flag.NewFlagSet("play", flag.ExitOnError)
	rounds := fs.Int("rounds", 1, "number of rounds to run")
	verbose := fs.Bool("v", false, "verbose log (game-level events)")
	useTUI := fs.Bool("tui", false, "launch interactive TUI (you at seat 0, 3 easy bots)")
	if err := fs.Parse(args); err != nil {
		return err
	}

	if *useTUI {
		return runPlayTUI()
	}

	logLevel := slog.LevelInfo
	if *verbose {
		logLevel = slog.LevelDebug
	}
	log := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: logLevel}))

	rule := sichuan.New()
	players := [game.NumPlayers]game.Player{
		easy.New("E"), easy.New("S"), easy.New("W"), easy.New("N"),
	}

	totals := [game.NumPlayers]int{}
	for r := 0; r < *rounds; r++ {
		dealer := r % game.NumPlayers
		res, err := game.RunRound(rule, players, dealer, log)
		if err != nil {
			return fmt.Errorf("round %d: %w", r, err)
		}
		printRoundResult(os.Stdout, r, res)
		for i := 0; i < game.NumPlayers; i++ {
			totals[i] += res.FinalScores[i]
		}
	}
	if *rounds > 1 {
		fmt.Fprintln(os.Stdout, "\n=== Cumulative ===")
		for i := 0; i < game.NumPlayers; i++ {
			fmt.Fprintf(os.Stdout, "  %s: %+d\n", players[i].Name(), totals[i])
		}
	}
	return nil
}

// runPlayTUI launches the interactive Bubble Tea program for a single round.
//
// Architecture:
//   - main goroutine runs the Bubble Tea Program (UI)
//   - background goroutine runs game.RunRoundWithObserver
//   - human seat 0 uses tui.HumanPlayer which sends prompts via prog.Send
//     and blocks on a response channel that the UI populates from key handlers
//   - bot seats use easy.Bot
//   - tui.TUIObserver pushes EventMsg to the UI on every public state change
func runPlayTUI() error {
	rule := sichuan.New()
	model := tui.NewPlayModel(rule)
	prog := tea.NewProgram(model, tea.WithAltScreen())

	// Game goroutine.
	go func() {
		players := [game.NumPlayers]game.Player{
			tui.NewHumanPlayer("you", prog),
			easy.New("S-bot"),
			easy.New("W-bot"),
			easy.New("N-bot"),
		}
		obs := tui.NewTUIObserver(prog)
		// Silent slog — TUI uses observer events for display.
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

func printRoundResult(w io.Writer, idx int, r *game.RoundResult) {
	fmt.Fprintf(w, "\n=== Round %d ===\n", idx+1)
	if len(r.Wins) == 0 {
		fmt.Fprintln(w, "  no wins (wall exhaustion)")
	}
	for _, win := range r.Wins {
		who := "tsumo"
		if !win.Tsumo {
			who = fmt.Sprintf("ron from seat %d", win.From)
		}
		fmt.Fprintf(w, "  seat %d wins via %s on %s: %v (fan=%d, base=%d)\n",
			win.Seat, who, win.Tile, win.Score.Patterns, win.Score.Fan, win.Score.BasePts)
	}
	fmt.Fprintln(w, "  Final scores:")
	for i := 0; i < game.NumPlayers; i++ {
		fmt.Fprintf(w, "    seat %d: %+d\n", i, r.FinalScores[i])
	}
	if r.Exhaustion {
		fmt.Fprintln(w, "  (wall exhausted)")
	}
}
