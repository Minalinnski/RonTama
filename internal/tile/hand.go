package tile

import (
	"fmt"
	"sort"
)

// MeldKind classifies a declared meld.
type MeldKind uint8

const (
	Chi          MeldKind = iota // 吃 / 顺子
	Pon                          // 碰 / 刻子
	Kan                          // 杠 (open / minkan)
	ConcealedKan                 // 暗杠
	AddedKan                     // 加杠 (pon -> kan)
)

func (k MeldKind) String() string {
	switch k {
	case Chi:
		return "Chi"
	case Pon:
		return "Pon"
	case Kan:
		return "Kan"
	case ConcealedKan:
		return "AnKan"
	case AddedKan:
		return "AddedKan"
	default:
		return fmt.Sprintf("MeldKind(%d)", k)
	}
}

// Meld is a declared (or concealed) set of tiles that no longer
// participates in shanten calculation of the concealed hand.
//
// Tiles are stored in canonical sorted order. From is the seat index
// (0..3) of the player whose discard formed the meld; -1 for
// concealed kan.
type Meld struct {
	Kind  MeldKind
	Tiles []Tile
	From  int
}

// IsConcealed reports whether the meld counts as concealed for scoring
// (only ConcealedKan).
func (m Meld) IsConcealed() bool { return m.Kind == ConcealedKan }

// IsKan reports whether the meld is any kind of kan.
func (m Meld) IsKan() bool {
	return m.Kind == Kan || m.Kind == ConcealedKan || m.Kind == AddedKan
}

// SetCount returns how many "sets" the meld counts as for shanten /
// scoring purposes (always 1; kans replace a triplet for shanten).
func (m Meld) SetCount() int { return 1 }

// Hand is a player's current tiles: the concealed portion as a count
// vector plus declared melds.
//
// The concealed portion has 13 - 3*len(melds) tiles when waiting and
// one more right after a draw / call.
type Hand struct {
	Concealed [NumKinds]int
	Melds     []Meld
}

// NewHand builds a Hand from a slice of tiles (no melds).
func NewHand(tiles []Tile) Hand {
	var h Hand
	for _, t := range tiles {
		h.Concealed[t]++
	}
	return h
}

// ConcealedCount returns the total number of concealed tiles in the hand.
func (h Hand) ConcealedCount() int {
	n := 0
	for _, c := range h.Concealed {
		n += c
	}
	return n
}

// Add adds one tile to the concealed portion.
func (h *Hand) Add(t Tile) { h.Concealed[t]++ }

// Remove removes one tile from the concealed portion. Returns false if
// no such tile is present.
func (h *Hand) Remove(t Tile) bool {
	if h.Concealed[t] == 0 {
		return false
	}
	h.Concealed[t]--
	return true
}

// ConcealedTiles returns the concealed tiles as a sorted slice.
func (h Hand) ConcealedTiles() []Tile {
	out := make([]Tile, 0, h.ConcealedCount())
	for i, c := range h.Concealed {
		for j := 0; j < c; j++ {
			out = append(out, Tile(i))
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i] < out[j] })
	return out
}

// String renders the hand in compact "1m 2m 3m | Pon(5p)" form.
func (h Hand) String() string {
	tiles := h.ConcealedTiles()
	s := ""
	for i, t := range tiles {
		if i > 0 {
			s += " "
		}
		s += t.String()
	}
	for _, m := range h.Melds {
		s += " | " + m.Kind.String() + "("
		for i, t := range m.Tiles {
			if i > 0 {
				s += " "
			}
			s += t.String()
		}
		s += ")"
	}
	return s
}
