package client

import (
	"fmt"
	"sync"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/Minalinnski/RonTama/internal/game"
	"github.com/Minalinnski/RonTama/internal/net/proto"
	"github.com/Minalinnski/RonTama/internal/rules"
	"github.com/Minalinnski/RonTama/internal/tile"
	"github.com/Minalinnski/RonTama/internal/tui"
)

// deadlineFrom translates a server-supplied TimeoutSec into a wall-clock
// deadline relative to "now" (received time). Returns the zero Time when
// no timeout is in effect.
func deadlineFrom(timeoutSec int) time.Time {
	if timeoutSec <= 0 {
		return time.Time{}
	}
	return time.Now().Add(time.Duration(timeoutSec) * time.Second)
}

// TUIDecider implements client.Decider by forwarding prompts to a
// running Bubble Tea program (the same PlayModel used for local play).
//
// Limitation: PlayModel hard-codes HumanSeat = 0 for layout, so this
// decider only renders correctly when the server assigns seat 0 to
// this client (which it does for the first joiner). Multi-human seats
// would need a configurable HumanSeat in PlayModel — deferred.
type TUIDecider struct {
	Prog *tea.Program
	Rule rules.RuleSet

	mu       sync.Mutex
	seat     int
	dealer   int
	wallLeft int
	turn     int
}

// NewTUIDecider returns a decider that drives the given Bubble Tea program.
func NewTUIDecider(prog *tea.Program, rule rules.RuleSet) *TUIDecider {
	return &TUIDecider{Prog: prog, Rule: rule}
}

// AssignSeat implements Decider. Updates the global HumanSeat so the
// TUI rotates the cross layout to put this seat at the bottom.
func (d *TUIDecider) AssignSeat(seat int, _ string) {
	d.mu.Lock()
	d.seat = seat
	d.mu.Unlock()
	tui.SetHumanSeat(seat)
	d.Prog.Send(tui.EventMsg{Note: fmt.Sprintf("Assigned seat %d (%s)", seat, seatLabel(seat))})
}

func seatLabel(seat int) string {
	return []string{"East", "South", "West", "North"}[seat%4]
}

// HandleStateUpdate implements Decider — synthesise a *game.State from
// the wire snapshot and ship it to the TUI as an EventMsg.
func (d *TUIDecider) HandleStateUpdate(upd proto.StateUpdate) {
	d.mu.Lock()
	d.dealer = upd.Dealer
	d.wallLeft = upd.WallLeft
	d.turn = upd.Turn
	seat := d.seat
	d.mu.Unlock()

	st := synthesiseState(upd, d.Rule, seat)
	d.Prog.Send(tui.EventMsg{State: st, Note: upd.Note})
}

// AnswerExchange3 blocks on the TUI for an exchange-three pick.
func (d *TUIDecider) AnswerExchange3(req proto.AskExchange3) [3]tile.Tile {
	resp := make(chan any, 1)
	view := game.PlayerView{
		Rule:    d.Rule,
		Seat:    d.seat,
		OwnHand: tile.Hand{Concealed: req.OwnHand},
	}
	d.Prog.Send(tui.HumanPromptMsg{
		Kind: "exchange3", View: view, Respond: resp,
		Deadline: deadlineFrom(req.TimeoutSec),
	})
	v := <-resp
	if picks, ok := v.([3]tile.Tile); ok {
		return picks
	}
	return [3]tile.Tile{}
}

// AnswerDingque blocks on the TUI for a suit choice.
func (d *TUIDecider) AnswerDingque(req proto.AskDingque) tile.Suit {
	resp := make(chan any, 1)
	view := game.PlayerView{
		Rule:    d.Rule,
		Seat:    d.seat,
		OwnHand: tile.Hand{Concealed: req.OwnHand},
	}
	d.Prog.Send(tui.HumanPromptMsg{
		Kind: "dingque", View: view, Respond: resp,
		Deadline: deadlineFrom(req.TimeoutSec),
	})
	v := <-resp
	if s, ok := v.(tile.Suit); ok {
		return s
	}
	return tile.SuitMan
}

