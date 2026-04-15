package main

import (
	"flag"
	"fmt"
	"io"
	"log/slog"
	"os"
	"strings"

	"github.com/Minalinnski/RonTama/internal/ai"
	"github.com/Minalinnski/RonTama/internal/ai/easy"
	"github.com/Minalinnski/RonTama/internal/ai/hard"
	"github.com/Minalinnski/RonTama/internal/ai/medium"
	"github.com/Minalinnski/RonTama/internal/game"
	"github.com/Minalinnski/RonTama/internal/rules/sichuan"
)

// newBot constructs the concrete game.Player for a given difficulty.
// Lives here (rather than internal/ai) because importing the tier
// subpackages from internal/ai would form a cycle.
func newBot(d ai.Difficulty, name string) game.Player {
	switch d {
	case ai.Easy:
		return easy.New(name)
	case ai.Medium:
		return medium.New(name)
	case ai.Hard:
		return hard.New(name)
	default:
		return easy.New(name)
	}
}

// runBotbattle plays N rounds with a configured set of bots and prints
// per-seat aggregate stats. Used to validate that Hard > Medium > Easy.
//
// Example:
//
//	rontama botbattle -rounds 1000 -seats easy,easy,medium,hard
func runBotbattle(args []string) error {
	fs := flag.NewFlagSet("botbattle", flag.ExitOnError)
	rounds := fs.Int("rounds", 200, "rounds to play")
	seatsRaw := fs.String("seats", "easy,medium,hard,hard", "comma-separated bot tiers per seat")
	if err := fs.Parse(args); err != nil {
		return err
	}

	tiers := strings.Split(*seatsRaw, ",")
	if len(tiers) != game.NumPlayers {
		return fmt.Errorf("-seats expects %d entries, got %d", game.NumPlayers, len(tiers))
	}

	var players [game.NumPlayers]game.Player
	for i, name := range tiers {
		d, err := ai.ParseDifficulty(strings.TrimSpace(name))
		if err != nil {
			return err
		}
		players[i] = newBot(d, fmt.Sprintf("seat%d-%s", i, d))
	}

	silent := slog.New(slog.NewTextHandler(io.Discard, nil))
	rule := sichuan.New()

	type stats struct {
		wins        int
		tsumos      int
		rons        int
		dealtIn     int
		exhaustions int
		score       int
	}
	var per [game.NumPlayers]stats

	for r := 0; r < *rounds; r++ {
		dealer := r % game.NumPlayers
		res, err := game.RunRound(rule, players, dealer, silent)
		if err != nil {
			return fmt.Errorf("round %d: %w", r, err)
		}
		for i, s := range res.FinalScores {
			per[i].score += s
		}
		for _, w := range res.Wins {
			per[w.Seat].wins++
			if w.Tsumo {
				per[w.Seat].tsumos++
			} else {
				per[w.Seat].rons++
				if w.From >= 0 {
					per[w.From].dealtIn++
				}
			}
		}
		if res.Exhaustion {
			for i := range per {
				per[i].exhaustions++
			}
		}
	}

	fmt.Fprintf(os.Stdout, "BotBattle: %d rounds, seats=%s\n\n", *rounds, *seatsRaw)
	fmt.Fprintf(os.Stdout, "%-20s %8s %8s %8s %8s %12s %12s\n",
		"seat", "wins", "tsumo", "ron", "dealt-in", "score", "score/rd")
	for i, s := range per {
		fmt.Fprintf(os.Stdout, "%-20s %8d %8d %8d %8d %12d %12.3f\n",
			fmt.Sprintf("%d (%s)", i, tiers[i]),
			s.wins, s.tsumos, s.rons, s.dealtIn, s.score, float64(s.score)/float64(*rounds))
	}
	return nil
}
