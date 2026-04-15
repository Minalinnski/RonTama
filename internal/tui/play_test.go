package tui

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/Minalinnski/RonTama/internal/game"
	"github.com/Minalinnski/RonTama/internal/rules/sichuan"
	"github.com/Minalinnski/RonTama/internal/tile"
)

// freshState returns a usable State for view-rendering tests.
func freshState(t *testing.T) *game.State {
	t.Helper()
	st, err := game.NewState(sichuan.New(), 0)
	if err != nil {
		t.Fatal(err)
	}
	st.Players[0].Dingque = tile.SuitSou
	return st
}

func TestPlayModel_LoadingViewBeforeState(t *testing.T) {
	m := NewPlayModel(sichuan.New())
	if !strings.Contains(m.View(), "Loading") {
		t.Errorf("expected 'Loading' in initial view, got %q", m.View())
	}
}

func TestPlayModel_HandlesEventMsg(t *testing.T) {
	m := NewPlayModel(sichuan.New())
	st := freshState(t)
	updated, _ := m.Update(EventMsg{State: st, Note: "hello world"})
	pm := updated.(PlayModel)
	if pm.state == nil {
		t.Fatal("state not set after EventMsg")
	}
	if len(pm.log) == 0 || pm.log[len(pm.log)-1] != "hello world" {
		t.Errorf("log not updated: %v", pm.log)
	}
}

func TestPlayModel_HandlesPromptAndKey(t *testing.T) {
	m := NewPlayModel(sichuan.New())
	st := freshState(t)
	m, _ = applyMsg(m, EventMsg{State: st})

	resp := make(chan any, 1)
	view := st.View(0)
	m, _ = applyMsg(m, HumanPromptMsg{Kind: "dingque", View: view, Respond: resp})
	pm := m
	if pm.prompt == nil {
		t.Fatal("prompt not stored")
	}

	// User picks 'm' (man suit).
	_, _ = applyMsg(pm, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'m'}})
	select {
	case v := <-resp:
		if v != tile.SuitMan {
			t.Errorf("dingque response = %v, want SuitMan", v)
		}
	default:
		t.Error("no response sent on 'm' key")
	}
}

func TestPlayModel_DiscardKey(t *testing.T) {
	m := NewPlayModel(sichuan.New())
	st := freshState(t)
	m, _ = applyMsg(m, EventMsg{State: st})

	resp := make(chan any, 1)
	view := st.View(0)
	m, _ = applyMsg(m, HumanPromptMsg{Kind: "draw", View: view, Respond: resp})

	// Press space to discard the currently-selected tile.
	_, _ = applyMsg(m, tea.KeyMsg{Type: tea.KeySpace})
	select {
	case v := <-resp:
		act, ok := v.(game.DrawAction)
		if !ok {
			t.Fatalf("expected DrawAction, got %T", v)
		}
		if act.Kind != game.DrawDiscard {
			t.Errorf("expected DrawDiscard, got %v", act.Kind)
		}
	default:
		t.Error("no discard response")
	}
}

func TestPlayModel_QuitOnQ(t *testing.T) {
	m := NewPlayModel(sichuan.New())
	_, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'q'}})
	if cmd == nil {
		t.Fatal("expected non-nil cmd on q")
	}
	msg := cmd()
	if _, ok := msg.(tea.QuitMsg); !ok {
		t.Errorf("expected QuitMsg, got %T", msg)
	}
}

// applyMsg is a small helper to call Update and re-cast the model.
func applyMsg(m PlayModel, msg tea.Msg) (PlayModel, tea.Cmd) {
	updated, cmd := m.Update(msg)
	return updated.(PlayModel), cmd
}
