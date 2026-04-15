// Package rules defines the abstraction shared by every supported
// mahjong variant (Sichuan, Riichi, ...). The game state machine in
// internal/game depends only on this interface; rule-specific logic
// (allowed calls, win validation, scoring) lives in subpackages.
package rules

import "github.com/Minalinnski/RonTama/internal/tile"

// RuleSet is everything internal/game needs to know about a variant.
type RuleSet interface {
	// Name returns a human-readable rule identifier ("sichuan-bloodbattle", "riichi", ...).
	Name() string

	// TileKinds returns the set of tile kinds in the wall.
	// Sichuan: 27 suited tiles. Riichi: all 34.
	TileKinds() []tile.Tile

	// CopiesPerTile is the count of each kind in the wall (always 4 in
	// supported variants but exposed for clarity).
	CopiesPerTile() int

	// HandSize is the concealed-hand size at deal time, before the
	// dealer's first draw (always 13 in supported variants).
	HandSize() int

	// AllowsChi reports whether 吃 (run-call from previous player) is allowed.
	// Sichuan: false. Riichi: true.
	AllowsChi() bool

	// RequiresDingque reports whether each player must renounce a suit
	// before play (Sichuan).
	RequiresDingque() bool

	// CanWin reports whether the given concealed hand + winning tile
	// constitutes a valid agari under the rule.
	CanWin(hand tile.Hand, winTile tile.Tile, ctx WinContext) bool

	// ScoreWin returns the score breakdown for a win. Behavior is
	// undefined when CanWin would return false.
	ScoreWin(hand tile.Hand, winTile tile.Tile, ctx WinContext) Score
}

// WinContext is the situational metadata around a potential win that
// affects validity / scoring (tsumo vs ron, dingque suit, last-tile
// flags, etc.).
type WinContext struct {
	WinningTile tile.Tile
	Tsumo       bool      // self-drawn win (otherwise: ron / discard win)
	From        int       // seat that discarded the winning tile (-1 if tsumo)
	Seat        int       // winner's seat (0..3)
	Dealer      int       // dealer seat for the round
	Dingque     tile.Suit // Sichuan-only: this player's renounced suit
	LastTile    bool      // 海底捞月 / 河底捞鱼
	AfterKan    bool      // 杠上开花
	KanGrab     bool      // 抢杠胡
}

// IsDealer reports whether the winner was the dealer this round.
func (c WinContext) IsDealer() bool { return c.Seat == c.Dealer }

// Score is a rule-agnostic scoring result.
type Score struct {
	Patterns []string // human-readable yaku / 番型 names
	Fan      int      // total fan / 番 (rule-specific aggregation)
	BasePts  int      // base points (rule-specific; what the loser(s) pay × seat factor)
}
