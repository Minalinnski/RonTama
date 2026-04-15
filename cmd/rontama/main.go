// Command rontama is the entrypoint for the RonTama mahjong game.
//
// Subcommands: see printUsage() below.
package main

import (
	"fmt"
	"os"

	"github.com/Minalinnski/RonTama/internal/tui"
)

// Build-time variables populated by GoReleaser via -ldflags.
var (
	version = "dev"
	commit  = "none"
	date    = "unknown"
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
	case "botbattle":
		if err := runBotbattle(rest); err != nil {
			fmt.Fprintln(os.Stderr, "botbattle:", err)
			os.Exit(1)
		}
	case "serve":
		if err := runServe(rest); err != nil {
			fmt.Fprintln(os.Stderr, "serve:", err)
			os.Exit(1)
		}
	case "join":
		if err := runJoin(rest); err != nil {
			fmt.Fprintln(os.Stderr, "join:", err)
			os.Exit(1)
		}
	case "version", "-v", "--version":
		fmt.Printf("rontama %s (%s, built %s)\n", version, commit, date)
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
  rontama play -tui    interactive 1-round Sichuan: you (seat 0) vs 3 Easy bots
  rontama botbattle [-rounds N] [-seats easy,medium,hard,hard]
                       compare bot tiers over N rounds
  rontama serve [-port 7777] [-timeout 30s]
                       host a Sichuan game over LAN; empty seats fill with Easy bots
  rontama join [-addr host:port] [-rule sichuan|riichi] [-bot]
                       connect to a server (mDNS auto-discover by default).
                       default: launch interactive TUI for seat 0.
                       -bot: headless Easy bot (testing / seat-filling)
  rontama version      print build version
`)
}
