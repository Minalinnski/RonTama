package tile

import (
	"testing"
)

func TestTile_Suit(t *testing.T) {
	cases := []struct {
		t    Tile
		suit Suit
	}{
		{Man1, SuitMan},
		{Man9, SuitMan},
		{Pin1, SuitPin},
		{Pin5, SuitPin},
		{Sou9, SuitSou},
		{East, SuitWind},
		{North, SuitWind},
		{White, SuitDragon},
		{Red, SuitDragon},
	}
	for _, c := range cases {
		if got := c.t.Suit(); got != c.suit {
			t.Errorf("%s.Suit() = %d, want %d", c.t, got, c.suit)
		}
	}
}

func TestTile_NumberAndPredicates(t *testing.T) {
	if Man5.Number() != 5 {
		t.Errorf("Man5.Number() = %d", Man5.Number())
	}
	if East.Number() != 0 {
		t.Errorf("East.Number() = %d, want 0", East.Number())
	}
	if !Man1.IsTerminal() {
		t.Error("Man1 should be terminal")
	}
	if !Man9.IsTerminal() {
		t.Error("Man9 should be terminal")
	}
	if Man5.IsTerminal() {
		t.Error("Man5 should not be terminal")
	}
	if !East.IsTerminal() {
		t.Error("East (honor) should be terminal/yaochuu")
	}
	if !Man5.IsSimple() {
		t.Error("Man5 should be simple")
	}
	if East.IsSimple() {
		t.Error("East should not be simple")
	}
	if !East.IsWind() || East.IsDragon() {
		t.Error("East: wind=true, dragon=false")
	}
	if !Red.IsDragon() || Red.IsWind() {
		t.Error("Red: dragon=true, wind=false")
	}
}

func TestTile_String(t *testing.T) {
	cases := map[Tile]string{
		Man1: "1m", Man9: "9m",
		Pin1: "1p", Pin9: "9p",
		Sou1: "1s", Sou9: "9s",
		East: "E", South: "S", West: "W", North: "N",
		White: "Wh", Green: "Gr", Red: "Rd",
	}
	for tile, want := range cases {
		if got := tile.String(); got != want {
			t.Errorf("%d.String() = %q, want %q", tile, got, want)
		}
	}
}

func TestParse_Roundtrip(t *testing.T) {
	for i := 0; i < NumKinds; i++ {
		tile := Tile(i)
		got, err := Parse(tile.String())
		if err != nil {
			t.Fatalf("Parse(%q): %v", tile.String(), err)
		}
		if got != tile {
			t.Errorf("Parse(%q) = %d, want %d", tile.String(), got, tile)
		}
	}
}

func TestParse_CJKAliases(t *testing.T) {
	cases := map[string]Tile{
		"東": East, "南": South, "西": West, "北": North,
		"白": White, "發": Green, "中": Red,
	}
	for s, want := range cases {
		got, err := Parse(s)
		if err != nil {
			t.Fatalf("Parse(%q): %v", s, err)
		}
		if got != want {
			t.Errorf("Parse(%q) = %d, want %d", s, got, want)
		}
	}
}

func TestParseHand(t *testing.T) {
	cases := []struct {
		in   string
		want []Tile
	}{
		{"123m", []Tile{Man1, Man2, Man3}},
		{"456p", []Tile{Pin4, Pin5, Pin6}},
		{"789s", []Tile{Sou7, Sou8, Sou9}},
		{"1234567z", []Tile{East, South, West, North, White, Green, Red}},
		{"123m 456p 789s 11z", []Tile{Man1, Man2, Man3, Pin4, Pin5, Pin6, Sou7, Sou8, Sou9, East, East}},
	}
	for _, c := range cases {
		got, err := ParseHand(c.in)
		if err != nil {
			t.Fatalf("ParseHand(%q): %v", c.in, err)
		}
		if len(got) != len(c.want) {
			t.Fatalf("ParseHand(%q) len = %d, want %d", c.in, len(got), len(c.want))
		}
		for i, tile := range got {
			if tile != c.want[i] {
				t.Errorf("ParseHand(%q)[%d] = %d, want %d", c.in, i, tile, c.want[i])
			}
		}
	}
}

func TestParseHand_Errors(t *testing.T) {
	bad := []string{"123", "0m", "8z", "abc", "12x"}
	for _, s := range bad {
		if _, err := ParseHand(s); err == nil {
			t.Errorf("ParseHand(%q) should error", s)
		}
	}
}

func TestAllKinds(t *testing.T) {
	all := AllKinds()
	if len(all) != NumKinds {
		t.Fatalf("AllKinds() len = %d, want %d", len(all), NumKinds)
	}
	for i, tile := range all {
		if tile != Tile(i) {
			t.Errorf("AllKinds()[%d] = %d", i, tile)
		}
	}
}

func TestSuitedKinds(t *testing.T) {
	suited := SuitedKinds()
	if len(suited) != 27 {
		t.Fatalf("SuitedKinds() len = %d, want 27", len(suited))
	}
	for _, tile := range suited {
		if tile.IsHonor() {
			t.Errorf("SuitedKinds() contains honor %s", tile)
		}
	}
}
