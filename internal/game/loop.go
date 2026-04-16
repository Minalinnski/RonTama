package game

import (
	cryptoRand "crypto/rand"
	"fmt"
	"log/slog"
	"sort"

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
	return RunRoundOpts(RoundOpts{Rule: rule, Players: players, Dealer: dealer, Log: log, Observer: obs})
}

// RoundOpts configures a round run. Used by the Match wrapper to seed
// cumulative scores / honba / carried riichi pot before the round starts.
type RoundOpts struct {
	Rule     rules.RuleSet
	Players  [NumPlayers]Player
	Dealer   int
	Log      *slog.Logger
	Observer Observer

	// Optional starting state. If nil, seats use rule.StartingScore().
	InitialScores *[NumPlayers]int
	// Carried riichi sticks from a previous exhaustive draw.
	CarryRiichiPot int
	// Honba (本場) counter — displayed to observers; bonus effect on
	// payouts is a Phase-TODO for proper Riichi scoring.
	Honba int
	// RoundWind for Riichi (東/南/西/北 based on round index). Hooks
	// read this to set the round wind in WinContext + yakuhai detection.
	// Zero = default to East.
	RoundWind tile.Tile
}

// RunRoundOpts is the flexible entrypoint used by both single-round
// callers (via RunRoundWithObserver) and the Match wrapper.
func RunRoundOpts(opts RoundOpts) (*RoundResult, error) {
	rule := opts.Rule
	players := opts.Players
	dealer := opts.Dealer
	log := opts.Log
	obs := opts.Observer
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
	if opts.InitialScores != nil {
		for i := 0; i < NumPlayers; i++ {
			st.Players[i].Score = opts.InitialScores[i]
		}
	}
	st.RiichiPot = opts.CarryRiichiPot
	// Seed seat names from the player implementations so observers
	// (TUI panels, network StateUpdates) can show who's who.
	for i := 0; i < NumPlayers; i++ {
		if players[i] != nil {
			st.Players[i].Name = players[i].Name()
		}
	}
	// ---- RuleHooks integration ----
	hooks := rule.Hooks()
	if hooks != nil {
		// Store the round wind in RuleState so hooks can read it during
		// OnRoundSetup. The hooks will overwrite RuleState with their own
		// state struct (but will read round wind first).
		if opts.RoundWind != 0 {
			st.RuleState = opts.RoundWind // temporary; hooks.OnRoundSetup reads + replaces
		}
		hooks.OnRoundSetup(st)
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
		// Notify hooks that this player is drawing (clears temp furiten).
		if hooks != nil {
			hooks.OnPlayerDraw(st, seat)
		}
		var drawn tile.Tile
		if st.skipNextDraw {
			st.skipNextDraw = false
			st.Players[seat].JustDrew = nil
		} else if st.AfterKan {
			// Kan-replacement draw (嶺上牌): draw from the END of the
			// wall (closest to the dead wall). This is the tile the kan
			// player uses; if they win on it, it's 嶺上開花.
			d, ok := st.Wall.DrawFromBack()
			if !ok {
				result.Exhaustion = true
				break
			}
			drawn = d
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

		// Hooks validation: pure check (no side effects), then apply if ok.
		if hooks != nil {
			rAction := toRulesDrawAction(action)
			if err := hooks.CheckAction(st, seat, rAction); err != nil {
				log.Warn("hooks rejected action — falling back to discard", "seat", seat, "err", err)
				action = DrawAction{Kind: DrawDiscard, Discard: drawn}
			} else {
				hooks.ApplyAction(st, seat, rAction)
			}
		}

		// If tsumo was declared, validate via CanWin before committing.
		if action.Kind == DrawTsumo {
			if st.Players[seat].JustDrew == nil {
				log.Warn("invalid tsumo (no draw) — falling back to discard", "seat", seat)
				action = DrawAction{Kind: DrawDiscard, Discard: drawn}
			} else {
				ctx := buildCtx(hooks, st, seat, drawn, true, -1)
				st.Players[seat].Hand.Remove(drawn)
				if !rule.CanWin(st.Players[seat].Hand, drawn, ctx) {
					st.Players[seat].Hand.Add(drawn)
					log.Warn("invalid tsumo (CanWin=false) — falling back to discard", "seat", seat)
					action = DrawAction{Kind: DrawDiscard, Discard: drawn}
				} else {
					st.Players[seat].Hand.Add(drawn)
				}
			}
		}

		switch action.Kind {
		case DrawTsumo:
			ctx := buildCtx(hooks, st, seat, drawn, true, -1)
			st.Players[seat].Hand.Remove(drawn)
			score := rule.ScoreWin(st.Players[seat].Hand, drawn, ctx)
			applySettlementWithHooks(hooks, st, seat, ctx, score)
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
			// Riichi declaration: hooks.ValidateAction already applied the
			// side effects (debit 1000, set riichi flag, open ippatsu) if
			// hooks are active. For hookless (Sichuan), riichi is never
			// declared so this block is a no-op.
			if action.DeclareRiichi && hooks == nil {
				// Legacy path (should never trigger for Sichuan, but defensive).
				if err := validateRiichiDeclaration(st, seat, discard); err != nil {
					return nil, fmt.Errorf("seat %d invalid riichi: %w", seat, err)
				}
				st.Players[seat].Score -= 1000
				st.RiichiPot += 1000
				st.Riichi[seat] = true
				st.IppatsuValid[seat] = true
			}
			if action.DeclareRiichi {
				log.Info("riichi", "seat", seat, "discard", discard)
			}
			st.Players[seat].Hand.Remove(discard)
			st.Players[seat].JustDrew = nil
			st.Discards[seat] = append(st.Discards[seat], discard)
			st.AfterKan = false
			// Hooks-driven post-discard bookkeeping (furiten, ippatsu).
			if hooks != nil {
				hooks.AfterDiscard(st, seat, discard)
			} else {
				// Legacy ippatsu (Sichuan: always no-op since Riichi is false).
				if !action.DeclareRiichi && st.Riichi[seat] {
					st.IppatsuValid[seat] = false
				}
			}
			obs.OnDiscard(st, seat, discard)
			log.Debug("discard", "seat", seat, "tile", discard)

			// Solicit calls from other live seats. Hooks filter (furiten,
			// riichi-no-call). Pon/kan are exclusive — first taker wins.
			calls := availableCalls(hooks, st, discard, seat)
			if len(calls) > 0 {
				next := resolveCalls(hooks, st, players, calls, discard, seat, log, obs)
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

		case DrawConcealedKan:
			// Concealed kan (暗杠): player has 4 of a kind in hand.
			kt := action.KanTile
			if st.Players[seat].Hand.Concealed[kt] < 4 {
				return nil, fmt.Errorf("seat %d tried concealed kan on %s but has < 4", seat, kt)
			}
			for i := 0; i < 4; i++ {
				st.Players[seat].Hand.Remove(kt)
			}
			st.Players[seat].Hand.Melds = append(st.Players[seat].Hand.Melds, tile.Meld{
				Kind: tile.ConcealedKan, Tiles: []tile.Tile{kt, kt, kt, kt}, From: -1,
			})
			obs.OnCall(st, CallKan, seat, seat, kt)
			log.Debug("concealed-kan", "seat", seat, "tile", kt)
			if hooks != nil {
				hooks.AfterCall(st, toRulesCallKind(CallKan), seat, seat, kt, nil)
			}
			st.AfterKan = true
			// Loop back to same seat's draw phase (the next iteration will
			// draw from the back because AfterKan=true).
			st.Current = seat
			continue

		case DrawAddedKan:
			// Added kan (加杠): promote an existing pon to a kan.
			kt := action.KanTile
			if st.Players[seat].Hand.Concealed[kt] < 1 {
				return nil, fmt.Errorf("seat %d tried added kan on %s but has 0", seat, kt)
			}
			promoted := false
			for mi, m := range st.Players[seat].Hand.Melds {
				if m.Kind == tile.Pon && len(m.Tiles) >= 3 && m.Tiles[0] == kt {
					st.Players[seat].Hand.Remove(kt)
					st.Players[seat].Hand.Melds[mi].Kind = tile.AddedKan
					st.Players[seat].Hand.Melds[mi].Tiles = append(st.Players[seat].Hand.Melds[mi].Tiles, kt)
					promoted = true
					break
				}
			}
			if !promoted {
				return nil, fmt.Errorf("seat %d tried added kan on %s but no matching pon", seat, kt)
			}
			obs.OnCall(st, CallKan, seat, seat, kt)
			log.Debug("added-kan", "seat", seat, "tile", kt)
			if hooks != nil {
				hooks.AfterCall(st, toRulesCallKind(CallKan), seat, seat, kt, nil)
			}
			// TODO: other players get a kan-grab (抢杠胡) window here.
			// For now skip straight to replacement draw.
			st.AfterKan = true
			st.Current = seat
			continue
		}
	}

	for i := 0; i < NumPlayers; i++ {
		result.FinalScores[i] = st.Players[i].Score
	}
	if hooks != nil {
		hooks.OnRoundEnd(st)
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
func resolveCalls(hooks rules.RuleHooks, st *State, players [NumPlayers]Player, calls []Call, discard tile.Tile, from int, log *slog.Logger, obs Observer) callResolution {
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
			for _, c := range opts {
				if c.Kind == CallRon {
					declaredRons = append(declaredRons, c)
					break
				}
			}
		}
	}

	// Temporary furiten: any player who was offered ron but DIDN'T take
	// it enters temp-furiten (can't ron anything until their next draw).
	if hooks != nil {
		for player, opts := range byPlayer {
			hadRonOption := false
			for _, c := range opts {
				if c.Kind == CallRon {
					hadRonOption = true
					break
				}
			}
			if hadRonOption && choices[player].Kind != CallRon {
				hooks.OnRonPassed(st, player)
			}
		}
	}

	if len(declaredRons) > 0 {
		_ = choices
		out := callResolution{endRound: false}
		for _, r := range declaredRons {
			ctx := buildCtx(hooks, st, r.Player, discard, false, from)
			score := st.Rule.ScoreWin(st.Players[r.Player].Hand, discard, ctx)
			applySettlementWithHooks(hooks, st, r.Player, ctx, score)
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

	// No ron — pick by priority Kan > Pon > Chi. Across players: a
	// higher-priority call from any player beats a lower-priority call
	// from any player. Within the same priority, take the first choice.
	var pickedCall *Call
	priorities := []CallKind{CallKan, CallPon, CallChi}
	for _, want := range priorities {
		for player, choice := range choices {
			if choice.Kind != want {
				continue
			}
			for _, c := range byPlayer[player] {
				// Match BOTH kind and (for chi) the chosen support tiles.
				if c.Kind != want {
					continue
				}
				if want == CallChi {
					if !sameSupport(c.Support, choice.Support) {
						continue
					}
				}
				tmp := c
				pickedCall = &tmp
				break
			}
			if pickedCall != nil {
				break
			}
		}
		if pickedCall != nil {
			break
		}
	}
	if pickedCall != nil {
		c := *pickedCall
		applyCall(st, c, discard, from)
		obs.OnCall(st, c.Kind, c.Player, from, discard)
		log.Debug("call", "kind", c.Kind, "seat", c.Player, "tile", discard)
		if hooks != nil {
			hooks.AfterCall(st, toRulesCallKind(c.Kind), c.Player, from, discard, c.Support)
		} else {
			for s := 0; s < NumPlayers; s++ {
				st.IppatsuValid[s] = false
			}
		}
		switch c.Kind {
		case CallPon, CallChi:
			st.skipNextDraw = true
		case CallKan:
			st.AfterKan = true
		}
		return callResolution{nextSeat: c.Player}
	}
	return callResolution{nextSeat: -1}
}

// sameSupport returns true if both tile slices contain the same
// multiset of tiles (used to match a player's chi choice against the
// available chi options).
func sameSupport(a, b []tile.Tile) bool {
	if len(a) != len(b) {
		return false
	}
	var ca, cb [tile.NumKinds]int
	for _, t := range a {
		ca[t]++
	}
	for _, t := range b {
		cb[t]++
	}
	return ca == cb
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
	// Sort chi tiles so the meld displays as 3-4-5 not 4-3-5 (cosmetic).
	if c.Kind == CallChi {
		sort.Slice(tiles, func(i, j int) bool { return tiles[i] < tiles[j] })
	}
	var kind tile.MeldKind
	switch c.Kind {
	case CallChi:
		kind = tile.Chi
	case CallPon:
		kind = tile.Pon
	case CallKan:
		kind = tile.Kan
	default:
		kind = tile.Pon
	}
	hand.Melds = append(hand.Melds, tile.Meld{
		Kind: kind, Tiles: tiles, From: from,
	})
}

// applySettlement is the legacy (non-hooks) settlement path.
// Used only when hooks == nil (Sichuan).
func applySettlement(st *State, winner int, ctx rules.WinContext, score rules.Score) {
	hasWon := [NumPlayers]bool{}
	for i := 0; i < NumPlayers; i++ {
		hasWon[i] = st.Players[i].HasWon
	}
	deltas := st.Rule.Settle(st.Dealer, winner, ctx, score, hasWon, 0)
	for i := 0; i < NumPlayers; i++ {
		st.Players[i].Score += deltas[i]
	}
	if st.RiichiPot > 0 {
		st.Players[winner].Score += st.RiichiPot
		st.RiichiPot = 0
	}
}
