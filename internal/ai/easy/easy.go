// Package easy is the lowest-difficulty bot: shanten-greedy.
//
// Behavior:
//   - Dingque: pick the suit with the fewest tiles (least useful)
//   - On draw: declare tsumo if possible; otherwise discard the tile
//     whose removal yields the lowest shanten, breaking ties by the
//     largest count of advancing tiles after the discard
//   - On call opportunity: always ron if available, never pon/kan
//     (stay concealed for simplicity)
//
// This is intentionally a "beginner human" baseline — lower difficulties
// add no defense, no EV, no rich melding. Phase 3 layers Medium/Hard on
// top of these primitives.
package easy

import (
	"github.com/Minalinnski/RonTama/internal/game"
	"github.com/Minalinnski/RonTama/internal/rules"
	"github.com/Minalinnski/RonTama/internal/shanten"
	"github.com/Minalinnski/RonTama/internal/tile"
)

// Bot is the shanten-greedy player.
type Bot struct {
	N string // optional name
}

// New returns a shanten-greedy bot with the given display name.
func New(name string) *Bot { return &Bot{N: name} }

// Name implements game.Player.
func (b *Bot) Name() string {
	if b.N == "" {
		return "easy"
	}
	return b.N
}

// ChooseDingque picks the suit (man/pin/sou) with the fewest tiles in hand.
func (b *Bot) ChooseDingque(view game.PlayerView) tile.Suit {
	counts := [3]int{}
	for s := 0; s < 3; s++ {
		for n := 0; n < 9; n++ {
			counts[s] += view.OwnHand.Concealed[s*9+n]
		}
	}
	best := tile.SuitMan
	bestCnt := counts[0]
	for s := 1; s < 3; s++ {
		if counts[s] < bestCnt {
			bestCnt = counts[s]
			best = tile.Suit(s)
		}
	}
	return best
}

// shantenFn returns the appropriate shanten function for the rule.
func shantenFn(r rules.RuleSet) func([tile.NumKinds]int, int) int {
	if r.RequiresDingque() {
		return shanten.OfSichuan
	}
	return shanten.Of
}

// OnDraw implements game.Player.
func (b *Bot) OnDraw(view game.PlayerView) game.DrawAction {
	hand := view.OwnHand
	melds := len(hand.Melds)
	sf := shantenFn(view.Rule)

	// Tsumo check via "is current count vector a winning hand?"
	if view.JustDrew != nil {
		ctx := rules.WinContext{
			WinningTile: *view.JustDrew,
			Tsumo:       true,
			From:        -1,
			Seat:        view.Seat,
			Dealer:      view.Dealer,
			Dingque:     view.Dingque[view.Seat],
		}
		// hand currently includes the drawn tile; CanWin re-adds, so
		// remove the drawn tile temporarily.
		concealed := hand.Concealed
		concealed[*view.JustDrew]--
		probe := tile.Hand{Concealed: concealed, Melds: hand.Melds}
		if view.Rule.CanWin(probe, *view.JustDrew, ctx) {
			return game.DrawAction{Kind: game.DrawTsumo}
		}
	}

	// Greedy discard: try removing each kind, recompute shanten.
	dingque := view.Dingque[view.Seat]
	concealed := hand.Concealed

	// Sichuan: dingque tiles must be discarded first.
	for i := 0; i < tile.NumKinds; i++ {
		if concealed[i] > 0 && tile.Tile(i).Suit() == dingque {
			return game.DrawAction{Kind: game.DrawDiscard, Discard: tile.Tile(i)}
		}
	}

	bestTile := tile.Tile(0)
	haveBest := false
	bestShanten := 99
	bestAdvance := -1

	// Compute "seen" counts for advancing-tile tally — own hand + own melds + visible discards.
	seen := computeSeen(view)

	for i := 0; i < tile.NumKinds; i++ {
		if concealed[i] == 0 {
			continue
		}
		concealed[i]--
		sh := sf(concealed, melds)
		// Count advancing tiles
		adv := 0
		for j := 0; j < tile.NumKinds; j++ {
			if seen[j] >= 4 {
				continue
			}
			concealed[j]++
			if sf(concealed, melds) < sh {
				adv++
			}
			concealed[j]--
		}
		concealed[i]++

		better := !haveBest ||
			sh < bestShanten ||
			(sh == bestShanten && adv > bestAdvance)
		if better {
			haveBest = true
			bestShanten = sh
			bestAdvance = adv
			bestTile = tile.Tile(i)
		}
	}
	return game.DrawAction{Kind: game.DrawDiscard, Discard: bestTile}
}

// OnCallOpportunity implements game.Player.
//
// Easy bot: always ron if offered, otherwise pass.
func (b *Bot) OnCallOpportunity(view game.PlayerView, discarded tile.Tile, from int, opps []game.Call) game.Call {
	for _, o := range opps {
		if o.Kind == game.CallRon {
			return o
		}
	}
	return game.Pass
}

// computeSeen tallies tile copies visible to this seat: own concealed,
// melds (including others'), and discards.
func computeSeen(view game.PlayerView) [tile.NumKinds]int {
	var seen [tile.NumKinds]int
	for i, c := range view.OwnHand.Concealed {
		seen[i] = c
	}
	for p := 0; p < game.NumPlayers; p++ {
		for _, m := range view.Melds[p] {
			for _, t := range m.Tiles {
				seen[t]++
			}
		}
		for _, t := range view.Discards[p] {
			seen[t]++
		}
	}
	return seen
}
