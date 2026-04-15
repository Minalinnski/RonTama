package game

import (
	"fmt"

	"github.com/Minalinnski/RonTama/internal/rules"
	"github.com/Minalinnski/RonTama/internal/tile"
)

// PlayerState holds the per-seat mutable game state.
type PlayerState struct {
	Hand     tile.Hand
	Dingque  tile.Suit // SuitWind sentinel = "not yet chosen"
	HasWon   bool      // blood-battle: true after first win in this round
	Score    int       // running cumulative score for the round
	JustDrew *tile.Tile
}

// State is the full rounds-state visible to the dealer/server.
//
// All fields are exported for ease of inspection in tests; callers
// outside this package should mutate only via methods.
type State struct {
	Rule    rules.RuleSet
	Wall    *tile.Wall
	Players [NumPlayers]*PlayerState
	Dealer  int // seat 0..3
	Current int // whose turn it is (for the "draws next" pointer)

	// Per-seat discard piles ("river").
	Discards [NumPlayers][]tile.Tile

	// Counters
	TurnsTaken int

	// Flags propagated to WinContext.
	AfterKan bool // next draw is the kan-replacement
	LastTile bool // this draw was the last from the wall

	// Bookkeeping for kan-grab (抢杠胡): the current player just declared
	// an added kan with this tile, allowing other players one window to ron.
	GrabbableKanTile *tile.Tile

	// skipNextDraw is set after a pon: the next iteration of the loop
	// should NOT draw from the wall — the caller has already absorbed
	// the discarded tile via the meld and must discard immediately.
	// (Kan from discard does draw a replacement tile so it does not set this.)
	skipNextDraw bool
}

// NewState initializes a fresh round.
func NewState(rule rules.RuleSet, dealer int) (*State, error) {
	w, err := tile.NewWall(rule.TileKinds(), rule.CopiesPerTile())
	if err != nil {
		return nil, fmt.Errorf("game: build wall: %w", err)
	}
	st := &State{
		Rule:    rule,
		Wall:    w,
		Dealer:  dealer,
		Current: dealer,
	}
	for i := 0; i < NumPlayers; i++ {
		st.Players[i] = &PlayerState{Dingque: tile.SuitWind} // "unset" sentinel
	}
	// Deal HandSize tiles to each player.
	hand := rule.HandSize()
	for p := 0; p < NumPlayers; p++ {
		ts := w.DrawN(hand)
		if len(ts) != hand {
			return nil, fmt.Errorf("game: wall too small (%d) to deal %d", len(w.Tiles), NumPlayers*hand)
		}
		st.Players[p].Hand = tile.NewHand(ts)
	}
	return st, nil
}

// LiveSeats returns the seats that haven't won yet (blood-battle: still in).
func (s *State) LiveSeats() []int {
	out := make([]int, 0, NumPlayers)
	for i := 0; i < NumPlayers; i++ {
		if !s.Players[i].HasWon {
			out = append(out, i)
		}
	}
	return out
}

// NextLiveSeat returns the next seat after `from` (mod NumPlayers) that
// hasn't won. Returns -1 if no live seat exists (round over).
func (s *State) NextLiveSeat(from int) int {
	for step := 1; step <= NumPlayers; step++ {
		i := (from + step) % NumPlayers
		if !s.Players[i].HasWon {
			return i
		}
	}
	return -1
}

// Done reports whether the round is over: <=1 live seats OR wall empty.
func (s *State) Done() bool {
	live := 0
	for _, p := range s.Players {
		if !p.HasWon {
			live++
		}
	}
	return live <= 1 || s.Wall.Remaining() == 0
}

// View constructs the per-seat snapshot.
func (s *State) View(seat int) PlayerView {
	v := PlayerView{
		Rule:       s.Rule,
		Seat:       seat,
		Dealer:     s.Dealer,
		WallLeft:   s.Wall.Remaining(),
		OwnHand:    s.Players[seat].Hand,
		JustDrew:   s.Players[seat].JustDrew,
		TurnsTaken: s.TurnsTaken,
	}
	for i := 0; i < NumPlayers; i++ {
		v.Dingque[i] = s.Players[i].Dingque
		v.HasWon[i] = s.Players[i].HasWon
		v.Discards[i] = s.Discards[i]
		v.Melds[i] = s.Players[i].Hand.Melds
		v.Scores[i] = s.Players[i].Score
	}
	return v
}

// AvailableCallsOnDiscard enumerates the calls each non-discarding seat
// could legally make against the given discard.
//
// Order matters for resolution: Ron > Kan > Pon, and Ron from a closer
// seat in the discard's wake takes priority over a farther one. The
// caller is responsible for resolving conflicts.
func (s *State) AvailableCallsOnDiscard(discard tile.Tile, from int) []Call {
	var out []Call
	for seat := 0; seat < NumPlayers; seat++ {
		if seat == from || s.Players[seat].HasWon {
			continue
		}
		hand := s.Players[seat].Hand
		// Ron check
		if s.canRonDiscard(seat, discard, from) {
			out = append(out, Call{Kind: CallRon, Player: seat, Tile: discard})
		}
		// Kan: need 3 copies in hand to call open kan on the discard.
		if hand.Concealed[discard] >= 3 {
			out = append(out, Call{
				Kind:    CallKan,
				Player:  seat,
				Tile:    discard,
				Support: []tile.Tile{discard, discard, discard},
			})
		}
		// Pon: need 2 copies.
		if hand.Concealed[discard] >= 2 {
			out = append(out, Call{
				Kind:    CallPon,
				Player:  seat,
				Tile:    discard,
				Support: []tile.Tile{discard, discard},
			})
		}
	}
	return out
}

// canRonDiscard returns whether `seat` can declare ron on the given discard.
// Validates against the rule's CanWin including dingque.
func (s *State) canRonDiscard(seat int, discard tile.Tile, from int) bool {
	p := s.Players[seat]
	if p.Hand.Concealed[discard] >= 0 {
		ctx := rules.WinContext{
			WinningTile: discard,
			Tsumo:       false,
			From:        from,
			Seat:        seat,
			Dealer:      s.Dealer,
			Dingque:     p.Dingque,
			LastTile:    s.LastTile,
		}
		// Try without modifying hand
		hand := p.Hand
		return s.Rule.CanWin(hand, discard, ctx)
	}
	return false
}
