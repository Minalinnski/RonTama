// Package hard is the strongest tier. Adds defensive discard on top of
// Medium's value-aware policy.
//
// Defense model (intentionally lightweight — proper放铳率 modeling is a
// research project):
//   - Estimate per-tile "danger" using the simple genbutsu rule:
//     tiles that have been discarded by an opponent are 100% safe
//     against that opponent (cannot ron a tile they discarded under
//     furiten-equivalent reasoning).
//   - Approximate "threat level" of opponents from their discard
//     count: opponents who've discarded ~14+ tiles are likely close to
//     tenpai.
//   - When at least one opponent is "threatening" and our own shanten
//     is >= 1 (we're not in tenpai), bias the discard toward the
//     safest tile that doesn't increase shanten by more than 1.
//
// At tenpai, we still play to win — defense yields to offense.
package hard

import (
	"github.com/Minalinnski/RonTama/internal/ai"
	"github.com/Minalinnski/RonTama/internal/ai/medium"
	"github.com/Minalinnski/RonTama/internal/game"
	"github.com/Minalinnski/RonTama/internal/tile"
)

// Bot is the defensive Medium-extension.
type Bot struct {
	N string
}

// New returns a Hard bot.
func New(n string) *Bot { return &Bot{N: n} }

// Name implements game.Player.
func (b *Bot) Name() string {
	if b.N == "" {
		return "hard"
	}
	return b.N
}

// ChooseDingque uses Medium's structural-value policy.
func (b *Bot) ChooseDingque(view game.PlayerView) tile.Suit {
	return medium.New(b.N).ChooseDingque(view)
}

// OnDraw layers defense on top of the value-aware policy.
func (b *Bot) OnDraw(view game.PlayerView) game.DrawAction {
	if ai.CanTsumo(view) {
		return game.DrawAction{Kind: game.DrawTsumo}
	}
	if t, ok := ai.MustDiscardDingque(view); ok {
		return game.DrawAction{Kind: game.DrawDiscard, Discard: t}
	}

	// Only switch to defensive selection when:
	//   - Opponents are clearly mid-late game (>= 18 discards)
	//   - We're far enough from tenpai that pushing wastes turns (>= 2-shanten)
	// Otherwise stay on Medium's value-aware policy. Defense slows our
	// attack, so over-applying it actually loses more than it saves.
	threat := threatLevel(view)
	mySh := ourShanten(view)
	if threat >= 18 && mySh >= 2 {
		t := PickSafeDiscard(view, mySh)
		return game.DrawAction{Kind: game.DrawDiscard, Discard: t}
	}
	t := medium.PickValueDiscard(view)
	return game.DrawAction{Kind: game.DrawDiscard, Discard: t}
}

// OnCallOpportunity: ron when offered, otherwise pass.
func (b *Bot) OnCallOpportunity(view game.PlayerView, _ tile.Tile, _ int, opps []game.Call) game.Call {
	return ai.AlwaysRon(opps)
}

// threatLevel returns the maximum discard count among opponents who
// haven't already won. A high value (~14+ in Sichuan's 108-tile wall)
// means at least one opponent is plausibly close to tenpai.
func threatLevel(view game.PlayerView) int {
	max := 0
	for p := 0; p < game.NumPlayers; p++ {
		if p == view.Seat || view.HasWon[p] {
			continue
		}
		if n := len(view.Discards[p]); n > max {
			max = n
		}
	}
	return max
}

// ourShanten returns the current concealed-hand shanten.
func ourShanten(view game.PlayerView) int {
	sf := ai.ShantenFn(view.Rule)
	return sf(view.OwnHand.Concealed, len(view.OwnHand.Melds))
}

// PickSafeDiscard selects the safest tile whose discard doesn't
// increase shanten by more than 1 from current. Safety = is the tile
// already in any live opponent's discard pile (genbutsu)?
func PickSafeDiscard(view game.PlayerView, currentShanten int) tile.Tile {
	hand := view.OwnHand
	melds := len(hand.Melds)
	sf := ai.ShantenFn(view.Rule)
	concealed := hand.Concealed

	// Build the set of "safe" tiles per opponent.
	threats := []int{}
	for p := 0; p < game.NumPlayers; p++ {
		if p == view.Seat || view.HasWon[p] {
			continue
		}
		threats = append(threats, p)
	}
	safeAgainst := func(t tile.Tile) int {
		// Returns number of threats for whom t is genbutsu (already in
		// their river).
		safe := 0
		for _, p := range threats {
			for _, d := range view.Discards[p] {
				if d == t {
					safe++
					break
				}
			}
		}
		return safe
	}

	type cand struct {
		t            tile.Tile
		safeCount    int
		shantenDelta int
	}
	var best *cand
	for i := 0; i < tile.NumKinds; i++ {
		if concealed[i] == 0 {
			continue
		}
		concealed[i]--
		sh := sf(concealed, melds)
		concealed[i]++
		delta := sh - currentShanten
		if delta > 1 {
			continue // too costly to our offense
		}
		t := tile.Tile(i)
		c := cand{t: t, safeCount: safeAgainst(t), shantenDelta: delta}
		if best == nil ||
			c.safeCount > best.safeCount ||
			(c.safeCount == best.safeCount && c.shantenDelta < best.shantenDelta) {
			tmp := c
			best = &tmp
		}
	}
	if best == nil {
		// Couldn't find any acceptable discard within the offense budget;
		// fall back to medium policy.
		return medium.PickValueDiscard(view)
	}
	return best.t
}
