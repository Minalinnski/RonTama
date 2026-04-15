package riichi

// basePoints computes the "base" point value (基本点) that the standard
// Riichi payment matrix multiplies.
//
//	basic = fu * 2^(han + 2)
//	caps:
//	  mangan       = 2000  (5 han, or 4 han 40 fu, or 3 han 70 fu)
//	  haneman      = 3000  (6-7 han)
//	  baiman       = 4000  (8-10 han)
//	  sanbaiman    = 6000  (11-12 han)
//	  yakuman      = 8000  (13+ han, or yakuman yaku)
//
// Multiple yakuman stack additively (double-yakuman = 16000, etc.).
func basePoints(han, fu int, yakumanNames []string) int {
	if len(yakumanNames) > 0 {
		return 8000 * len(yakumanNames)
	}
	switch {
	case han >= 13:
		return 8000
	case han >= 11:
		return 6000
	case han >= 8:
		return 4000
	case han >= 6:
		return 3000
	case han >= 5:
		return 2000
	}
	// 1-4 han: compute from fu
	pts := fu << (han + 2) // fu * 2^(han+2)
	if pts > 2000 {
		return 2000 // mangan cap
	}
	return pts
}
