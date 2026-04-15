package tui

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
)

func TestHelloModel_RendersTitle(t *testing.T) {
	m := helloModel{width: 80, height: 24}
	out := m.View()
	if !strings.Contains(out, "RonTama") {
		t.Fatalf("expected view to contain 'RonTama', got:\n%s", out)
	}
	if !strings.Contains(out, "Phase 0") {
		t.Fatalf("expected view to contain 'Phase 0', got:\n%s", out)
	}
}

func TestHelloModel_QuitsOnQ(t *testing.T) {
	m := helloModel{}
	_, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'q'}})
	if cmd == nil {
		t.Fatal("expected non-nil cmd from 'q' key, got nil")
	}
	msg := cmd()
	if _, ok := msg.(tea.QuitMsg); !ok {
		t.Fatalf("expected tea.QuitMsg from 'q' key, got %T", msg)
	}
}

func TestHelloModel_TracksWindowSize(t *testing.T) {
	m := helloModel{}
	updated, _ := m.Update(tea.WindowSizeMsg{Width: 120, Height: 40})
	hm := updated.(helloModel)
	if hm.width != 120 || hm.height != 40 {
		t.Fatalf("expected width=120 height=40, got width=%d height=%d", hm.width, hm.height)
	}
}
