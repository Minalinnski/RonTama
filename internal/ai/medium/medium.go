// Package medium is the mid-tier bot: shanten greedy with a
// value-aware tiebreaker.
//
// Differences from Easy:
//   - When evaluating discards at the same shanten + advance count,
//     prefer the discard that keeps the hand closer to a high-value
//     pattern: 清一色 (single suit) and 大对子 / 七对 (pairs-heavy).
//   - Dingque: pick the suit minimising "regret" — both raw count and
//     scattered isolation (a 1m alone is cheap to drop; a 1-2-3m run is
//     expensive). Calculated as: tiles + paired/runnable bonus.
//
// Defense and rich melding are still skipped (those are Hard).
package medium

import (
	"github.com/Minalinnski/RonTama/internal/ai"
	"github.com/Minalinnski/RonTama/internal/ai/easy"
	"github.com/Minalinnski/RonTama/internal/game"
	"github.com/Minalinnski/RonTama/internal/tile"
)

// Bot extends Easy with value-weighted tiebreaks.
type Bot struct {
	N string
}

// New returns a Medium bot.
func New(n string) *Bot { return &Bot{N: n} }

// Name implements game.Player.
func (b *Bot) Name() string {
	if b.N == "" {
		return "medium"
	}
	return b.N
}

// ChooseDingque considers both tile count and structural value (pairs,
// adjacent tiles). Lower score = better dingque candidate.
func (b *Bot) ChooseDingque(view game.PlayerView) tile.Suit {
	scores := [3]int{}
	c := view.OwnHand.Concealed
	for s := 0; s < 3; s++ {
		for n := 0; n < 9; n++ {
			cnt := c[s*9+n]
			scores[s] += cnt
			if cnt >= 2 {
				scores[s] += 2 // pairs are valuable
			}
			if n < 8 && cnt > 0 && c[s*9+n+1] > 0 {
				scores[s]++ // adjacent tile = potential run
			}
			if n < 7 && cnt > 0 && c[s*9+n+2] > 0 {
				scores[s]++ // kanchan
			}
		}
	}
	best := tile.SuitMan
	bestScore := scores[0]
	for s := 1; s < 3; s++ {
		if scores[s] < bestScore {
			bestScore = scores[s]
			best = tile.Suit(s)
		}
	}
	return best
}

// OnDraw uses the value-aware discard policy.
func (b *Bot) OnDraw(view game.PlayerView) game.DrawAction {
	if ai.CanTsumo(view) {
		return game.DrawAction{Kind: game.DrawTsumo}
	}
	if t, ok := ai.MustDiscardDingque(view); ok {
		return game.DrawAction{Kind: game.DrawDiscard, Discard: t}
	}
	t := PickValueDiscard(view)
	act := game.DrawAction{Kind: game.DrawDiscard, Discard: t}
	if ai.ShouldDeclareRiichi(view, t) {
		act.DeclareRiichi = true
	}
	return act
}

// OnCallOpportunity: same as Easy (always ron, never call to stay concealed).
func (b *Bot) OnCallOpportunity(view game.PlayerView, _ tile.Tile, _ int, opps []game.Call) game.Call {
	return ai.AlwaysRon(opps)
}

// PickValueDiscard picks a discard like Easy.PickGreedyDiscard but
// breaks ties by preferring the candidate that better preserves
// progress toward a high-value shape.
//
// Value bonus per remaining hand:
//   +3 if all remaining tiles are one suit (清一色 in sight)
//   +1 per pair beyond 3 (七对 / 大对子 in sight)
func PickValueDiscard(view game.PlayerView) tile.Tile {
	hand := view.OwnHand
	melds := len(hand.Melds)
	sf := ai.ShantenFn(view.Rule)
	concealed := hand.Concealed
	seen := ai.ComputeSeen(view)

	type cand struct {
		t       tile.Tile
		sh, adv int
		val     int
	}
	var best *cand

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
		val := valueBonus(concealed, melds)
		concealed[i]++

		c := cand{t: tile.Tile(i), sh: sh, adv: adv, val: val}
		if best == nil ||
			c.sh < best.sh ||
			(c.sh == best.sh && c.adv > best.adv) ||
			(c.sh == best.sh && c.adv == best.adv && c.val > best.val) {
			tmp := c
			best = &tmp
		}
	}
	if best == nil {
		// fallback (shouldn't happen): use easy
		return easy.PickGreedyDiscard(view)
	}
	return best.t
}

func valueBonus(c [tile.NumKinds]int, melds int) int {
	bonus := 0
	// Single-suit bonus: if every nonzero tile is in one suit (and no melds
	// of other suits — assumption: melds checked elsewhere).
	suit := tile.Suit(255)
	single := true
	pairs := 0
	for i := 0; i < tile.NumKinds; i++ {
		if c[i] == 0 {
			continue
		}
		s := tile.Tile(i).Suit()
		if suit == 255 {
			suit = s
		} else if suit != s {
			single = false
		}
		if c[i] == 2 {
			pairs++
		}
	}
	if single && melds == 0 {
		bonus += 3
	}
	if pairs >= 4 && melds == 0 {
		bonus += pairs - 3
	}
	return bonus
}
