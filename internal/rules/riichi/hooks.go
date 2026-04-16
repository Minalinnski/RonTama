package riichi

import (
	"fmt"

	"github.com/Minalinnski/RonTama/internal/rules"
	"github.com/Minalinnski/RonTama/internal/shanten"
	"github.com/Minalinnski/RonTama/internal/tile"
)

// Hooks implements rules.RuleHooks for Japanese Riichi mahjong.
//
// All Riichi-specific state (dora indicators, furiten tracking, ippatsu
// window, riichi declarations, dead wall) lives HERE, not in game.State.
// The game loop only sees these through the hooks interface, keeping
// the loop itself variant-agnostic.
type Hooks struct {
	rule *Rule // back-reference for CanWin / ScoreWin calls

	// Dead wall & dora.
	deadWall       []tile.Tile   // 14 tiles carved from the end of the wall
	doraIndicators []tile.Tile   // visible dora indicators (initially 1; +1 per kan)
	uraIndicators  []tile.Tile   // ura-dora (same count as dora; revealed on riichi win)
	kanCount       int           // kans declared this round (affects dora reveal count)

	// Riichi declaration.
	riichi    [4]bool // true once a player has declared riichi
	riichiPot int     // accumulated 1000-point sticks

	// Ippatsu tracking.
	// ippatsuAt[seat] = TurnsTaken at riichi declaration time, or -1 if
	// not active. Ippatsu is valid for 1 full rotation (4 opponent
	// discards) without any call interruption.
	ippatsuAt [4]int

	// Furiten: per-seat set of tiles the player has discarded. A player
	// in furiten cannot ron on any tile in their set.
	// More precisely: "own-discard furiten" — if tile X is in your river,
	// you can never ron X (even from a different player's discard).
	furiten [4]map[tile.Tile]bool

	// Round wind (場風). East for the first 4 hands, South for next 4, etc.
	roundWind tile.Tile
}

// NewHooks creates a new Hooks instance. Called once per round.
func NewHooks(rule *Rule, roundWind tile.Tile) *Hooks {
	if roundWind == 0 {
		roundWind = tile.East
	}
	h := &Hooks{
		rule:      rule,
		roundWind: roundWind,
	}
	for i := range h.ippatsuAt {
		h.ippatsuAt[i] = -1
	}
	for i := range h.furiten {
		h.furiten[i] = map[tile.Tile]bool{}
	}
	return h
}

// ---- rules.RuleHooks implementation ----

// OnRoundSetup carves the 14-tile dead wall, reveals the first dora
// indicator, and stores self in RuleState.
func (h *Hooks) OnRoundSetup(st rules.StateAccessor) {
	// Carve dead wall (14 tiles from the end of the shuffled wall).
	h.deadWall = st.DrawFromWallBack(14)
	// Dora indicator: 3rd tile in the dead wall (position [2] of the
	// 14-tile block in Tenhou convention). Ura-dora is position [3].
	if len(h.deadWall) >= 4 {
		h.doraIndicators = []tile.Tile{h.deadWall[2]}
		h.uraIndicators = []tile.Tile{h.deadWall[3]}
	}
	st.SetRuleState(h)
}

// BuildWinContext is the SINGLE source of truth for WinContext.
func (h *Hooks) BuildWinContext(st rules.StateAccessor, seat int, winTile tile.Tile, tsumo bool, from int) rules.WinContext {
	return rules.WinContext{
		WinningTile:       winTile,
		Tsumo:             tsumo,
		From:              from,
		Seat:              seat,
		Dealer:            st.GetDealer(),
		Dingque:           st.GetPlayerDingque(seat),
		LastTile:          st.GetLastTile(),
		AfterKan:          st.GetAfterKan(),
		KanGrab:           false, // TODO: proper kan-grab tracking
		RoundWind:         h.roundWind,
		Riichi:            h.riichi[seat],
		DoubleRiichi:      h.riichi[seat] && st.GetTurnsTaken() <= 4,
		Ippatsu:           h.isIppatsuValid(st, seat),
		DoraIndicators:    h.doraIndicators,
		UraDoraIndicators: h.uraIndicators,
	}
}

