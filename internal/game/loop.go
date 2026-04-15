package game

import (
	"fmt"
	"log/slog"

	"github.com/Minalinnski/RonTama/internal/rules"
	"github.com/Minalinnski/RonTama/internal/tile"
)

// RoundResult summarises one round's outcome.
type RoundResult struct {
	Wins        []WinEvent
	Exhaustion  bool // true if wall ran out before all wins were settled
	FinalScores [NumPlayers]int
}

// Observer receives notifications as the round progresses. All methods
// are optional; pass NoopObserver{} or nil to ignore. The TUI uses
// this to drive view updates without polling the State.
type Observer interface {
	OnRoundStart(s *State)
	OnDingque(s *State, seat int, suit tile.Suit)
	OnDraw(s *State, seat int, t tile.Tile)
	OnDiscard(s *State, seat int, t tile.Tile)
	OnCall(s *State, kind CallKind, seat, from int, t tile.Tile)
	OnWin(s *State, w WinEvent)
	OnRoundEnd(s *State, r *RoundResult)
}

// NoopObserver implements Observer with empty methods.
type NoopObserver struct{}

func (NoopObserver) OnRoundStart(*State)                                {}
func (NoopObserver) OnDingque(*State, int, tile.Suit)                   {}
func (NoopObserver) OnDraw(*State, int, tile.Tile)                      {}
func (NoopObserver) OnDiscard(*State, int, tile.Tile)                   {}
func (NoopObserver) OnCall(*State, CallKind, int, int, tile.Tile)       {}
func (NoopObserver) OnWin(*State, WinEvent)                             {}
func (NoopObserver) OnRoundEnd(*State, *RoundResult)                    {}

// WinEvent records one player's win during the round.
type WinEvent struct {
	Seat  int
	Tsumo bool
	From  int // seat that fed the winning tile (-1 for tsumo)
	Tile  tile.Tile
	Score rules.Score
}

// RunRound drives a single round to completion. The caller passes
// four players in seat order. Returns the round summary.
func RunRound(rule rules.RuleSet, players [NumPlayers]Player, dealer int, log *slog.Logger) (*RoundResult, error) {
	return RunRoundWithObserver(rule, players, dealer, log, NoopObserver{})
}

