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

	// StartingScore is the per-player score at round start. Sichuan
	// uses 0 (rounds settle as deltas); Riichi uses 25000 (so riichi
	// stick payments are real subtractions).
	StartingScore() int

	// AllowsChi reports whether 吃 (run-call from previous player) is allowed.
	// Sichuan: false. Riichi: true.
	AllowsChi() bool

	// RequiresDingque reports whether each player must renounce a suit
	// before play (Sichuan).
	RequiresDingque() bool

	// RequiresExchange3 reports whether the round opens with the
	// "exchange three" (换三张) phase: every player picks 3 tiles of
	// one suit and passes them in a fixed direction. Sichuan only.
	RequiresExchange3() bool

	// CanWin reports whether the given concealed hand + winning tile
	// constitutes a valid agari under the rule.
	CanWin(hand tile.Hand, winTile tile.Tile, ctx WinContext) bool

	// ScoreWin returns the score breakdown for a win. Behavior is
	// undefined when CanWin would return false.
	ScoreWin(hand tile.Hand, winTile tile.Tile, ctx WinContext) Score

	// Settle returns per-seat point deltas for one win, given the
	// already-computed Score, the dealer seat, and which seats have
	// already won this round (for blood-battle: those seats don't pay).
	//
	// The riichi pot (if any) is settled by the caller, not here —
	// rules don't need to know about it.
	Settle(dealer, winner int, ctx WinContext, score Score, hasWon [4]bool) [4]int

	// Hooks returns the rule-specific lifecycle hooks, or nil if the
	// variant doesn't need them (Sichuan). When non-nil, the game
	// loop delegates WinContext construction, action validation, call
	// enumeration, and post-action bookkeeping to these hooks — keeping
	// the loop itself variant-agnostic.
	Hooks() RuleHooks
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

	// Riichi-only:
	RoundWind        tile.Tile // East/South/West/North (場風)
	Riichi           bool      // declared riichi this round
	DoubleRiichi     bool      // declared riichi on first turn (W-riichi)
	Ippatsu          bool      // win within one go-around of riichi declaration
	DoraIndicators   []tile.Tile // visible dora indicators
	UraDoraIndicators []tile.Tile // ura-dora indicators (revealed only on riichi win)
}

// SeatWind returns the player's seat wind tile relative to the dealer.
// Seat 0 sits at the dealer position; (seat - dealer) mod 4 = 0 → East,
// 1 → South, 2 → West, 3 → North.
func (c WinContext) SeatWind() tile.Tile {
	winds := []tile.Tile{tile.East, tile.South, tile.West, tile.North}
	idx := (c.Seat - c.Dealer + 4) % 4
	return winds[idx]
}

// IsDealer reports whether the winner was the dealer this round.
func (c WinContext) IsDealer() bool { return c.Seat == c.Dealer }

// Score is a rule-agnostic scoring result.
type Score struct {
	Patterns []string // human-readable yaku / 番型 names
	Fan      int      // total fan / 番 (rule-specific aggregation)
	BasePts  int      // base points (rule-specific; what the loser(s) pay × seat factor)
}

// ---------------------------------------------------------------------------
// RuleHooks — optional per-variant lifecycle callbacks
// ---------------------------------------------------------------------------

