package tile

import "testing"

func TestNewWall_Sichuan(t *testing.T) {
	w, err := NewWall(SuitedKinds(), 4)
	if err != nil {
		t.Fatal(err)
	}
	if len(w.Tiles) != 108 {
		t.Errorf("Sichuan wall has %d tiles, want 108", len(w.Tiles))
	}
	counts := map[Tile]int{}
	for _, tile := range w.Tiles {
		counts[tile]++
		if tile.IsHonor() {
			t.Errorf("Sichuan wall contains honor %s", tile)
		}
	}
	for _, k := range SuitedKinds() {
		if counts[k] != 4 {
			t.Errorf("Sichuan wall has %d %s, want 4", counts[k], k)
		}
	}
}

func TestNewWall_Riichi(t *testing.T) {
	w, err := NewWall(AllKinds(), 4)
	if err != nil {
		t.Fatal(err)
	}
	if len(w.Tiles) != 136 {
		t.Errorf("Riichi wall has %d tiles, want 136", len(w.Tiles))
	}
	counts := map[Tile]int{}
	for _, tile := range w.Tiles {
		counts[tile]++
	}
	for i := 0; i < NumKinds; i++ {
		if counts[Tile(i)] != 4 {
			t.Errorf("wall has %d %s, want 4", counts[Tile(i)], Tile(i))
		}
	}
}

func TestNewWall_ShuffleNonTrivial(t *testing.T) {
	// Not a probabilistic test — just check the wall isn't sorted.
	w, err := NewWall(AllKinds(), 4)
	if err != nil {
		t.Fatal(err)
	}
	sortedCount := 0
	for i := 1; i < len(w.Tiles); i++ {
		if w.Tiles[i] >= w.Tiles[i-1] {
			sortedCount++
		}
	}
	if sortedCount == len(w.Tiles)-1 {
		t.Errorf("wall appears to be fully sorted; shuffle didn't run")
	}
}

func TestWall_Draw(t *testing.T) {
	w, err := NewWall(SuitedKinds(), 4)
	if err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 108; i++ {
		if w.Remaining() != 108-i {
			t.Errorf("Remaining()=%d at draw %d, want %d", w.Remaining(), i, 108-i)
		}
		_, ok := w.Draw()
		if !ok {
			t.Fatalf("Draw at %d failed", i)
		}
	}
	if _, ok := w.Draw(); ok {
		t.Error("Draw on empty wall should fail")
	}
	if w.Remaining() != 0 {
		t.Errorf("Remaining() after exhaust = %d", w.Remaining())
	}
}

func TestRandIntN_Distribution(t *testing.T) {
	// Loose smoke test: across many calls, all values appear.
	const n = 4
	const iters = 4000
	hits := [n]int{}
	for i := 0; i < iters; i++ {
		v, err := randIntN(n)
		if err != nil {
			t.Fatal(err)
		}
		if v < 0 || v >= n {
			t.Fatalf("randIntN(%d) = %d, out of range", n, v)
		}
		hits[v]++
	}
	for i, h := range hits {
		// Each bucket should see ~1000 hits; allow generous slack.
		if h < 800 || h > 1200 {
			t.Errorf("bucket %d hit count = %d, expected ~1000 (±200)", i, h)
		}
	}
}

func TestHand_AddRemove(t *testing.T) {
	h := NewHand(MustParseHand("123m"))
	if h.ConcealedCount() != 3 {
		t.Errorf("ConcealedCount = %d", h.ConcealedCount())
	}
	h.Add(Man4)
	if h.Concealed[Man4] != 1 {
		t.Errorf("after Add: count = %d", h.Concealed[Man4])
	}
	if !h.Remove(Man4) {
		t.Error("Remove(Man4) returned false")
	}
	if h.Remove(Man9) {
		t.Error("Remove(Man9) should fail")
	}
}

func TestHand_String(t *testing.T) {
	h := NewHand(MustParseHand("123m 5p"))
	got := h.String()
	want := "1m 2m 3m 5p"
	if got != want {
		t.Errorf("Hand.String() = %q, want %q", got, want)
	}
}
