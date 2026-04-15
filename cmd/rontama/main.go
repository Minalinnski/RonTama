// Command rontama is the entrypoint for the RonTama mahjong game.
//
// In Phase 0 it just launches the placeholder TUI. Subcommands
// (serve, join, play, botbattle) come in later phases.
package main

import (
	"fmt"
	"os"

	"github.com/Minalinnski/RonTama/internal/tui"
)

func main() {
	if _, err := tui.NewHello().Run(); err != nil {
		fmt.Fprintln(os.Stderr, "rontama:", err)
		os.Exit(1)
	}
}
