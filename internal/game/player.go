package game

import "github.com/Minalinnski/RonTama/internal/tile"

// Player is the decision interface implemented by bots, network
// clients, or human-input adapters. All methods receive a PlayerView
// snapshot and return the chosen action; no side effects.
type Player interface {
	// Name is a stable identifier used in logs.
	Name() string

	// ChooseDingque is called once at round start (Sichuan only).
	// Implementations should pick the suit with the fewest / weakest
	// holdings. For non-Sichuan rules this is never called.
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
