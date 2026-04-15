package game

import "github.com/Minalinnski/RonTama/internal/tile"

// Player is the decision interface implemented by bots, network
// clients, or human-input adapters. All methods receive a PlayerView
// snapshot and return the chosen action; no side effects.
type Player interface {
	// Name is a stable identifier used in logs.
	Name() string

	// ChooseExchange3 is called once at round start when the rule
	// requires 换三张 (Sichuan). Returns 3 tiles of one suit to pass.
	// Caller validates same-suit + tiles-in-hand and panics on bad
	// returns; bots / clients should always honour the contract.
	ChooseExchange3(view PlayerView) [3]tile.Tile

	// ChooseDingque is called once at round start (Sichuan only),
	// AFTER ChooseExchange3 has settled. The hand passed via view
	// reflects the post-exchange tiles.
	ChooseDingque(view PlayerView) tile.Suit

	// OnDraw is called after this player draws a tile (the draw is
	// already in view.OwnHand and view.JustDrew). The player decides
	// to discard, declare tsumo, or declare a kan.
	OnDraw(view PlayerView) DrawAction

	// OnCallOpportunity is called when another player discards a tile
	// that this player could pon / kan / ron. opportunities lists the
	// available calls; the player returns the chosen one (or Pass).
	OnCallOpportunity(view PlayerView, discarded tile.Tile, from int, opportunities []Call) Call
}
