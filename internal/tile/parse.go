package tile

import (
	"fmt"
	"strings"
)

// Parse converts a short identifier into a Tile.
// Accepts: "1m".."9m", "1p".."9p", "1s".."9s", honors as either
// CJK ("東/南/西/北/白/發/中") or legacy ASCII ("E/S/W/N/Wh/Gr/Rd").
func Parse(s string) (Tile, error) {
	switch s {
	case "東", "E":
		return East, nil
	case "南", "S":
		return South, nil
	case "西", "W":
		return West, nil
	case "北", "N":
		return North, nil
	case "白", "Wh":
		return White, nil
	case "發", "发", "Gr":
		return Green, nil
	case "中", "Rd":
		return Red, nil
	}
	if len(s) == 2 {
		n := int(s[0] - '0')
		if n >= 1 && n <= 9 {
			switch s[1] {
			case 'm':
				return Tile(n - 1), nil
			case 'p':
				return Tile(9 + n - 1), nil
			case 's':
				return Tile(18 + n - 1), nil
			}
		}
	}
	return 0, fmt.Errorf("tile.Parse: invalid %q", s)
}

// MustParse is Parse but panics on error. For tests / fixtures.
func MustParse(s string) Tile {
	t, err := Parse(s)
	if err != nil {
		panic(err)
	}
	return t
}

// ParseHand parses a compact hand notation like "123m 456p 789s 1122z"
// into a slice of tiles. Honor digits 1..7 in a "z" group map to
// East,South,West,North,White,Green,Red.
//
// Spaces between groups are optional. This is the standard Tenhou-style
// notation used in test fixtures across mahjong literature.
func ParseHand(s string) ([]Tile, error) {
	s = strings.ReplaceAll(s, " ", "")
	var out []Tile
	var pending []byte
	for i := 0; i < len(s); i++ {
		c := s[i]
		switch c {
		case 'm', 'p', 's', 'z':
			for _, d := range pending {
				n := int(d - '0')
				switch c {
				case 'm':
					if n < 1 || n > 9 {
						return nil, fmt.Errorf("invalid man digit %q", d)
					}
					out = append(out, Tile(n-1))
				case 'p':
					if n < 1 || n > 9 {
						return nil, fmt.Errorf("invalid pin digit %q", d)
					}
					out = append(out, Tile(9+n-1))
				case 's':
					if n < 1 || n > 9 {
						return nil, fmt.Errorf("invalid sou digit %q", d)
					}
					out = append(out, Tile(18+n-1))
				case 'z':
					if n < 1 || n > 7 {
						return nil, fmt.Errorf("invalid honor digit %q", d)
					}
					out = append(out, Tile(27+n-1))
				}
			}
			pending = pending[:0]
		default:
			if c < '0' || c > '9' {
				return nil, fmt.Errorf("unexpected char %q in hand %q", c, s)
			}
			pending = append(pending, c)
		}
	}
	if len(pending) > 0 {
		return nil, fmt.Errorf("trailing digits without suit in %q", s)
	}
	return out, nil
}

// MustParseHand is ParseHand but panics on error.
func MustParseHand(s string) []Tile {
	out, err := ParseHand(s)
	if err != nil {
		panic(err)
	}
	return out
}

// Counts returns a [NumKinds] count vector from a slice of tiles.
func Counts(tiles []Tile) [NumKinds]int {
	var c [NumKinds]int
	for _, t := range tiles {
		c[t]++
	}
	return c
}
