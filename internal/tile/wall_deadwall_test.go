package tile

import "testing"

func TestSplitDeadWall(t *testing.T) {
	w, err := NewWall(AllKinds(), 4) // 136 tiles
	if err != nil {
		t.Fatal(err)
	}
	dead := w.SplitDeadWall(14)
	if len(dead) != 14 {
		t.Fatalf("dead wall len = %d, want 14", len(dead))
	}
	if w.Remaining() != 122 {
		t.Errorf("wall remaining = %d, want 122 (136-14)", w.Remaining())
	}
	for i, d := range dead {
		if d > Tile(NumKinds-1) {
			t.Errorf("dead[%d] = %d, out of range", i, d)
		}
	}
}

func TestSplitDeadWall_AfterDeal(t *testing.T) {
	// Simulate: deal 13×4=52 tiles first, then split dead wall.
	w, err := NewWall(AllKinds(), 4) // 136 tiles
	if err != nil {
		t.Fatal(err)
	}
	_ = w.DrawN(52) // deal
	dead := w.SplitDeadWall(14)
	if len(dead) != 14 {
		t.Fatalf("dead wall len = %d, want 14", len(dead))
	}
	// 136 - 52 dealt - 14 dead = 70 remaining
	if w.Remaining() != 70 {
		t.Errorf("remaining = %d, want 70", w.Remaining())
	}
}

func TestDrawFromBack(t *testing.T) {
	w, err := NewWall(SuitedKinds(), 4) // 108 tiles
	if err != nil {
		t.Fatal(err)
	}
	origLen := w.Remaining()
	back, ok := w.DrawFromBack()
	if !ok {
		t.Fatal("DrawFromBack failed on non-empty wall")
	}
	_ = back
	if w.Remaining() != origLen-1 {
		t.Errorf("remaining after DrawFromBack = %d, want %d", w.Remaining(), origLen-1)
	}
	// Draw from front should still work.
	front, ok := w.Draw()
	if !ok {
		t.Fatal("front Draw failed after DrawFromBack")
	}
	_ = front
	if w.Remaining() != origLen-2 {
		t.Errorf("remaining = %d, want %d", w.Remaining(), origLen-2)
	}
}