// RunRoundWithObserver is RunRound + an Observer that gets notified at
// every public state change. Used by the TUI to drive view updates.
func RunRoundWithObserver(rule rules.RuleSet, players [NumPlayers]Player, dealer int, log *slog.Logger, obs Observer) (*RoundResult, error) {
	if log == nil {
		log = slog.Default()
	}
	if obs == nil {
		obs = NoopObserver{}
	}
	st, err := NewState(rule, dealer)
	if err != nil {
		return nil, err
	}
	obs.OnRoundStart(st)

	// Phase 1: dingque (Sichuan).
	if rule.RequiresDingque() {
		for seat := 0; seat < NumPlayers; seat++ {
			ds := players[seat].ChooseDingque(st.View(seat))
			if ds == tile.SuitWind || ds == tile.SuitDragon {
				return nil, fmt.Errorf("seat %d returned invalid dingque suit %d", seat, ds)
			}
			st.Players[seat].Dingque = ds
			obs.OnDingque(st, seat, ds)
			log.Debug("dingque chosen", "seat", seat, "suit", ds)
		}
	}

	// Phase 2: dealer draws first then standard turn loop.
	result := &RoundResult{}
	for !st.Done() {
		// Draw for the current live seat.
		seat := st.Current
		if st.Players[seat].HasWon {
			seat = st.NextLiveSeat(seat - 1)
			if seat < 0 {
				break
			}
			st.Current = seat
		}
		drawn, ok := st.Wall.Draw()
		if !ok {
			result.Exhaustion = true
			break
		}
		st.LastTile = st.Wall.Remaining() == 0
		st.Players[seat].Hand.Add(drawn)
		st.Players[seat].JustDrew = &drawn
		st.TurnsTaken++
		obs.OnDraw(st, seat, drawn)
		log.Debug("draw", "seat", seat, "tile", drawn, "wall_left", st.Wall.Remaining())

		action := players[seat].OnDraw(st.View(seat))
		switch action.Kind {
		case DrawTsumo:
			ctx := rules.WinContext{
				WinningTile: drawn,
				Tsumo:       true,
				From:        -1,
				Seat:        seat,
				Dealer:      st.Dealer,
				Dingque:     st.Players[seat].Dingque,
				LastTile:    st.LastTile,
				AfterKan:    st.AfterKan,
			}
			// Validate: hand+drawn was already added, validate by removing
			// drawn before passing to CanWin (CanWin re-adds it).
			st.Players[seat].Hand.Remove(drawn)
			if !rule.CanWin(st.Players[seat].Hand, drawn, ctx) {
				st.Players[seat].Hand.Add(drawn)
				return nil, fmt.Errorf("seat %d declared invalid tsumo", seat)
			}
			score := rule.ScoreWin(st.Players[seat].Hand, drawn, ctx)
			settleTsumo(st, seat, score)
			win := WinEvent{Seat: seat, Tsumo: true, From: -1, Tile: drawn, Score: score}
			result.Wins = append(result.Wins, win)
			obs.OnWin(st, win)
			log.Info("tsumo", "seat", seat, "patterns", score.Patterns, "fan", score.Fan)
			st.Players[seat].HasWon = true
			st.AfterKan = false
			st.Current = st.NextLiveSeat(seat)
			if st.Current < 0 {
				break
			}
			continue

		case DrawDiscard:
			discard := action.Discard
			if st.Players[seat].Hand.Concealed[discard] == 0 {
				return nil, fmt.Errorf("seat %d tried to discard %s but hand has 0", seat, discard)
			}
			st.Players[seat].Hand.Remove(discard)
			st.Players[seat].JustDrew = nil
			st.Discards[seat] = append(st.Discards[seat], discard)
			st.AfterKan = false
			obs.OnDiscard(st, seat, discard)
			log.Debug("discard", "seat", seat, "tile", discard)

			// Solicit calls from other live seats. In Sichuan, multiple
			// players may ron the same discard ("一炮多响"). Pon/kan are
			// exclusive — first taker wins (priority order: ron > kan > pon).
			calls := st.AvailableCallsOnDiscard(discard, seat)
			if len(calls) > 0 {
				next := resolveCalls(st, players, calls, discard, seat, log, obs)
				if next.endRound {
					// All wins settled in resolveCalls; round may continue if not Done.
					st.Current = next.nextSeat
					if st.Current < 0 {
						break
					}
					// extract any wins
					for _, w := range next.wins {
						result.Wins = append(result.Wins, w)
					}
					continue
				}
				if next.nextSeat >= 0 {
					st.Current = next.nextSeat
					continue
				}
			}
			// No call: pass to next seat.
			st.Current = st.NextLiveSeat(seat)
			if st.Current < 0 {
				break
			}

		case DrawConcealedKan, DrawAddedKan:
			// Stub: skip kan declarations in the Phase 2 driver. Player
			// implementations should not return these yet. If they do
			// we treat as a discard of the kan tile.
			return nil, fmt.Errorf("seat %d tried to declare kan; not implemented in Phase 2", seat)
		}
	}

	for i := 0; i < NumPlayers; i++ {
		result.FinalScores[i] = st.Players[i].Score
	}
	obs.OnRoundEnd(st, result)
	return result, nil
}

// callResolution describes the outcome of resolving a discard's call window.
type callResolution struct {
	endRound bool
	nextSeat int
	wins     []WinEvent
}

