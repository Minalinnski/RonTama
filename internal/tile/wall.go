package tile

import (
	"crypto/rand"
	"encoding/binary"
	"fmt"
)

// Wall is the shuffled stack of tiles for a hand.
//
// Tiles are drawn from the front (Drawn cursor moves forward). Riichi
// rules carve out a 14-tile dead wall at the back of the wall — that's
// modeled by callers, not by Wall itself, so the same Wall serves
// both rule sets.
type Wall struct {
	Tiles []Tile
	Drawn int
}

// NewWall builds a wall containing `copies` of each kind in `kinds`,
// shuffled with crypto/rand. For Sichuan: kinds=SuitedKinds(), copies=4.
// For Riichi: kinds=AllKinds(), copies=4.
func NewWall(kinds []Tile, copies int) (*Wall, error) {
	tiles := make([]Tile, 0, len(kinds)*copies)
	for _, k := range kinds {
		for i := 0; i < copies; i++ {
			tiles = append(tiles, k)
		}
	}
	if err := shuffle(tiles); err != nil {
		return nil, fmt.Errorf("wall: %w", err)
	}
	return &Wall{Tiles: tiles}, nil
}

// Remaining returns the count of tiles still drawable.
func (w *Wall) Remaining() int { return len(w.Tiles) - w.Drawn }

// Draw pops one tile from the front. Returns false if the wall is empty.
func (w *Wall) Draw() (Tile, bool) {
	if w.Drawn >= len(w.Tiles) {
		return 0, false
	}
	t := w.Tiles[w.Drawn]
	w.Drawn++
	return t, true
}

// SplitDeadWall removes `n` tiles from the END of the wall and
// returns them. The wall's drawable range shrinks accordingly.
//
// In Riichi mahjong the dead wall is always 14 tiles (7 pairs). The
// first pair's upper tile is the initial dora indicator; the rest
// are used for kan-replacement draws and additional dora flips.
//
// Must be called AFTER NewWall and BEFORE any Draw calls (Drawn == 0).
// Panics if the wall is too small or draws have already started.
func (w *Wall) SplitDeadWall(n int) []Tile {
	if w.Drawn != 0 {
		panic("SplitDeadWall: draws already started")
	}
	if n > len(w.Tiles) {
		panic(fmt.Sprintf("SplitDeadWall: n=%d > wall size %d", n, len(w.Tiles)))
	}
	split := len(w.Tiles) - n
	dead := make([]Tile, n)
	copy(dead, w.Tiles[split:])
	w.Tiles = w.Tiles[:split]
	return dead
}

// DrawFromBack pops one tile from the END of the wall. Used for
// kan-replacement draws (嶺上牌). Returns false if the wall is empty.
func (w *Wall) DrawFromBack() (Tile, bool) {
	if w.Drawn >= len(w.Tiles) {
		return 0, false
	}
	t := w.Tiles[len(w.Tiles)-1]
	w.Tiles = w.Tiles[:len(w.Tiles)-1]
	return t, true
}

// DrawN pops up to n tiles. Returns the slice (may be shorter if the
// wall ran out).
func (w *Wall) DrawN(n int) []Tile {
	out := make([]Tile, 0, n)
	for i := 0; i < n; i++ {
		t, ok := w.Draw()
		if !ok {
			break
		}
		out = append(out, t)
	}
	return out
}

// shuffle does a Fisher-Yates shuffle using crypto/rand. We use
// crypto/rand (not math/rand) so a friendly LAN game can't be
// challenged on shuffle reproducibility.
func shuffle(tiles []Tile) error {
	for i := len(tiles) - 1; i > 0; i-- {
		j, err := randIntN(i + 1)
		if err != nil {
			return err
		}
		tiles[i], tiles[j] = tiles[j], tiles[i]
	}
	return nil
}

// randIntN returns a uniformly random int in [0, n) using crypto/rand.
// Uses rejection sampling to avoid modulo bias.
func randIntN(n int) (int, error) {
	if n <= 0 {
		return 0, fmt.Errorf("randIntN: n must be positive, got %d", n)
	}
	if n == 1 {
		return 0, nil
	}
	// Use 8 bytes (uint64) and reject values in the bias zone.
	max := ^uint64(0)
	limit := max - (max % uint64(n))
	var buf [8]byte
	for {
		if _, err := rand.Read(buf[:]); err != nil {
			return 0, err
		}
		v := binary.BigEndian.Uint64(buf[:])
		if v < limit {
			return int(v % uint64(n)), nil
		}
	}
}
