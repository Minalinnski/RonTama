package game_test

import (
	"testing"

	"github.com/Minalinnski/RonTama/internal/game"
	"github.com/Minalinnski/RonTama/internal/rules/riichi"
	"github.com/Minalinnski/RonTama/internal/rules/sichuan"
	"github.com/Minalinnski/RonTama/internal/tile"
)

func TestChi_NotOfferedInSichuan(t *testing.T) {
	st, err := game.NewState(sichuan.New(), 0)
	if err != nil {
		t.Fatal(err)
	}
	// Plant 4m + 6m in seat 1 (next after seat 0).
	st.Players[1].Hand.Concealed[tile.Man4] = 1
	st.Players[1].Hand.Concealed[tile.Man6] = 1
	calls := st.AvailableCallsOnDiscard(tile.Man5, 0)
	for _, c := range calls {
		if c.Kind == game.CallChi {
			t.Errorf("Sichuan should not offer chi, got %v", c)
		}
	}
}

func TestChi_OfferedToNextPlayerOnly(t *testing.T) {
	st, err := game.NewState(riichi.New(), 0)
	if err != nil {
		t.Fatal(err)
	}
	// Reset all hands to known empty.
	for i := 0; i < game.NumPlayers; i++ {
		st.Players[i].Hand.Concealed = [tile.NumKinds]int{}
	}
	// seat 1 (next after 0): 4m + 6m → can chi 5m as kanchan
	st.Players[1].Hand.Concealed[tile.Man4] = 1
	st.Players[1].Hand.Concealed[tile.Man6] = 1
	// seat 2: also has 4m + 6m but should NOT be offered chi (not the next seat)
	st.Players[2].Hand.Concealed[tile.Man4] = 1
	st.Players[2].Hand.Concealed[tile.Man6] = 1

	calls := st.AvailableCallsOnDiscard(tile.Man5, 0)
	chiCount := 0
	chiSeat := -1
	for _, c := range calls {
		if c.Kind == game.CallChi {
			chiCount++
			chiSeat = c.Player
		}
	}
	if chiCount != 1 {
		t.Errorf("expected 1 chi opportunity, got %d", chiCount)
	}
	if chiSeat != 1 {
		t.Errorf("chi offered to seat %d, want 1", chiSeat)
	}
}

func TestChi_AllThreePatterns(t *testing.T) {
	st, err := game.NewState(riichi.New(), 0)
	if err != nil {
		t.Fatal(err)
	}
	for i := 0; i < game.NumPlayers; i++ {
		st.Players[i].Hand.Concealed = [tile.NumKinds]int{}
	}
	// seat 1 holds 3m, 4m, 6m, 7m → on discarded 5m has 3 chi options:
	//   3m-4m + 5m  (right edge)
	//   4m-6m + 5m  (kanchan)
	//   6m-7m + 5m  (left edge)
	st.Players[1].Hand.Concealed[tile.Man3] = 1
	st.Players[1].Hand.Concealed[tile.Man4] = 1
	st.Players[1].Hand.Concealed[tile.Man6] = 1
	st.Players[1].Hand.Concealed[tile.Man7] = 1

	calls := st.AvailableCallsOnDiscard(tile.Man5, 0)
	chiCount := 0
	for _, c := range calls {
		if c.Kind == game.CallChi {
			chiCount++
		}
	}
	if chiCount != 3 {
		t.Errorf("expected 3 chi options for 5m with 3,4,6,7m in hand, got %d", chiCount)
	}
}

func TestChi_TerminalEdgeCases(t *testing.T) {
	st, err := game.NewState(riichi.New(), 0)
	if err != nil {
		t.Fatal(err)
	}
	for i := 0; i < game.NumPlayers; i++ {
		st.Players[i].Hand.Concealed = [tile.NumKinds]int{}
	}
	// seat 1 has 2m, 3m → on discard 1m only 1 pattern: 2-3 + 1.
	st.Players[1].Hand.Concealed[tile.Man2] = 1
	st.Players[1].Hand.Concealed[tile.Man3] = 1
	calls := st.AvailableCallsOnDiscard(tile.Man1, 0)
	chiCount := 0
	for _, c := range calls {
		if c.Kind == game.CallChi {
			chiCount++
		}
	}
	if chiCount != 1 {
		t.Errorf("1m + (2m,3m): expected 1 chi pattern, got %d", chiCount)
	}
}