// resolveCalls picks the highest-priority call(s) and applies them.
func resolveCalls(st *State, players [NumPlayers]Player, calls []Call, discard tile.Tile, from int, log *slog.Logger, obs Observer) callResolution {
	// Group by kind. Ron has highest priority (and may be claimed by multiple).
	var rons, kans, pons []Call
	for _, c := range calls {
		switch c.Kind {
		case CallRon:
			rons = append(rons, c)
		case CallKan:
			kans = append(kans, c)
		case CallPon:
			pons = append(pons, c)
		}
	}

	// Ask each ron-eligible player; collect those that opt in.
	var declaredRons []Call
	for _, r := range rons {
		view := st.View(r.Player)
		choice := players[r.Player].OnCallOpportunity(view, discard, from, []Call{r})
		if choice.Kind == CallRon {
			declaredRons = append(declaredRons, r)
		}
	}
	if len(declaredRons) > 0 {
		out := callResolution{endRound: false}
		for _, r := range declaredRons {
			ctx := rules.WinContext{
				WinningTile: discard,
				Tsumo:       false,
				From:        from,
				Seat:        r.Player,
				Dealer:      st.Dealer,
				Dingque:     st.Players[r.Player].Dingque,
				LastTile:    st.LastTile,
				KanGrab:     st.GrabbableKanTile != nil && *st.GrabbableKanTile == discard,
			}
			score := st.Rule.ScoreWin(st.Players[r.Player].Hand, discard, ctx)
			settleRon(st, r.Player, from, score)
			win := WinEvent{Seat: r.Player, Tsumo: false, From: from, Tile: discard, Score: score}
			out.wins = append(out.wins, win)
			st.Players[r.Player].HasWon = true
			obs.OnWin(st, win)
			log.Info("ron", "seat", r.Player, "from", from, "patterns", score.Patterns, "fan", score.Fan)
		}
		// next live seat after `from`
		out.nextSeat = st.NextLiveSeat(from)
		out.endRound = true
		return out
	}

	// No ron — try kan, then pon. Both consume the discard and shift turn.
	allCalls := append([]Call{}, kans...)
	allCalls = append(allCalls, pons...)
	for _, c := range allCalls {
		view := st.View(c.Player)
		choice := players[c.Player].OnCallOpportunity(view, discard, from, []Call{c})
		if choice.Kind == c.Kind {
			applyCall(st, c, discard, from)
			obs.OnCall(st, c.Kind, c.Player, from, discard)
			log.Debug("call", "kind", c.Kind, "seat", c.Player, "tile", discard)
			// caller must now act (they will OnDraw without drawing? Actually after
			// pon they must discard immediately. We model that by setting Current
			// without drawing. For simplicity: caller's hand now has 14 tiles
			// post-pon-via-meld-and-removed-supports? Actually pon doesn't add a
			// draw — the meld absorbs the discard. We need them to discard next.
			// So set Current = c.Player and let next iteration treat them as
			// already-drew-but-skip-draw.
			//
			// Phase 2 simplification: random-bot driver handles this in OnDraw
			// path, so we instead trigger a "mock draw" of nil. To avoid extra
			// branches, the loop will simply make c.Player draw next, which is
			// incorrect for kan/pon flow. We'll fix in a later phase.
			return callResolution{nextSeat: c.Player}
		}
	}
	return callResolution{nextSeat: -1}
}

// applyCall records a meld for the calling player and removes the supporting tiles.
func applyCall(st *State, c Call, discard tile.Tile, from int) {
	hand := &st.Players[c.Player].Hand
	for _, sup := range c.Support {
		hand.Remove(sup)
	}
	tiles := append([]tile.Tile{}, c.Support...)
	tiles = append(tiles, discard)
	kind := tile.Pon
	if c.Kind == CallKan {
		kind = tile.Kan
	}
	hand.Melds = append(hand.Melds, tile.Meld{
		Kind: kind, Tiles: tiles, From: from,
	})
}

// settleTsumo charges all live non-winners.
func settleTsumo(st *State, winner int, score rules.Score) {
	for i := 0; i < NumPlayers; i++ {
		if i == winner || st.Players[i].HasWon {
			continue
		}
		st.Players[i].Score -= score.BasePts
		st.Players[winner].Score += score.BasePts
	}
}

// settleRon charges only the discarder.
func settleRon(st *State, winner, loser int, score rules.Score) {
	pts := score.BasePts * 2 // ron pays double in Sichuan
	st.Players[loser].Score -= pts
	st.Players[winner].Score += pts
}
