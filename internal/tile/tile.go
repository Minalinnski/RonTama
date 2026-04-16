// Package tile defines the core mahjong tile type and operations shared
// across rule sets (Sichuan and Riichi).
//
// Tiles are encoded as a small integer 0..33 ("kinds"), grouped by suit:
//
//	0..8   = 1m..9m (萬子)
//	9..17  = 1p..9p (筒子)
//	18..26 = 1s..9s (索子)
//	27..30 = E S W N (風牌)
//	31..33 = White Green Red dragons (三元牌)
//
// Sichuan rules use only the suited tiles 0..26; Riichi uses all 34.
//
// Aka (red) dora variants for Riichi (5m/5p/5s) are tracked at the wall /
// hand level via a separate Aka flag, not encoded into the Tile value, so
// shanten and rule logic can ignore them uniformly.
package tile

import "fmt"

// Tile is the kind of mahjong tile.
type Tile uint8

// All tile kinds.
const (
	Man1 Tile = iota
	Man2
	Man3
	Man4
	Man5
	Man6
	Man7
	Man8
	Man9
	Pin1
	Pin2
	Pin3
	Pin4
	Pin5
	Pin6
	Pin7
	Pin8
	Pin9
	Sou1
	Sou2
	Sou3
	Sou4
	Sou5
	Sou6
	Sou7
	Sou8
	Sou9
	East
	South
	West
	North
	White
	Green
	Red
)

// NumKinds is the number of distinct tile kinds (34).
const NumKinds = 34

// Suit identifies the suit family of a tile.
type Suit uint8

const (
	SuitMan    Suit = iota // 萬
	SuitPin                // 筒
	SuitSou                // 索
	SuitWind               // 風
	SuitDragon             // 三元
)

// Suit returns the suit family of t.
func (t Tile) Suit() Suit {
	switch {
	case t < 9:
		return SuitMan
	case t < 18:
		return SuitPin
	case t < 27:
		return SuitSou
	case t < 31:
		return SuitWind
	default:
		return SuitDragon
	}
}

// Number returns 1..9 for suited tiles, 0 for honors.
func (t Tile) Number() int {
	if t < 27 {
		return int(t%9) + 1
	}
	return 0
}

// IsSuit reports whether t is a numbered suit tile (man/pin/sou).
func (t Tile) IsSuit() bool { return t < 27 }

// IsHonor reports whether t is a wind or dragon tile.
func (t Tile) IsHonor() bool { return t >= 27 }

// IsWind reports whether t is one of the four wind tiles.
func (t Tile) IsWind() bool { return t >= East && t <= North }

// IsDragon reports whether t is one of the three dragon tiles.
func (t Tile) IsDragon() bool { return t >= White && t <= Red }

// IsTerminal reports whether t is a terminal (1 or 9 of a suit) or any honor.
// Yaochuuhai (幺九牌): the set used for kokushi musou.
func (t Tile) IsTerminal() bool {
	if t.IsHonor() {
		return true
	}
	n := t.Number()
	return n == 1 || n == 9
}

// IsTerminalSuit reports whether t is a 1 or 9 of a suit (not honor).
func (t Tile) IsTerminalSuit() bool {
	if !t.IsSuit() {
		return false
	}
	n := t.Number()
	return n == 1 || n == 9
}

// IsSimple reports whether t is a 2..8 of any suit (not terminal/honor).
// Tanyao (断幺九) tiles.
func (t Tile) IsSimple() bool { return !t.IsTerminal() }

// SuitOffset returns t's index within its suit (0..8) or 0..6 for honors.
// Useful for per-suit table lookups in shanten / scoring.
func (t Tile) SuitOffset() int {
	switch {
	case t < 27:
		return int(t % 9)
	default:
		return int(t - 27)
	}
}

// String returns a short identifier mixing ASCII for suits and CJK
// for honors: "1m", "9p", "5s", "東", "南", "西", "北", "白", "發", "中".
//
// CJK characters are 2 terminal columns wide (same as "1m"), so the
// renderer can lay them out in the same fixed-width tile boxes.
func (t Tile) String() string {
	if t.IsSuit() {
		suit := "mps"[t/9]
		return fmt.Sprintf("%d%c", t.Number(), suit)
	}
	switch t {
	case East:
		return "東"
	case South:
		return "南"
	case West:
		return "西"
	case North:
		return "北"
	case White:
		return "白"
	case Green:
		return "發"
	case Red:
		return "中"
	}
	return fmt.Sprintf("?%d", uint8(t))
}

// Unicode returns the Unicode mahjong glyph for t (e.g. "🀇" for 1m).
//
// Note: many terminal fonts render these as half-width or with poor
// alignment; production TUI uses ASCII boxes for hand display and falls
// back to these glyphs only for compact river/discard areas.
func (t Tile) Unicode() string {
	if int(t) < len(unicodeGlyphs) {
		return unicodeGlyphs[t]
	}
	return "?"
}

// unicodeGlyphs maps tile kinds to their Unicode mahjong characters.
// The Unicode block (U+1F000..U+1F021) is laid out as winds → dragons →
// chars → bamboos → circles, which differs from our index ordering, so
// we map explicitly.
var unicodeGlyphs = [NumKinds]string{
	"🀇", "🀈", "🀉", "🀊", "🀋", "🀌", "🀍", "🀎", "🀏", // 1m..9m
	"🀙", "🀚", "🀛", "🀜", "🀝", "🀞", "🀟", "🀠", "🀡", // 1p..9p
	"🀐", "🀑", "🀒", "🀓", "🀔", "🀕", "🀖", "🀗", "🀘", // 1s..9s
	"🀀", "🀁", "🀂", "🀃", // E S W N
	"🀆", "🀅", "🀄", // White Green Red
}

// AllKinds returns a fresh slice of every tile kind in canonical order.
func AllKinds() []Tile {
	out := make([]Tile, NumKinds)
	for i := range out {
		out[i] = Tile(i)
	}
	return out
}

// SuitedKinds returns the 27 numbered suit tiles (used for Sichuan).
func SuitedKinds() []Tile {
	out := make([]Tile, 27)
	for i := range out {
		out[i] = Tile(i)
	}
	return out
}
