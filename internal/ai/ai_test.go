package ai_test

import (
	"testing"

	"github.com/Minalinnski/RonTama/internal/ai"
)

func TestParseDifficulty(t *testing.T) {
	cases := map[string]ai.Difficulty{
		"easy":   ai.Easy,
		"medium": ai.Medium,
		"hard":   ai.Hard,
	}
	for s, want := range cases {
		got, err := ai.ParseDifficulty(s)
		if err != nil {
			t.Fatalf("ParseDifficulty(%q): %v", s, err)
		}
		if got != want {
			t.Errorf("ParseDifficulty(%q) = %d, want %d", s, got, want)
		}
	}
	if _, err := ai.ParseDifficulty("super"); err == nil {
		t.Error("expected error on unknown difficulty")
	}
}

func TestDifficulty_String(t *testing.T) {
	cases := map[ai.Difficulty]string{
		ai.Easy:   "easy",
		ai.Medium: "medium",
		ai.Hard:   "hard",
	}
	for d, want := range cases {
		if got := d.String(); got != want {
			t.Errorf("%d.String() = %q, want %q", d, got, want)
		}
	}
}
