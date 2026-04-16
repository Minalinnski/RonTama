package game

import (
	"github.com/Minalinnski/RonTama/internal/rules"
	"github.com/Minalinnski/RonTama/internal/tile"
)

// buildCtx creates a WinContext using hooks (if non-nil) or a plain
// default for Sichuan (no hooks). This eliminates the 3+ diverging
// inline WinContext builders that were the root cause of most Riichi bugs.
func buildCtx(hooks rules.RuleHooks, st *State, seat int, winTile tile.Tile, tsumo bool, from int) rules.WinContext {
	if hooks != nil {
		return hooks.BuildWinContext(st, seat, winTile, tsumo, from)
	}
	return rules.WinContext{
		WinningTile: winTile,
		Tsumo:       tsumo,
		From:        from,
		Seat:        seat,
		Dealer:      st.Dealer,
		Dingque:     st.Players[seat].Dingque,
		LastTile:    st.Wall.Remaining() == 0,
		AfterKan:    st.AfterKan,
	}
}

// availableCalls returns the call opportunities for a discard, using
// hooks if available (Riichi: includes furiten filtering, riichi-no-call),
// otherwise falling back to st.AvailableCallsOnDiscard (Sichuan default).
func availableCalls(hooks rules.RuleHooks, st *State, discard tile.Tile, from int) []Call {
	if hooks != nil {
		rCalls := hooks.AvailableCalls(st, discard, from)
		if rCalls != nil {
			return fromRulesCalls(rCalls)
		}
	}
	return st.AvailableCallsOnDiscard(discard, from)
}

// applySettlementWithHooks calls rule.Settle + awards riichi pot from
// hooks (or from st.RiichiPot for Sichuan).
func applySettlementWithHooks(hooks rules.RuleHooks, st *State, winner int, ctx rules.WinContext, score rules.Score) {
	hasWon := [NumPlayers]bool{}
	for i := 0; i < NumPlayers; i++ {
		hasWon[i] = st.Players[i].HasWon
	}
	// TODO: get honba from hooks or state. For now pass 0 (callers using
	// applySettlementWithHooks should pass honba if they have it).
	deltas := st.Rule.Settle(st.Dealer, winner, ctx, score, hasWon, 0)
	for i := 0; i < NumPlayers; i++ {
		st.Players[i].Score += deltas[i]
	}
	// Riichi pot to winner.
	if hooks != nil {
		pot := hooks.ConsumeRiichiPot()
		st.Players[winner].Score += pot
	} else if st.RiichiPot > 0 {
		st.Players[winner].Score += st.RiichiPot
		st.RiichiPot = 0
	}
}

// toRulesDrawAction converts game.DrawAction → rules.DrawAction.
func toRulesDrawAction(a DrawAction) rules.DrawAction {
	return rules.DrawAction{
		Kind:          int(a.Kind),
		Discard:       a.Discard,
		KanTile:       a.KanTile,
		DeclareRiichi: a.DeclareRiichi,
	}
}

// fromRulesCalls converts []rules.Call → []game.Call.
func fromRulesCalls(rCalls []rules.Call) []Call {
	out := make([]Call, len(rCalls))
	for i, rc := range rCalls {
		out[i] = Call{
			Kind:    CallKind(rc.Kind),
			Player:  rc.Player,
			Tile:    rc.Tile,
			Support: rc.Support,
		}
	}
	return out
}

// toRulesCallKind converts game.CallKind → rules.CallKind.
func toRulesCallKind(k CallKind) rules.CallKind {
	return rules.CallKind(k)
}
