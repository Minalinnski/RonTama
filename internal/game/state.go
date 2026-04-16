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
	Name     string // display name (from Player.Name() or remote-supplied)
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

	// RuleState holds variant-specific opaque state, set by
	// hooks.OnRoundSetup(). For Riichi: *riichi.HooksState containing
	// dead wall, dora, furiten, ippatsu, riichi flags. For Sichuan: nil.
	RuleState any

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

	// Riichi tracking (Riichi rule only).
	//   Riichi[s]       — seat s has declared riichi this round
	//   IppatsuValid[s] — seat s could still win 一発 (riichi + win
	//                     within one go-around, no interrupting call)
	//   RiichiPot       — total 1000-point sticks accumulated this round
	Riichi       [NumPlayers]bool
	IppatsuValid [NumPlayers]bool
	RiichiPot    int
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
	startScore := rule.StartingScore()
	for i := 0; i < NumPlayers; i++ {
		st.Players[i] = &PlayerState{
			Dingque: tile.SuitWind, // "unset" sentinel
			Score:   startScore,
		}
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
	hooks := s.Rule.Hooks()
	for i := 0; i < NumPlayers; i++ {
		v.Dingque[i] = s.Players[i].Dingque
		v.HasWon[i] = s.Players[i].HasWon
		if hooks != nil {
			v.Riichi[i] = hooks.IsRiichi(i)
		} else {
			v.Riichi[i] = s.Riichi[i]
		}
		v.Discards[i] = s.Discards[i]
		v.Melds[i] = s.Players[i].Hand.Melds
		v.Scores[i] = s.Players[i].Score
		v.Names[i] = s.Players[i].Name
	}

	// Pre-compute action flags so TUI doesn't need to replicate rule logic.
	if hooks != nil {
		v.IsRiichi = hooks.IsRiichi(seat)
	} else {
		v.IsRiichi = s.Riichi[seat]
	}
	if v.JustDrew != nil {
		ctx := buildCtx(hooks, s, seat, *v.JustDrew, true, -1)
		probeHand := s.Players[seat].Hand
		probeHand.Concealed[*v.JustDrew]--
		v.CanTsumo = s.Rule.CanWin(probeHand, *v.JustDrew, ctx)
	}
	// CanRiichi: per-tile flag for every concealed tile + drawn tile.
	// Uses hooks.CheckAction (pure, no side effects) to test each tile.
	if hooks != nil && !v.IsRiichi {
		sorted := s.Players[seat].Hand.ConcealedTiles()
		v.CanRiichi = make([]bool, len(sorted))
		for i, t := range sorted {
			rda := rules.DrawAction{Kind: 0, Discard: t, DeclareRiichi: true}
			v.CanRiichi[i] = hooks.CheckAction(s, seat, rda) == nil
		}
	}
	return v
}

// AvailableCallsOnDiscard enumerates the calls each non-discarding seat
// could legally make against the given discard.
//
// Priority resolution (handled by resolveCalls): Ron > Kan > Pon > Chi.
// Chi is only available to the next-in-turn seat AND only when the
// rule allows it (Riichi yes, Sichuan no).
func (s *State) AvailableCallsOnDiscard(discard tile.Tile, from int) []Call {
	var out []Call
	nextSeat := (from + 1) % NumPlayers
	allowChi := s.Rule.AllowsChi()
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
		// Chi: only the next player in turn order, and only if the rule
		// allows it. Each pattern produces a distinct Call so the
		// player can pick which 2 hand-tiles to use.
		if allowChi && seat == nextSeat && discard.IsSuit() {
			for _, pat := range chiPatterns(discard) {
				if hand.Concealed[pat[0]] > 0 && hand.Concealed[pat[1]] > 0 {
					sup := []tile.Tile{pat[0], pat[1]}
					out = append(out, Call{
						Kind:    CallChi,
						Player:  seat,
						Tile:    discard,
						Support: sup,
					})
				}
			}
		}
	}
	return out
}

// chiPatterns returns the up-to-3 (support[0], support[1]) pairs that
// would form a 3-tile run with `discard`. Each pair is the two hand
// tiles needed; the discard fills in.
func chiPatterns(discard tile.Tile) [][2]tile.Tile {
	if !discard.IsSuit() {
		return nil
	}
	n := discard.Number() // 1..9
	base := tile.Tile(int(discard) - (n - 1)) // suit's "1" tile
	var out [][2]tile.Tile
	// (n-2, n-1) + n  — discard at the right edge
	if n >= 3 {
		out = append(out, [2]tile.Tile{base + tile.Tile(n-3), base + tile.Tile(n-2)})
	}
	// (n-1, n+1) — discard in the middle (kanchan-shaped chi)
	if n >= 2 && n <= 8 {
		out = append(out, [2]tile.Tile{base + tile.Tile(n-2), base + tile.Tile(n)})
	}
	// (n+1, n+2) + n — discard at the left edge
	if n <= 7 {
		out = append(out, [2]tile.Tile{base + tile.Tile(n), base + tile.Tile(n+1)})
	}
	return out
}

// canRonDiscard returns whether `seat` can declare ron on the given discard.
// Validates against the rule's CanWin including dingque, riichi, and
// all situational flags that affect yaku eligibility (a missing Riichi
// flag here meant that a riichi-only hand was never offered ron — bug).
func (s *State) canRonDiscard(seat int, discard tile.Tile, from int) bool {
	p := s.Players[seat]
	ctx := rules.WinContext{
		WinningTile: discard,
		Tsumo:       false,
		From:        from,
		Seat:        seat,
		Dealer:      s.Dealer,
		Dingque:     p.Dingque,
		LastTile:    s.LastTile,
		KanGrab:     s.GrabbableKanTile != nil && *s.GrabbableKanTile == discard,
		Riichi:      s.Riichi[seat],
		Ippatsu:     s.IppatsuValid[seat],
		RoundWind:   tile.East,
	}
	hand := p.Hand
	return s.Rule.CanWin(hand, discard, ctx)
}
