package match_test

import (
	"testing"

	"github.com/karamble/dcr4inarow/internal/board"
	"github.com/karamble/dcr4inarow/internal/match"
)

// Column orders that finish a board, played by whoever is to move. The opener
// takes the even positions, so these are read as "opener, responder, opener".
var (
	openerWinsIn7    = []uint8{0, 1, 0, 1, 0, 1, 0}
	openerWinsIn9    = []uint8{2, 3, 0, 1, 0, 1, 0, 1, 0}
	responderWinsIn8 = []uint8{1, 0, 1, 0, 1, 0, 2, 0}
)

func play(t *testing.T, m *match.Match, cols []uint8) {
	t.Helper()
	for i, col := range cols {
		mv := match.Move{Board: uint8(m.Index()), Seat: m.Turn(), Column: col}
		if err := m.Play(mv); err != nil {
			t.Fatalf("move %d into column %d: %v", i, col, err)
		}
	}
}

func start(t *testing.T, creator uint32) *match.Match {
	t.Helper()
	m, err := match.New(creator)
	if err != nil {
		t.Fatalf("new match: %v", err)
	}
	return m
}

// drawSequence finds a column order that fills a board with nobody making a
// line.
//
// Searched rather than written down on purpose: a hand-made "draw" that quietly
// contained a line would make every test built on it vacuous, whereas a
// sequence found under the rule "no move may complete a line" is a draw by
// construction.
func drawSequence(opener uint32) ([]uint8, bool) {
	var cols []uint8
	budget := 2_000_000
	var walk func(b board.Board, turn uint32) bool
	walk = func(b board.Board, turn uint32) bool {
		if budget--; budget <= 0 {
			return false
		}
		if b.Full() {
			return true
		}
		for col := 0; col < board.Cols; col++ {
			next := b
			if _, err := next.Drop(col, turn); err != nil {
				continue
			}
			if _, won := next.Winner(); won {
				continue
			}
			cols = append(cols, uint8(col))
			if walk(next, match.Other(turn)) {
				return true
			}
			cols = cols[:len(cols)-1]
		}
		return false
	}
	if !walk(board.Board{}, opener) {
		return nil, false
	}
	return cols, true
}

func drawBoard(t *testing.T, m *match.Match) {
	t.Helper()
	cols, ok := drawSequence(m.Turn())
	if !ok {
		t.Fatal("no drawn board found within the search budget")
	}
	play(t, m, cols)
}

func TestTheCreatorOpensTheFirstBoardAndTheOtherSeatTheSecond(t *testing.T) {
	m := start(t, 1)
	if m.Turn() != 1 {
		t.Fatalf("seat %d opened the first board, want the creator", m.Turn())
	}
	play(t, m, openerWinsIn7)
	if m.Index() != 1 {
		t.Fatalf("after a decided board the match is on board %d, want 1", m.Index())
	}
	if m.Turn() != 0 {
		t.Fatalf("seat %d opened the second board, want the non-creator", m.Turn())
	}
}

func TestTheThirdBoardOpensToWhoeverWonInFewerMoves(t *testing.T) {
	m := start(t, 0)
	play(t, m, openerWinsIn9) // seat 0 wins its own board, slowly
	play(t, m, openerWinsIn7) // seat 1 wins its own board, faster
	if m.Index() != 2 {
		t.Fatalf("one win each left the match on board %d, want 2", m.Index())
	}
	if m.Turn() != 1 {
		t.Fatalf("seat %d opened the decider; seat 1 won in fewer moves", m.Turn())
	}
}

func TestAnExactTieOnMovesFallsBackToTheCreator(t *testing.T) {
	m := start(t, 0)
	play(t, m, openerWinsIn7) // seat 0, seven moves
	play(t, m, openerWinsIn7) // seat 1, seven moves
	if m.Turn() != 0 {
		t.Fatalf("seat %d opened the decider; a tie belongs to the creator", m.Turn())
	}
}

func TestTwoWinsEndTheMatchBeforeTheThirdBoard(t *testing.T) {
	m := start(t, 0)
	play(t, m, openerWinsIn7)    // seat 0 opens and wins
	play(t, m, responderWinsIn8) // seat 1 opens, seat 0 wins again
	if !m.Done() {
		t.Fatal("two wins did not finish the match")
	}
	if len(m.Results()) != 2 {
		t.Fatalf("%d boards were played, want 2", len(m.Results()))
	}
	winner, won, done := m.Outcome()
	if !done || !won || winner != 0 {
		t.Fatalf("outcome was (%d, %v, %v), want seat 0 paid", winner, won, done)
	}
	if err := m.Play(match.Move{Board: 2, Seat: 0, Column: 0}); err == nil {
		t.Fatal("a finished match accepted another move")
	}
}

func TestThreeDrawnBoardsAreVoid(t *testing.T) {
	m := start(t, 0)
	for i := 0; i < match.Boards; i++ {
		drawBoard(t, m)
	}
	if !m.Done() {
		t.Fatal("three boards did not finish the match")
	}
	_, won, done := m.Outcome()
	if !done || won {
		t.Fatalf("outcome was (won %v, done %v), want void", won, done)
	}
}

// The sharp edge of "first to two": a seat can win a board, never lose one, and
// still be paid nothing. It is what was agreed, and it is worth failing loudly
// if it ever changes by accident.
func TestOneWinAndTwoDrawsIsStillVoid(t *testing.T) {
	m := start(t, 0)
	play(t, m, openerWinsIn7)
	drawBoard(t, m)
	drawBoard(t, m)
	if s := m.Score(); s[0] != 1 || s[1] != 0 {
		t.Fatalf("score is %v, want one win to nil", s)
	}
	_, won, done := m.Outcome()
	if !done || won {
		t.Fatalf("outcome was (won %v, done %v), want void", won, done)
	}
}

func TestAMoveOutOfTurnIsRefused(t *testing.T) {
	m := start(t, 0)
	if err := m.Play(match.Move{Board: 0, Seat: 1, Column: 0}); err == nil {
		t.Fatal("the seat that does not open played first")
	}
}

func TestAMoveOnTheWrongBoardIsRefused(t *testing.T) {
	m := start(t, 0)
	if err := m.Play(match.Move{Board: 1, Seat: 0, Column: 0}); err == nil {
		t.Fatal("a move for a later board was played on the first")
	}
}
