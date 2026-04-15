package game

import (
	cryptoRand "crypto/rand"
	"fmt"
	"log/slog"

	"github.com/Minalinnski/RonTama/internal/rules"
	"github.com/Minalinnski/RonTama/internal/shanten"
	"github.com/Minalinnski/RonTama/internal/tile"
)

// shantenAfter is a tiny shim used by validateRiichiDeclaration: pick
// the most permissive shanten (Riichi rule allows chiitoi/kokushi).
func shantenAfter(c [tile.NumKinds]int, melds int) int {
	return shanten.Of(c, melds)
}

// tileRandIntN: uniform [0, n). Modulo bias is negligible for the
// small n values used here (direction = 3).
func tileRandIntN(n int) (int, error) {
	if n <= 0 {
		return 0, fmt.Errorf("tileRandIntN: n must be positive")
	}
	var b [1]byte
	if _, err := cryptoRand.Read(b[:]); err != nil {
		return 0, err
	}
	return int(b[0]) % n, nil
}

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
	OnExchange3(s *State, picks [NumPlayers][3]tile.Tile, direction int)
	OnDingque(s *State, seat int, suit tile.Suit)
	OnDraw(s *State, seat int, t tile.Tile)
	OnDiscard(s *State, seat int, t tile.Tile)
	OnCall(s *State, kind CallKind, seat, from int, t tile.Tile)
	OnWin(s *State, w WinEvent)
	OnRoundEnd(s *State, r *RoundResult)
}

// NoopObserver implements Observer with empty methods.
type NoopObserver struct{}

