// Package game implements the rules-agnostic mahjong state machine
// and the synchronous game loop. Rule-specific behavior is delegated
// to a rules.RuleSet; player decisions are delegated to a Player
// interface so bots and (later) network clients are interchangeable.
package game

import (
	"fmt"

	"github.com/Minalinnski/RonTama/internal/rules"
	"github.com/Minalinnski/RonTama/internal/tile"
)

// NumPlayers is fixed at 4.
const NumPlayers = 4

// CallKind classifies an opportunity to call on someone's discard,
// or an in-turn declaration.
type CallKind uint8

const (
	CallPass CallKind = iota
	CallChi           // 吃 (Riichi only; next player only)
	CallPon           // 碰
	CallKan           // 杠 (open kan from discard)
	CallRon           // 胡 (winning on discard)
)

func (k CallKind) String() string {
	switch k {
	case CallPass:
		return "Pass"
	case CallChi:
		return "Chi"
	case CallPon:
		return "Pon"
	case CallKan:
		return "Kan"
	case CallRon:
		return "Ron"
	default:
		return fmt.Sprintf("CallKind(%d)", k)
	}
}

// Call is a chosen action on someone else's discard.
type Call struct {
	Kind   CallKind
	Player int       // seat that will perform the action (0..3)
	Tile   tile.Tile // the discard being acted on
	// For Pon/Kan: the supporting tiles from the player's hand.
	Support []tile.Tile
}

// Pass is the no-op call (used as a default return).
var Pass = Call{Kind: CallPass}

// DrawActionKind is what a player wants to do after drawing a tile.
type DrawActionKind uint8

const (
	DrawDiscard      DrawActionKind = iota // discard one tile
	DrawTsumo                              // declare tsumo win
	DrawConcealedKan                       // declare 暗杠
	DrawAddedKan                           // declare 加杠 (add to existing pon)
)

// DrawAction is the player's response to having just drawn a tile.
type DrawAction struct {
	Kind    DrawActionKind
	Discard tile.Tile // when Kind == DrawDiscard
	KanTile tile.Tile // when Kind == DrawConcealedKan or DrawAddedKan

	// DeclareRiichi (Riichi rule only) — when true and Kind ==
	// DrawDiscard, the player simultaneously declares riichi: pays a
	// 1000-point stick into the pot, locks the hand for the rest of
	// the round, and gains the 立直 yaku on any subsequent win.
	DeclareRiichi bool
}

// PlayerView is the per-player visible information passed to bots /
// clients. Hidden info (other hands, the wall) is not included.
type PlayerView struct {
	Rule       rules.RuleSet
	Seat       int
	Dealer     int
	WallLeft   int
	OwnHand    tile.Hand
	JustDrew   *tile.Tile            // last tile drawn (nil if not currently this player's draw)
	Dingque    [NumPlayers]tile.Suit // chosen dingque per seat (Sichuan); SuitWind = "not yet"
	HasWon     [NumPlayers]bool      // who has already won this round (blood battle)
	Riichi     [NumPlayers]bool      // declared riichi this round (Riichi rule)
	Discards   [NumPlayers][]tile.Tile
	Melds      [NumPlayers][]tile.Meld
	Scores     [NumPlayers]int
	Names      [NumPlayers]string // display names per seat
	Round      int
	TurnsTaken int
}

