// Command rontama is the entrypoint for the RonTama mahjong game.
//
// Subcommands: see printUsage() below.
package main

import (
	"fmt"
	"os"

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
		// No args → launch the lobby. The lobby returns a config; main
		// dispatches into local play, host, or join based on it.
		if err := runLobbyFlow(); err != nil {
			fmt.Fprintln(os.Stderr, "lobby:", err)
			os.Exit(1)
		}
		return
	}
	cmd, rest := args[0], args[1:]
	switch cmd {
	case "lobby":
		if err := runLobbyFlow(); err != nil {
			fmt.Fprintln(os.Stderr, "lobby:", err)
			os.Exit(1)
		}
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

func printUsage() {
	fmt.Fprint(os.Stderr, `rontama — terminal mahjong (Sichuan + Riichi)

Just run it:
  rontama              open the lobby (new local / host LAN / join LAN)

That covers everything the average player needs. The remaining
subcommands are for scripts, automation, and bot research:

  rontama version                       print build version
  rontama play [-rule sichuan|riichi] [-rounds N] [-tui]
                                        launch a game directly (skip the lobby)
  rontama serve [-port 7777]            host a LAN game directly (skip the lobby)
  rontama join  [-addr host:port] [-bot]
                                        join a LAN game directly (skip the lobby)
  rontama botbattle [-rounds N] [-seats easy,medium,hard,hard]
                                        4-bot stress / strength comparison
  rontama tui                           hello-world placeholder (legacy)
`)
}