// ValidateAction checks a player's action before the loop processes it.
func (h *Hooks) ValidateAction(st rules.StateAccessor, seat int, action rules.DrawAction) error {
	// Post-riichi: must tsumogiri (discard exactly the drawn tile).
	if h.riichi[seat] && action.Kind == 0 { // 0 = Discard
		jd := st.GetPlayerJustDrew(seat)
		if jd != nil && action.Discard != *jd {
			return fmt.Errorf("riichi: must discard drawn tile (%s), not %s", *jd, action.Discard)
		}
	}
	// Riichi declaration validation.
	if action.DeclareRiichi {
		return h.validateRiichi(st, seat, action.Discard)
	}
	return nil
}

// AvailableCalls returns calls with furiten filtering + riichi restrictions.
// Returns nil to fall through to the default logic when no filtering is needed.
func (h *Hooks) AvailableCalls(st rules.StateAccessor, discard tile.Tile, from int) []rules.Call {
	// Build the default call set (same as game.State.AvailableCallsOnDiscard).
	defaults := h.buildDefaultCalls(st, discard, from)

	// Filter.
	var out []rules.Call
	for _, c := range defaults {
		// Furiten: can't ron a tile you've previously discarded.
		if c.Kind == rules.CallRon {
			if h.isFuriten(c.Player, discard) {
				continue
			}
		}
		// Riichi'd player can only ron (no chi/pon/kan).
		if h.riichi[c.Player] {
			if c.Kind == rules.CallChi || c.Kind == rules.CallPon || c.Kind == rules.CallKan {
				continue
			}
		}
		out = append(out, c)
	}
	return out
}

// AfterDiscard updates furiten sets and ippatsu countdown.
func (h *Hooks) AfterDiscard(st rules.StateAccessor, seat int, t tile.Tile) {
	// Own-discard furiten: add to the set.
	h.furiten[seat][t] = true

	// Ippatsu: if this is NOT the riichi-declaration discard itself
	// (which is the same turn), close the window on the next discard.
	if h.riichi[seat] && h.ippatsuAt[seat] >= 0 && h.ippatsuAt[seat] != st.GetTurnsTaken() {
		h.ippatsuAt[seat] = -1
	}
}

// AfterCall invalidates ippatsu for all riichi'd players (a call breaks
// the uninterrupted rotation that ippatsu requires).
func (h *Hooks) AfterCall(_ rules.StateAccessor, _ rules.CallKind, _, _ int) {
	for i := range h.ippatsuAt {
		h.ippatsuAt[i] = -1
	}
}

// OnRoundEnd is a cleanup hook.
func (h *Hooks) OnRoundEnd(_ rules.StateAccessor) {}

// GetRiichiPot returns the current pot.
func (h *Hooks) GetRiichiPot() int { return h.riichiPot }

// ConsumeRiichiPot zeroes the pot and returns the old value.
func (h *Hooks) ConsumeRiichiPot() int {
	old := h.riichiPot
	h.riichiPot = 0
	return old
}

// ---- Internal helpers ----

// validateRiichi checks riichi preconditions.
func (h *Hooks) validateRiichi(st rules.StateAccessor, seat int, discard tile.Tile) error {
	if h.riichi[seat] {
		return fmt.Errorf("already in riichi")
	}
	melds := st.GetPlayerMelds(seat)
	for _, m := range melds {
		if m.Kind != tile.ConcealedKan {
			return fmt.Errorf("hand is open (cannot riichi)")
		}
	}
	if st.GetPlayerScore(seat) < 1000 {
		return fmt.Errorf("insufficient score (%d < 1000)", st.GetPlayerScore(seat))
	}
	if st.GetWallRemaining() < 4 {
		return fmt.Errorf("wall too low (%d remaining)", st.GetWallRemaining())
	}
	concealed := st.GetPlayerConcealed(seat)
	if concealed[discard] == 0 {
		return fmt.Errorf("no %s in hand to discard", discard)
	}
	concealed[discard]--
	if shanten.Of(concealed, len(melds)) > 0 {
		return fmt.Errorf("hand not tenpai after discarding %s", discard)
	}
	return nil
}