// AnswerDraw blocks on the TUI for a draw action.
func (d *TUIDecider) AnswerDraw(req proto.AskDraw) game.DrawAction {
	resp := make(chan any, 1)
	jd := req.JustDrew
	view := game.PlayerView{
		Rule:     d.Rule,
		Seat:     d.seat,
		Dealer:   d.dealer,
		WallLeft: d.wallLeft,
		OwnHand:  tile.Hand{Concealed: req.OwnHand},
		JustDrew: &jd,
	}
	view.Dingque[d.seat] = req.Dingque
	d.Prog.Send(tui.HumanPromptMsg{
		Kind: "draw", View: view, Respond: resp,
		Deadline: deadlineFrom(req.TimeoutSec),
	})
	v := <-resp
	if a, ok := v.(game.DrawAction); ok {
		return a
	}
	return fallbackDiscard(req.OwnHand)
}

// AnswerCall blocks on the TUI for a call decision.
func (d *TUIDecider) AnswerCall(req proto.AskCall) game.Call {
	resp := make(chan any, 1)
	view := game.PlayerView{
		Rule: d.Rule,
		Seat: d.seat,
	}
	d.Prog.Send(tui.HumanPromptMsg{
		Kind: "call", View: view, Discarded: req.Discarded, Calls: req.Calls, Respond: resp,
		Deadline: deadlineFrom(req.TimeoutSec),
	})
	v := <-resp
	if c, ok := v.(game.Call); ok {
		return c
	}
	return game.Pass
}

// HandleRoundEnd informs the TUI the round is done.
func (d *TUIDecider) HandleRoundEnd(end proto.RoundEnd) {
	d.Prog.Send(tui.RoundDoneMsg{Result: end.Result})
}

// HandleError surfaces server errors as a log line.
func (d *TUIDecider) HandleError(msg proto.ErrorMsg) {
	d.Prog.Send(tui.EventMsg{Note: "server error: " + msg.Message})
}

// synthesiseState constructs a fake *game.State from a wire snapshot.
// Hidden info (other players' tiles, the wall) is filled with placeholder
// values that match the visible counts (e.g. each opponent's Concealed
// holds HandSize copies of Man1 — they're never rendered, just counted).
func synthesiseState(upd proto.StateUpdate, rule rules.RuleSet, mySeat int) *game.State {
	st := &game.State{
		Rule:       rule,
		Wall:       &tile.Wall{Tiles: make([]tile.Tile, upd.WallLeft)},
		Dealer:     upd.Dealer,
		TurnsTaken: upd.Turn,
	}
	for i := 0; i < game.NumPlayers; i++ {
		seatPub := upd.Seats[i]
		ps := &game.PlayerState{
			Hand: tile.Hand{
				Concealed: [tile.NumKinds]int{},
				Melds:     append([]tile.Meld{}, seatPub.Melds...),
			},
			Dingque: seatPub.Dingque,
			HasWon:  seatPub.HasWon,
			Score:   seatPub.Score,
			Name:    seatPub.Name,
		}
		if i == mySeat {
			ps.Hand.Concealed = upd.OwnHand
			ps.Hand.Melds = upd.OwnMelds
			ps.JustDrew = upd.JustDrew
		} else if seatPub.HandSize > 0 {
			// Phantom tiles to make ConcealedCount() report the right size.
			ps.Hand.Concealed[0] = seatPub.HandSize
		}
		st.Players[i] = ps
		st.Discards[i] = append([]tile.Tile{}, seatPub.Discards...)
	}
	return st
}

// fallbackDiscard mirrors the server-side fallback when the TUI returns
// an unexpected response type.
func fallbackDiscard(c [tile.NumKinds]int) game.DrawAction {
	for i := 0; i < tile.NumKinds; i++ {
		if c[i] > 0 {
			return game.DrawAction{Kind: game.DrawDiscard, Discard: tile.Tile(i)}
		}
	}
	return game.DrawAction{Kind: game.DrawDiscard}
}
