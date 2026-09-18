package manifest

import "testing"

func good() Manifest {
	return Manifest{
		Version: Version, GameVersion: 1,
		Cols: 7, Rows: 6, Line: 4, Boards: 3, Wins: 2,
		MoveDeadlineBlocks: 12, MatchDeadlineBlocks: 144,
		OpenerRule: OpenerSeatThenEarned, PayoutRule: PayoutWinnerTakesPot,
	}
}

func TestAWholeManifestHashes(t *testing.T) {
	if err := good().Validate(); err != nil {
		t.Fatalf("a complete manifest was refused: %v", err)
	}
	h, err := good().Hash()
	if err != nil {
		t.Fatalf("hash: %v", err)
	}
	again, err := good().Hash()
	if err != nil || h != again {
		t.Fatal("the same manifest hashed differently twice")
	}
}

// Every field is part of the agreement, so changing any one of them has to
// change the hash. A field left out of the encoding is a field two peers could
// silently disagree about while believing they had agreed.
func TestEveryFieldChangesTheHash(t *testing.T) {
	base, err := good().Hash()
	if err != nil {
		t.Fatalf("hash: %v", err)
	}
	for name, mut := range map[string]func(*Manifest){
		"game version":   func(m *Manifest) { m.GameVersion = 2 },
		"columns":        func(m *Manifest) { m.Cols = 8 },
		"rows":           func(m *Manifest) { m.Rows = 7 },
		"line":           func(m *Manifest) { m.Line = 5 },
		"boards":         func(m *Manifest) { m.Boards = 5 },
		"wins":           func(m *Manifest) { m.Wins = 3 },
		"move deadline":  func(m *Manifest) { m.MoveDeadlineBlocks = 24 },
		"match deadline": func(m *Manifest) { m.MatchDeadlineBlocks = 288 },
	} {
		t.Run(name, func(t *testing.T) {
			m := good()
			mut(&m)
			h, err := m.Hash()
			if err != nil {
				t.Fatalf("hash: %v", err)
			}
			if h == base {
				t.Fatalf("changing the %s did not change the hash", name)
			}
			if m.Same(good()) {
				t.Fatalf("a manifest with a different %s compared equal", name)
			}
		})
	}
}

func TestAnUnplayableManifestIsRefused(t *testing.T) {
	for name, mut := range map[string]func(*Manifest){
		"wrong version":        func(m *Manifest) { m.Version = 2 },
		"no game version":      func(m *Manifest) { m.GameVersion = 0 },
		"board too small":      func(m *Manifest) { m.Cols = 3 },
		"line too short":       func(m *Manifest) { m.Line = 2 },
		"more wins than games": func(m *Manifest) { m.Wins = 4 },
		"no boards":            func(m *Manifest) { m.Boards = 0 },
		"no move deadline":     func(m *Manifest) { m.MoveDeadlineBlocks = 0 },
		"move outlasts match":  func(m *Manifest) { m.MoveDeadlineBlocks = 200 },
		"unknown opener":       func(m *Manifest) { m.OpenerRule = "coin-flip" },
		"unknown payout":       func(m *Manifest) { m.PayoutRule = "house-takes-half" },
	} {
		t.Run(name, func(t *testing.T) {
			m := good()
			mut(&m)
			if err := m.Validate(); err == nil {
				t.Fatalf("%s was accepted", name)
			}
			if _, err := m.Hash(); err == nil {
				t.Fatalf("%s hashed anyway", name)
			}
		})
	}
}

// The hash is a frozen input: every signature ever made over it depends on
// these exact bytes. If this value changes, old signatures stop verifying.
func TestTheHashIsFrozen(t *testing.T) {
	h, err := good().Hash()
	if err != nil {
		t.Fatalf("hash: %v", err)
	}
	const want = "e2f6856157f303bc"
	got := hex16(h)
	if got != want {
		t.Fatalf("the manifest hash changed: %s (was %s).\n"+
			"If this is a deliberate rules change, bump Version and update this test; "+
			"if it is not, the encoding drifted and every existing signature is now invalid.", got, want)
	}
}

func hex16(h [32]byte) string {
	const digits = "0123456789abcdef"
	out := make([]byte, 16)
	for i := 0; i < 8; i++ {
		out[i*2] = digits[h[i]>>4]
		out[i*2+1] = digits[h[i]&0xf]
	}
	return string(out)
}
