package board

import "testing"

func drop(t *testing.T, b *Board, col int, seat uint32) int {
	t.Helper()
	row, err := b.Drop(col, seat)
	if err != nil {
		t.Fatalf("drop column %d for seat %d: %v", col, seat, err)
	}
	return row
}

func TestPiecesStackFromTheBottom(t *testing.T) {
	b := New()
	if row := drop(t, b, 3, 0); row != 0 {
		t.Fatalf("first piece landed in row %d, want the floor", row)
	}
	if row := drop(t, b, 3, 1); row != 1 {
		t.Fatalf("second piece landed in row %d, want on top of the first", row)
	}
	if b.At(3, 0) == b.At(3, 1) {
		t.Fatal("two seats' pieces are indistinguishable")
	}
}

func TestAFullColumnRefusesAnother(t *testing.T) {
	b := New()
	for i := 0; i < Rows; i++ {
		drop(t, b, 0, uint32(i%2))
	}
	if _, err := b.Drop(0, 0); err == nil {
		t.Fatal("a seventh piece went into a six-row column")
	}
}

func TestAColumnOffTheBoardIsRefused(t *testing.T) {
	b := New()
	for _, col := range []int{-1, Cols} {
		if _, err := b.Drop(col, 0); err == nil {
			t.Fatalf("column %d was accepted", col)
		}
	}
}

func TestASeatThatIsNotAtTheTableIsRefused(t *testing.T) {
	b := New()
	if _, err := b.Drop(0, 2); err == nil {
		t.Fatal("a third seat played a heads-up board")
	}
}

// Every direction, because a scan that misses one is a scan that pays the wrong
// player.
func TestFourInALineWinsInEveryDirection(t *testing.T) {
	for _, tc := range []struct {
		name string
		play [][2]int // column, seat
	}{
		{"vertical", [][2]int{{0, 0}, {1, 1}, {0, 0}, {1, 1}, {0, 0}, {1, 1}, {0, 0}}},
		{"horizontal", [][2]int{{0, 0}, {0, 1}, {1, 0}, {1, 1}, {2, 0}, {2, 1}, {3, 0}}},
		{"rising", [][2]int{
			{0, 0}, {1, 1}, {1, 0}, {2, 1}, {2, 0}, {3, 1}, {2, 0}, {3, 1}, {3, 0}, {6, 1}, {3, 0},
		}},
		{"falling", [][2]int{
			{6, 0}, {5, 1}, {5, 0}, {4, 1}, {4, 0}, {3, 1}, {4, 0}, {3, 1}, {3, 0}, {0, 1}, {3, 0},
		}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			b := New()
			for _, p := range tc.play {
				drop(t, b, p[0], uint32(p[1]))
			}
			seat, won := b.Winner()
			if !won {
				t.Fatal("four in a line was not seen")
			}
			if seat != 0 {
				t.Fatalf("seat %d was paid for seat 0's line", seat)
			}
		})
	}
}

func TestAnEmptyBoardHasNoWinnerAndIsNotOver(t *testing.T) {
	b := New()
	if _, won := b.Winner(); won {
		t.Fatal("an empty board has a winner")
	}
	if b.Over() {
		t.Fatal("an empty board is over")
	}
}

// Three in a line is not four. The off-by-one here pays somebody.
func TestThreeInALineIsNotAWin(t *testing.T) {
	b := New()
	for col := 0; col < 3; col++ {
		drop(t, b, col, 0)
	}
	if _, won := b.Winner(); won {
		t.Fatal("three in a row won the board")
	}
}

func TestMovesCountsEveryPiece(t *testing.T) {
	b := New()
	for i := 0; i < 5; i++ {
		drop(t, b, i, uint32(i%2))
	}
	if b.Moves() != 5 {
		t.Fatalf("board counted %d moves, want 5", b.Moves())
	}
}
