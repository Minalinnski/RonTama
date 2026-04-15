package tui

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/Minalinnski/RonTama/internal/game"
	"github.com/Minalinnski/RonTama/internal/tile"
)

// HumanPlayer is a game.Player adapter that delegates decisions to the
// running Bubble Tea program by sending HumanPromptMsg messages and
// blocking on a response channel.
//
// The TUI's Update handler must drain pendingPrompt.Respond when the
// user makes a choice; otherwise the game goroutine deadlocks.
type HumanPlayer struct {
	N    string
	Prog *tea.Program
}

// NewHumanPlayer wires a HumanPlayer to a Bubble Tea program.
func NewHumanPlayer(n string, p *tea.Program) *HumanPlayer {
	return &HumanPlayer{N: n, Prog: p}
}

// Name implements game.Player.
func (h *HumanPlayer) Name() string {
	if h.N == "" {
		return "you"
	}
	return h.N
}

// ChooseDingque implements game.Player by prompting the UI.
func (h *HumanPlayer) ChooseDingque(view game.PlayerView) tile.Suit {
	resp := make(chan any, 1)
	h.Prog.Send(HumanPromptMsg{Kind: "dingque", View: view, Respond: resp})
	v := <-resp
	suit, ok := v.(tile.Suit)
	if !ok {
		// safe fallback
		return tile.SuitMan
	}
	return suit
}

// OnDraw implements game.Player by prompting the UI.
func (h *HumanPlayer) OnDraw(view game.PlayerView) game.DrawAction {
	resp := make(chan any, 1)
	h.Prog.Send(HumanPromptMsg{Kind: "draw", View: view, Respond: resp})
	v := <-resp
	act, ok := v.(game.DrawAction)
	if !ok {
		// fallback: discard the first concealed tile
		for i := 0; i < tile.NumKinds; i++ {
			if view.OwnHand.Concealed[i] > 0 {
				return game.DrawAction{Kind: game.DrawDiscard, Discard: tile.Tile(i)}
			}
		}
		// shouldn't reach
		panic(fmt.Sprintf("HumanPlayer.OnDraw: empty hand at seat %d", view.Seat))
	}
	return act
}

// OnCallOpportunity implements game.Player by prompting the UI.
func (h *HumanPlayer) OnCallOpportunity(view game.PlayerView, discarded tile.Tile, from int, opps []game.Call) game.Call {
	resp := make(chan any, 1)
	h.Prog.Send(HumanPromptMsg{Kind: "call", View: view, Discarded: discarded, Calls: opps, Respond: resp})
	v := <-resp
	c, ok := v.(game.Call)
	if !ok {
		return game.Pass
	}
	return c
}

// TUIObserver pushes EventMsg notifications to a Bubble Tea program.
type TUIObserver struct {
	Prog *tea.Program
}

// NewTUIObserver constructs a game.Observer that forwards events to the TUI.
func NewTUIObserver(p *tea.Program) *TUIObserver { return &TUIObserver{Prog: p} }

func (o *TUIObserver) OnRoundStart(s *game.State) {
	o.Prog.Send(EventMsg{State: s, Note: "Round start"})
}

func (o *TUIObserver) OnDingque(s *game.State, seat int, suit tile.Suit) {
	o.Prog.Send(EventMsg{State: s, Note: fmt.Sprintf("%s 缺 %s", seatLabel(seat), renderSuit(suit))})
}

func (o *TUIObserver) OnDraw(s *game.State, seat int, t tile.Tile) {
	if seat == HumanSeat {
		o.Prog.Send(EventMsg{State: s, Note: fmt.Sprintf("you draw %s", t)})
		return
	}
	o.Prog.Send(EventMsg{State: s, Note: fmt.Sprintf("%s draws", seatLabel(seat))})
}

func (o *TUIObserver) OnDiscard(s *game.State, seat int, t tile.Tile) {
	o.Prog.Send(EventMsg{State: s, Note: fmt.Sprintf("%s discards %s", seatLabel(seat), t)})
}

func (o *TUIObserver) OnCall(s *game.State, kind game.CallKind, seat, from int, t tile.Tile) {
	o.Prog.Send(EventMsg{State: s, Note: fmt.Sprintf("%s %s on %s (from %s)",
		seatLabel(seat), kind, t, seatLabel(from))})
}

func (o *TUIObserver) OnWin(s *game.State, w game.WinEvent) {
	o.Prog.Send(EventMsg{State: s, Note: formatWinNote(s, w)})
}

// formatWinNote builds a multi-line, prominent win announcement.
func formatWinNote(s *game.State, w game.WinEvent) string {
	how := "TSUMO"
	if !w.Tsumo {
		how = fmt.Sprintf("RON from %s", seatLabel(w.From))
	}
	hand := s.Players[w.Seat].Hand
	handStr := hand.String()
	if handStr == "" {
		handStr = "(empty)"
	}
	patterns := "(no yaku?)"
	if len(w.Score.Patterns) > 0 {
		patterns = strings.Join(w.Score.Patterns, " · ")
	}
	return fmt.Sprintf("🎉 %s WINS — %s on %s   [%s]  %d han, base %d\n   Hand: %s + %s",
		seatLabel(w.Seat), how, w.Tile, patterns, w.Score.Fan, w.Score.BasePts, handStr, w.Tile)
}

func (o *TUIObserver) OnRoundEnd(s *game.State, r *game.RoundResult) {
	o.Prog.Send(RoundDoneMsg{Result: r})
}
