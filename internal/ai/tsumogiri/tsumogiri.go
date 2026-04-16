// Package tsumogiri is the degenerate "disconnected player" bot:
// discards the drawn tile, never calls, never tsumo, never riichi.
//
// Real-life parlance: 摸切 / ツモ切り — "draw-cut". It's what happens at
// a live mahjong table when someone has to step away: their seat
// discards whatever they draw until they return or the round ends.
//
// Used by the LAN server as the disconnect fallback for a session:
// when a client's TCP connection drops, the seat keeps being active
// (so the round doesn't stall) but plays minimally and predictably.
// Reconnection restores the human to the seat; nothing has changed
// from their perspective except possibly some cheap discards.
package tsumogiri

import (
	"github.com/Minalinnski/RonTama/internal/ai"
	"github.com/Minalinnski/RonTama/internal/game"
	"github.com/Minalinnski/RonTama/internal/tile"
)

// Bot implements game.Player with tsumogiri behaviour.
type Bot struct {
	N string
}

// New returns a tsumogiri bot carrying display name n (typically the
// disconnected human's name, so observers still see who "owns" the seat).
func New(n string) *Bot { return &Bot{N: n} }

// Name implements game.Player. If a name was supplied it's used as-is
// plus a disconnected-marker so other players can tell at a glance.
func (b *Bot) Name() string {
	if b.N == "" {
		return "(disconnected)"
	}
	return b.N + " (掉线)"
}

// ChooseExchange3 falls back to the shared Easy-style policy (suit with
// fewest tiles → least painful to lose).
func (b *Bot) ChooseExchange3(view game.PlayerView) [3]tile.Tile {
	return ai.PickExchange3(view)
}

// ChooseDingque picks the suit with fewest tiles (same as Easy).
func (b *Bot) ChooseDingque(view game.PlayerView) tile.Suit {
	return ai.ChooseDingqueLeastTiles(view)
}

// OnDraw: discard exactly the drawn tile. If Sichuan dingque leaves
// stragglers in the renounced suit, clear those first (the rule
// requires it before any other discard).
func (b *Bot) OnDraw(view game.PlayerView) game.DrawAction {
	// Sichuan: must clear dingque suit before anything else.
	if t, ok := ai.MustDiscardDingque(view); ok {
		return game.DrawAction{Kind: game.DrawDiscard, Discard: t}
	}
	if view.JustDrew != nil {
		return game.DrawAction{Kind: game.DrawDiscard, Discard: *view.JustDrew}
	}
	// Post-call discard (we don't call ourselves but sessionPlayer might
	// have handed over to us mid-turn). Cut the smallest-valued tile.
	for i := 0; i < tile.NumKinds; i++ {
		if view.OwnHand.Concealed[i] > 0 {
			return game.DrawAction{Kind: game.DrawDiscard, Discard: tile.Tile(i)}
		}
	}
	return game.DrawAction{Kind: game.DrawDiscard}
}

// OnCallOpportunity: always pass. No calls, no ron — a tsumogiri
// player doesn't take offered wins either (real rule: if you're
// effectively AFK you can't claim ron).
func (b *Bot) OnCallOpportunity(_ game.PlayerView, _ tile.Tile, _ int, _ []game.Call) game.Call {
	return game.Pass
}