func (NoopObserver) OnRoundStart(*State)                                       {}
func (NoopObserver) OnExchange3(*State, [NumPlayers][3]tile.Tile, int)         {}
func (NoopObserver) OnDingque(*State, int, tile.Suit)                          {}
func (NoopObserver) OnDraw(*State, int, tile.Tile)                             {}
func (NoopObserver) OnDiscard(*State, int, tile.Tile)                          {}
func (NoopObserver) OnCall(*State, CallKind, int, int, tile.Tile)              {}
func (NoopObserver) OnWin(*State, WinEvent)                                    {}
func (NoopObserver) OnRoundEnd(*State, *RoundResult)                           {}

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

	// Phase 1a: exchange-three (Sichuan 换三张). Each player picks 3
	// tiles of one suit; a randomly chosen direction (left/right/across)
	// shifts them. Direction is the same for all 4 seats this round.
	if rule.RequiresExchange3() {
		direction, err := pickExchangeDirection()
		if err != nil {
			return nil, err
		}
		var picks [NumPlayers][3]tile.Tile
		for seat := 0; seat < NumPlayers; seat++ {
			picks[seat] = players[seat].ChooseExchange3(st.View(seat))
			if err := validateExchange3(st, seat, picks[seat]); err != nil {
				return nil, fmt.Errorf("seat %d invalid exchange-3: %w", seat, err)
			}
			for _, t := range picks[seat] {
				st.Players[seat].Hand.Remove(t)
			}
		}
		// Distribute (each seat receives from seat - direction mod 4).
		for seat := 0; seat < NumPlayers; seat++ {
			source := (seat - direction + NumPlayers) % NumPlayers
			for _, t := range picks[source] {
				st.Players[seat].Hand.Add(t)
			}
		}
		obs.OnExchange3(st, picks, direction)
		log.Debug("exchange3 applied", "direction", direction)
	}

	// Phase 1b: dingque (Sichuan).
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
		var drawn tile.Tile
		if st.skipNextDraw {
			// Post-pon: the caller absorbed the discarded tile via the
			// meld, so don't pull from the wall. They must discard.
			st.skipNextDraw = false
			st.Players[seat].JustDrew = nil
		} else {
			d, ok := st.Wall.Draw()
			if !ok {
				result.Exhaustion = true
				break
			}
			drawn = d
			st.LastTile = st.Wall.Remaining() == 0
			st.Players[seat].Hand.Add(drawn)
			st.Players[seat].JustDrew = &drawn
			obs.OnDraw(st, seat, drawn)
			log.Debug("draw", "seat", seat, "tile", drawn, "wall_left", st.Wall.Remaining())
		}
		st.TurnsTaken++

		action := players[seat].OnDraw(st.View(seat))
		switch action.Kind {
		case DrawTsumo:
			if st.Players[seat].JustDrew == nil {
				return nil, fmt.Errorf("seat %d declared tsumo without a draw (post-call hands cannot tsumo without kan-replacement)", seat)
			}
			ctx := rules.WinContext{
				WinningTile: drawn,
				Tsumo:       true,
				From:        -1,
				Seat:        seat,
				Dealer:      st.Dealer,
				Dingque:     st.Players[seat].Dingque,
				LastTile:    st.LastTile,
				AfterKan:    st.AfterKan,
				Riichi:      st.Riichi[seat],
				Ippatsu:     st.IppatsuValid[seat],
				RoundWind:   tile.East,
			}
			// Validate: hand+drawn was already added, validate by removing
			// drawn before passing to CanWin (CanWin re-adds it).
			st.Players[seat].Hand.Remove(drawn)
			if !rule.CanWin(st.Players[seat].Hand, drawn, ctx) {
				st.Players[seat].Hand.Add(drawn)
				return nil, fmt.Errorf("seat %d declared invalid tsumo", seat)
			}
			score := rule.ScoreWin(st.Players[seat].Hand, drawn, ctx)
			applySettlement(st, seat, ctx, score)
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
			// Riichi declaration: validate before mutating state.
			if action.DeclareRiichi {
				if err := validateRiichiDeclaration(st, seat, discard); err != nil {
					return nil, fmt.Errorf("seat %d invalid riichi: %w", seat, err)
				}
				st.Players[seat].Score -= 1000
				st.RiichiPot += 1000
				st.Riichi[seat] = true
				st.IppatsuValid[seat] = true
				log.Info("riichi", "seat", seat, "discard", discard)
			}
			st.Players[seat].Hand.Remove(discard)
			st.Players[seat].JustDrew = nil
			st.Discards[seat] = append(st.Discards[seat], discard)
			st.AfterKan = false
			// Ippatsu invalidation: a player's own discard AFTER their
			// riichi declaration closes their ippatsu window. Pon/kan
			// invalidation happens in resolveCalls when the call is applied.
			if !action.DeclareRiichi && st.Riichi[seat] {
				st.IppatsuValid[seat] = false
			}
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
//
// Each eligible player is asked ONCE with all their applicable options
// (e.g. a player who could ron AND pon sees both choices and picks one).
// Ron always wins over pon/kan; multi-ron is allowed (一炮多响).
func resolveCalls(st *State, players [NumPlayers]Player, calls []Call, discard tile.Tile, from int, log *slog.Logger, obs Observer) callResolution {
	// Group calls by player.
	byPlayer := map[int][]Call{}
	for _, c := range calls {
		byPlayer[c.Player] = append(byPlayer[c.Player], c)
	}

	// First pass: ask every eligible player.
	choices := map[int]Call{}
	var declaredRons []Call
	for player, opts := range byPlayer {
		view := st.View(player)
		choice := players[player].OnCallOpportunity(view, discard, from, opts)
		choices[player] = choice
		if choice.Kind == CallRon {
			// Find the matching ron Call from opts (Player + Kind match).
			for _, c := range opts {
				if c.Kind == CallRon {
					declaredRons = append(declaredRons, c)
					break
				}
			}
		}
	}

	if len(declaredRons) > 0 {
		_ = choices // pon/kan choices ignored when ron wins
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
				Riichi:      st.Riichi[r.Player],
				Ippatsu:     st.IppatsuValid[r.Player],
				RoundWind:   tile.East,
			}
			score := st.Rule.ScoreWin(st.Players[r.Player].Hand, discard, ctx)
			applySettlement(st, r.Player, ctx, score)
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

	// No ron — apply the first kan/pon choice we received. Kan beats pon
	// when both are claimed by the same player (the player can pick either).
	// Across players: kan from any player beats pon from any player.
	var pickedCall *Call
	for player, choice := range choices {
		if choice.Kind == CallKan {
			for _, c := range byPlayer[player] {
				if c.Kind == CallKan {
					tmp := c
					pickedCall = &tmp
					break
				}
			}
			break
		}
	}
	if pickedCall == nil {
		for player, choice := range choices {
			if choice.Kind == CallPon {
				for _, c := range byPlayer[player] {
					if c.Kind == CallPon {
						tmp := c
						pickedCall = &tmp
						break
					}
				}
				break
			}
		}
	}
	if pickedCall != nil {
		c := *pickedCall
		applyCall(st, c, discard, from)
		obs.OnCall(st, c.Kind, c.Player, from, discard)
		log.Debug("call", "kind", c.Kind, "seat", c.Player, "tile", discard)
		// Any call invalidates ippatsu for every riichi'd player.
		for s := 0; s < NumPlayers; s++ {
			st.IppatsuValid[s] = false
		}
		switch c.Kind {
		case CallPon:
			st.skipNextDraw = true
		case CallKan:
			st.AfterKan = true
		}
		return callResolution{nextSeat: c.Player}
	}
	return callResolution{nextSeat: -1}
}

// pickExchangeDirection picks 1 (left), 2 (across), or 3 (right) at
// random using the wall's crypto/rand pathway via tile.NewWall... we
// just need a uniform 1..3.
func pickExchangeDirection() (int, error) {
	// 1, 2, or 3 — no zero, since 0 would be self.
	idx, err := tileRandIntN(3)
	if err != nil {
		return 0, fmt.Errorf("exchange direction rand: %w", err)
	}
	return idx + 1, nil
}

// validateExchange3 enforces: same suit, no honors, all 3 tiles in hand.
func validateExchange3(st *State, seat int, picks [3]tile.Tile) error {
	suit := picks[0].Suit()
	if suit != tile.SuitMan && suit != tile.SuitPin && suit != tile.SuitSou {
		return fmt.Errorf("exchange tile must be a suit tile")
	}
	for _, t := range picks {
		if t.Suit() != suit {
			return fmt.Errorf("exchange tiles must be same suit")
		}
	}
	// Count occurrences in picks vs hand.
	need := [tile.NumKinds]int{}
	for _, t := range picks {
		need[t]++
	}
	for i, n := range need {
		if n > st.Players[seat].Hand.Concealed[i] {
			return fmt.Errorf("not enough %s in hand for exchange", tile.Tile(i))
		}
	}
	return nil
}

// validateRiichiDeclaration checks that seat may declare riichi by
// discarding `discard` this turn:
//   - rule must allow it (Riichi only, indicated by !RequiresDingque)
//   - hand must be fully concealed (no open melds; ankans are OK)
//   - score >= 1000
//   - wall has at least 4 tiles left (opponents must have a chance to deal in)
//   - discarding `discard` leaves the hand at tenpai (shanten == 0)
//   - hasn't already declared riichi this round
func validateRiichiDeclaration(st *State, seat int, discard tile.Tile) error {
	rule := st.Rule
	if rule.RequiresDingque() {
		return fmt.Errorf("ruleset %q does not support riichi", rule.Name())
	}
	if st.Riichi[seat] {
		return fmt.Errorf("already in riichi")
	}
	p := st.Players[seat]
	for _, m := range p.Hand.Melds {
		if m.Kind != tile.ConcealedKan {
			return fmt.Errorf("hand is open (cannot riichi)")
		}
	}
	if p.Score < 1000 {
		return fmt.Errorf("insufficient score (%d < 1000)", p.Score)
	}
	if st.Wall.Remaining() < 4 {
		return fmt.Errorf("wall too low (%d remaining)", st.Wall.Remaining())
	}
	// Tenpai after this discard: simulate the discard and call shanten.
	probe := p.Hand.Concealed
	if probe[discard] == 0 {
		return fmt.Errorf("hand has no %s to discard", discard)
	}
	probe[discard]--
	if shantenAfter(probe, len(p.Hand.Melds)) > 0 {
		return fmt.Errorf("hand not tenpai after discarding %s", discard)
	}
	return nil
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

// applySettlement asks the rule for per-seat deltas and applies them,
// then pays the riichi pot to the winner (whoever takes the round
// first claims the entire accumulated pot).
func applySettlement(st *State, winner int, ctx rules.WinContext, score rules.Score) {
	hasWon := [NumPlayers]bool{}
	for i := 0; i < NumPlayers; i++ {
		hasWon[i] = st.Players[i].HasWon
	}
	deltas := st.Rule.Settle(st.Dealer, winner, ctx, score, hasWon)
	for i := 0; i < NumPlayers; i++ {
		st.Players[i].Score += deltas[i]
	}
	if st.RiichiPot > 0 {
		st.Players[winner].Score += st.RiichiPot
		st.RiichiPot = 0
	}
}