// applyRiichi is called by the game loop (via the hooks) when a riichi
// declaration is validated. Debits 1000 from the player's score and
// records the declaration.
func (h *Hooks) applyRiichi(st rules.StateAccessor, seat int) {
	st.SetPlayerScore(seat, st.GetPlayerScore(seat)-1000)
	h.riichiPot += 1000
	h.riichi[seat] = true
	h.ippatsuAt[seat] = st.GetTurnsTaken()
}

// IsRiichi reports whether a seat has declared riichi.
func (h *Hooks) IsRiichi(seat int) bool { return h.riichi[seat] }

// isIppatsuValid checks the ippatsu window: valid if declared within
// the last 4 turns and no call has interrupted.
func (h *Hooks) isIppatsuValid(st rules.StateAccessor, seat int) bool {
	if !h.riichi[seat] || h.ippatsuAt[seat] < 0 {
		return false
	}
	return st.GetTurnsTaken()-h.ippatsuAt[seat] <= 4
}

// isFuriten reports whether `seat` is in own-discard furiten for `t`.
func (h *Hooks) isFuriten(seat int, t tile.Tile) bool {
	return h.furiten[seat][t]
}

// buildDefaultCalls enumerates the standard pon/kan/chi/ron calls for
// a discard. Mirrors game.State.AvailableCallsOnDiscard but operates
// through StateAccessor so it doesn't import internal/game.
func (h *Hooks) buildDefaultCalls(st rules.StateAccessor, discard tile.Tile, from int) []rules.Call {
	n := st.GetNumPlayers()
	nextSeat := (from + 1) % n
	allowChi := st.GetAllowsChi()
	var out []rules.Call

	for seat := 0; seat < n; seat++ {
		if seat == from || st.GetPlayerHasWon(seat) {
			continue
		}
		concealed := st.GetPlayerConcealed(seat)
		melds := st.GetPlayerMelds(seat)

		// Ron check via Rule.CanWin.
		ctx := h.BuildWinContext(st, seat, discard, false, from)
		hand := tile.Hand{Concealed: concealed, Melds: melds}
		if h.rule.CanWin(hand, discard, ctx) {
			out = append(out, rules.Call{Kind: rules.CallRon, Player: seat, Tile: discard})
		}

		// Kan: 3 copies in hand.
		if concealed[discard] >= 3 {
			out = append(out, rules.Call{
				Kind:    rules.CallKan,
				Player:  seat,
				Tile:    discard,
				Support: []tile.Tile{discard, discard, discard},
			})
		}

		// Pon: 2 copies.
		if concealed[discard] >= 2 {
			out = append(out, rules.Call{
				Kind:    rules.CallPon,
				Player:  seat,
				Tile:    discard,
				Support: []tile.Tile{discard, discard},
			})
		}

		// Chi: only next player, only suit tiles.
		if allowChi && seat == nextSeat && discard.IsSuit() {
			for _, pat := range chiPatterns(discard) {
				if concealed[pat[0]] > 0 && concealed[pat[1]] > 0 {
					out = append(out, rules.Call{
						Kind:    rules.CallChi,
						Player:  seat,
						Tile:    discard,
						Support: []tile.Tile{pat[0], pat[1]},
					})
				}
			}
		}
	}
	return out
}

// chiPatterns returns the 1-3 (support[0], support[1]) pairs that
// form a sequential run with the discard tile.
func chiPatterns(discard tile.Tile) [][2]tile.Tile {
	if !discard.IsSuit() {
		return nil
	}
	n := discard.Number()
	base := tile.Tile(int(discard) - (n - 1))
	var out [][2]tile.Tile
	if n >= 3 {
		out = append(out, [2]tile.Tile{base + tile.Tile(n-3), base + tile.Tile(n-2)})
	}
	if n >= 2 && n <= 8 {
		out = append(out, [2]tile.Tile{base + tile.Tile(n-2), base + tile.Tile(n)})
	}
	if n <= 7 {
		out = append(out, [2]tile.Tile{base + tile.Tile(n), base + tile.Tile(n+1)})
	}
	return out
}
