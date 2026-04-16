package tui

import (
	"fmt"
	"strings"
	"time"

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

// localDeadline returns a soft deadline for local prompts (not
// enforced — the game won't auto-pass — but the TUI shows the ⏳
// countdown so the player knows time is passing).
func localDeadline() time.Time { return time.Now().Add(60 * time.Second) }

// ChooseExchange3 prompts the UI for 3 tiles of one suit.
func (h *HumanPlayer) ChooseExchange3(view game.PlayerView) [3]tile.Tile {
	resp := make(chan any, 1)
	h.Prog.Send(HumanPromptMsg{Kind: "exchange3", View: view, Respond: resp, Deadline: localDeadline()})
	v := <-resp
	picks, ok := v.([3]tile.Tile)
	if !ok {
		// fallback: use the easy strategy (suit with fewest tiles)
		return fallbackExchange3(view)
	}
	return picks
}

// fallbackExchange3 mirrors the ai.PickExchange3 strategy without the
// import (avoids tui→ai cycle).
func fallbackExchange3(view game.PlayerView) [3]tile.Tile {
	c := view.OwnHand.Concealed
	counts := [3]int{}
	for s := 0; s < 3; s++ {
		for n := 0; n < 9; n++ {
			counts[s] += c[s*9+n]
		}
	}
	bestSuit := 0
	bestCount := counts[0]
	for s := 1; s < 3; s++ {
		if counts[s] >= 3 && (counts[s] < bestCount || bestCount < 3) {
			bestSuit = s
			bestCount = counts[s]
		}
	}
	var picks [3]tile.Tile
	n := 0
	for i := bestSuit * 9; i < (bestSuit+1)*9 && n < 3; i++ {
		for j := 0; j < c[i] && n < 3; j++ {
			picks[n] = tile.Tile(i)
			n++
		}
	}
	return picks
}

// ChooseDingque implements game.Player by prompting the UI.
func (h *HumanPlayer) ChooseDingque(view game.PlayerView) tile.Suit {
	resp := make(chan any, 1)
	h.Prog.Send(HumanPromptMsg{Kind: "dingque", View: view, Respond: resp, Deadline: localDeadline()})
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
	h.Prog.Send(HumanPromptMsg{Kind: "draw", View: view, Respond: resp, Deadline: localDeadline()})
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
	h.Prog.Send(HumanPromptMsg{Kind: "call", View: view, Discarded: discarded, Calls: opps, Respond: resp, Deadline: localDeadline()})
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

func (o *TUIObserver) OnExchange3(s *game.State, picks [game.NumPlayers][3]tile.Tile, direction int) {
	dir := []string{"", "→ next", "↔ across", "← prev"}[direction]
	o.Prog.Send(EventMsg{State: s, Note: "换三张 " + dir})
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
