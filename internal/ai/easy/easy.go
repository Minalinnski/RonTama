// Package easy is the lowest-difficulty bot: shanten-greedy.
//
// Behavior:
//   - Dingque: the suit with the fewest tiles in hand
//   - On draw: tsumo if available; otherwise discard the tile whose
//     removal yields the lowest shanten (tiebreak by largest count of
//     advancing tiles after the discard)
//   - On call opportunity: always ron, never pon/kan
//
// This is a beginner-human baseline. Medium and Hard layer EV and
// defense on top.
package easy

import (
	"github.com/Minalinnski/RonTama/internal/ai"
	"github.com/Minalinnski/RonTama/internal/game"
	"github.com/Minalinnski/RonTama/internal/tile"
)

// Bot is the shanten-greedy player.
type Bot struct {
	N string
}

// New returns an Easy bot with display name n.
func New(n string) *Bot { return &Bot{N: n} }

// Name implements game.Player.
func (b *Bot) Name() string {
	if b.N == "" {
		return "easy"
	}
	return b.N
}

// ChooseDingque implements game.Player.
func (b *Bot) ChooseDingque(view game.PlayerView) tile.Suit {
	return ai.ChooseDingqueLeastTiles(view)
}

// OnDraw implements game.Player.
func (b *Bot) OnDraw(view game.PlayerView) game.DrawAction {
	if ai.CanTsumo(view) {
		return game.DrawAction{Kind: game.DrawTsumo}
	}
	if t, ok := ai.MustDiscardDingque(view); ok {
		return game.DrawAction{Kind: game.DrawDiscard, Discard: t}
	}
	t := PickGreedyDiscard(view)
	act := game.DrawAction{Kind: game.DrawDiscard, Discard: t}
	if ai.ShouldDeclareRiichi(view, t) {
		act.DeclareRiichi = true
	}
	return act
}

// OnCallOpportunity implements game.Player.
func (b *Bot) OnCallOpportunity(view game.PlayerView, _ tile.Tile, _ int, opps []game.Call) game.Call {
	return ai.AlwaysRon(opps)
}

// PickGreedyDiscard returns the discard that minimises shanten,
// breaking ties by most advancing tiles. Exported for reuse by Medium
// and Hard which extend this primitive.
func PickGreedyDiscard(view game.PlayerView) tile.Tile {
	hand := view.OwnHand
	melds := len(hand.Melds)
	sf := ai.ShantenFn(view.Rule)
	concealed := hand.Concealed
	seen := ai.ComputeSeen(view)

	bestTile := tile.Tile(0)
	haveBest := false
	bestShanten := 99
	bestAdvance := -1

	for i := 0; i < tile.NumKinds; i++ {
		if concealed[i] == 0 {
			continue
		}
		concealed[i]--
		sh := sf(concealed, melds)
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
	return bestTile
}