// RuleHooks lets a variant (e.g. Riichi) inject rule-specific logic
// into the game loop WITHOUT the game loop knowing what variant is
// running.
//
// The game loop calls hooks at well-defined points (setup, draw,
// discard, call, round-end). The hooks own ALL variant-specific state
// (riichi flags, ippatsu window, furiten sets, dora indicators, dead
// wall) via State.RuleState, so game.State stays variant-agnostic.
//
// Sichuan returns nil from Hooks() — the game loop skips all hook
// calls and uses its own default logic.
//
// A forward-declared import cycle is avoided because RuleHooks lives
// in the rules package (which game already imports) and accepts
// opaque *game.State via an interface or a concrete pointer.
// However, since rules can't import game (cycle), hooks receive an
// opaque StateAccessor interface instead of *game.State directly.
// See StateAccessor below.
type RuleHooks interface {
	// OnRoundSetup is called after the wall is dealt, before any
	// player action. Riichi: carve dead wall, flip dora indicator,
	// set round wind, init furiten sets.
	OnRoundSetup(st StateAccessor)

	// BuildWinContext is the SINGLE source of truth for WinContext
	// construction. Called by the game loop every time it needs a
	// context (tsumo validation, tsumo scoring, ron checks, ron
	// scoring). Eliminates the 3+ diverging WinContext builders that
	// caused most of the Riichi bugs.
	BuildWinContext(st StateAccessor, seat int, winTile tile.Tile, tsumo bool, from int) WinContext

	// ValidateAction checks a player's DrawAction before the loop
	// processes it. Return non-nil to reject (the loop will
	// downgrade to a tsumogiri discard).
	// Riichi: enforce post-riichi tsumogiri, validate riichi declaration.
	ValidateAction(st StateAccessor, seat int, action DrawAction) error

	// AvailableCalls returns the call opportunities for a discard.
	// Returning nil tells the loop to use its own default logic
	// (pon/kan/chi based on hand contents).
	// Riichi: filters out furiten rons, blocks chi/pon for riichi'd players.
	AvailableCalls(st StateAccessor, discard tile.Tile, from int) []Call

	// AfterDiscard is called after a tile is placed in the river.
	// Riichi: update furiten set, manage ippatsu countdown.
	AfterDiscard(st StateAccessor, seat int, t tile.Tile)

	// AfterCall is called after a pon/chi/kan is applied.
	// Riichi: invalidate ippatsu for affected players.
	AfterCall(st StateAccessor, kind CallKind, seat, from int)

	// OnRoundEnd is called when the round finishes (win or exhaustion).
	// Cleanup hook.
	OnRoundEnd(st StateAccessor)

	// GetRiichiPot returns the current riichi pot value (for settlement).
	// Returns 0 for rules without riichi.
	GetRiichiPot() int

	// ConsumeRiichiPot zeroes the pot and returns the old value (for
	// awarding to winner).
	ConsumeRiichiPot() int

	// IsRiichi reports whether a seat has declared riichi this round.
	// Used by PlayerView to populate IsRiichi field.
	IsRiichi(seat int) bool
}

// DrawAction mirrors game.DrawAction to break the import cycle
// (rules can't import game). The game loop converts between the two.
type DrawAction struct {
	Kind          int // 0=Discard, 1=Tsumo, 2=ConcealedKan, 3=AddedKan
	Discard       tile.Tile
	KanTile       tile.Tile
	DeclareRiichi bool
}

// CallKind mirrors game.CallKind.
type CallKind int

const (
	CallPass CallKind = iota
	CallChi
	CallPon
	CallKan
	CallRon
)

// Call mirrors game.Call for the hooks interface.
type Call struct {
	Kind    CallKind
	Player  int
	Tile    tile.Tile
	Support []tile.Tile
}

// StateAccessor is a narrow read/write interface into game.State that
// the hooks package can use without importing internal/game (which
// would create a cycle). The game loop implements this on *State.
// StateAccessor is a narrow read/write interface into game.State.
// Method names use Get* prefix to avoid colliding with the exported
// field names on game.State (Go forbids a struct from having both a
// field and a method with the same name).
type StateAccessor interface {
	// Reads
	GetNumPlayers() int
	GetDealer() int
	GetCurrent() int
	GetTurnsTaken() int
	GetWallRemaining() int
	GetPlayerConcealed(seat int) [tile.NumKinds]int
	GetPlayerMelds(seat int) []tile.Meld
	GetPlayerScore(seat int) int
	GetPlayerDingque(seat int) tile.Suit
	GetPlayerJustDrew(seat int) *tile.Tile
	GetPlayerHasWon(seat int) bool
	GetPlayerName(seat int) string
	GetDiscards(seat int) []tile.Tile
	GetAllowsChi() bool
	GetAfterKan() bool
	GetLastTile() bool

	// Writes
	SetPlayerScore(seat int, score int)
	SetAfterKan(v bool)
	SetRuleState(v any)
	GetRuleState() any

	// Wall operations (for dead wall carving, kan-replacement draw)
	DrawFromWallBack(n int) []tile.Tile // draw from END for dead wall
	DrawFromWall() (tile.Tile, bool)    // normal draw from front
}
