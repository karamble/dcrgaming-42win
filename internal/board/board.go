// Package board is the four-in-a-row grid and nothing else: no clock, no
// randomness, no network, no money.
//
// A board is only ever rebuilt by replaying moves the log already agreed on, so
// everything here is a pure function of those moves. Two peers that disagree
// about what a grid holds disagree about who won, and the point of the log is
// that they cannot.
package board

import "fmt"

// The standard grid: seven columns, six rows, four in a line to win.
const (
	Cols = 7
	Rows = 6
	Line = 4
)

// Seats is how many seats a board has. Two, and the rest of the game says so:
// the opener schedule, the scoring and the escrow are all heads-up.
const Seats = 2

// Cell is what occupies one square.
type Cell uint8

const (
	Empty  Cell = iota
	First       // seat 0's mark
	Second      // seat 1's mark
)

// MarkOf is the cell a seat plays.
func MarkOf(seat uint32) (Cell, error) {
	if seat >= Seats {
		return Empty, fmt.Errorf("seat %d: a board has seats 0 and %d", seat, Seats-1)
	}
	return Cell(seat + 1), nil
}

// Seat is whose mark this is. Empty has no seat.
func (c Cell) Seat() (uint32, bool) {
	if c == Empty {
		return 0, false
	}
	return uint32(c) - 1, true
}

// Board is a grid mid-play. Its zero value is an empty board.
//
// Rows count from the bottom, because that is the direction pieces fall and a
// coordinate system that disagreed with gravity would be wrong in every loop
// below.
type Board struct {
	cells  [Cols][Rows]Cell
	height [Cols]int
	moves  int
}

// New is an empty board.
func New() *Board { return &Board{} }

// Moves is how many pieces have been dropped. It is what decides who opens the
// third board, so it is part of the result rather than a diagnostic.
func (b *Board) Moves() int { return b.moves }

// At is what occupies a square. Out-of-range squares are Empty, so the win scan
// can walk off the edge without bounds-checking every step.
func (b *Board) At(col, row int) Cell {
	if col < 0 || col >= Cols || row < 0 || row >= Rows {
		return Empty
	}
	return b.cells[col][row]
}

// Full reports whether every square is taken.
func (b *Board) Full() bool { return b.moves >= Cols*Rows }

// Drop plays a seat's piece in a column and reports the row it landed in.
//
// It refuses a full column and an unknown seat, and it does not care whose turn
// it is: turn order is the match's rule, not the grid's, exactly as the log's
// structure is separate from whether a seat was entitled to act.
func (b *Board) Drop(col int, seat uint32) (int, error) {
	mark, err := MarkOf(seat)
	if err != nil {
		return 0, err
	}
	if col < 0 || col >= Cols {
		return 0, fmt.Errorf("column %d: the board has columns 0 to %d", col, Cols-1)
	}
	row := b.height[col]
	if row >= Rows {
		return 0, fmt.Errorf("column %d is full", col)
	}
	b.cells[col][row] = mark
	b.height[col]++
	b.moves++
	return row, nil
}

// directions are the four axes a line can run along. Their opposites are walked
// by negating the step, so four entries cover all eight directions.
var directions = [4][2]int{
	{1, 0},  // -
	{0, 1},  // |
	{1, 1},  // /
	{1, -1}, // \
}

// Winner is the seat holding a line of four, if any.
//
// A full scan rather than an incremental check around the last piece: it is
// forty-two squares, it is a pure function of the grid alone, and a verifier
// that reaches the same verdict without being told which move was last is one
// fewer thing a transcript has to be trusted about.
func (b *Board) Winner() (uint32, bool) {
	for col := 0; col < Cols; col++ {
		for row := 0; row < Rows; row++ {
			cell := b.cells[col][row]
			if cell == Empty {
				continue
			}
			for _, d := range directions {
				n := 1
				for n < Line && b.At(col+d[0]*n, row+d[1]*n) == cell {
					n++
				}
				if n == Line {
					seat, _ := cell.Seat()
					return seat, true
				}
			}
		}
	}
	return 0, false
}

// Over reports whether the board can still be played: a line ends it, and so
// does a full grid with no line in it.
func (b *Board) Over() bool {
	if _, won := b.Winner(); won {
		return true
	}
	return b.Full()
}
