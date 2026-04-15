// Command rontama is the entrypoint for the RonTama mahjong game.
//
// Subcommands:
//
//	rontama          launch the (Phase-0 placeholder) TUI
//	rontama play     run a CLI 4-bot Sichuan match (Phase 2+)
//
// More subcommands (serve, join, botbattle) come in later phases.
package main

import (
	"fmt"
	"os"

	"github.com/Minalinnski/RonTama/internal/tui"
)

func main() {
	args := os.Args[1:]
	if len(args) == 0 {
		runTUI()
		return
	}
	cmd, rest := args[0], args[1:]
	switch cmd {
	case "tui":
		runTUI()
	case "play":
		if err := runPlay(rest); err != nil {
			fmt.Fprintln(os.Stderr, "play:", err)
			os.Exit(1)
		}
	case "-h", "--help", "help":
		printUsage()
	default:
		fmt.Fprintf(os.Stderr, "unknown subcommand %q\n\n", cmd)
		printUsage()
		os.Exit(2)
	}
}

func runTUI() {
	if _, err := tui.NewHello().Run(); err != nil {
		fmt.Fprintln(os.Stderr, "rontama:", err)
		os.Exit(1)
	}
}

func printUsage() {
	fmt.Fprint(os.Stderr, `rontama - terminal mahjong (Sichuan + Riichi)

Usage:
  rontama              launch TUI (Phase 0 placeholder)
  rontama tui          same as above
  rontama play [-v] [-rounds N]
                       run an N-round Sichuan match between 4 Easy bots
`)
}
