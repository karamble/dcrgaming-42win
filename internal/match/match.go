// Package match is the best-of-three rules: which seat opens which board, what
// a board result is worth, and when the table is finished.
//
// It is kept apart from the log for the reason the SDK keeps them apart. The
// log enforces structure - signature, signer, sequence, linkage - and says
// nothing about whether a seat was entitled to act. Whose turn it is is a rule,
// rules are the game's, and a verifier holds both and checks both.
package match

import (
	"fmt"

	"github.com/karamble/dcr4inarow/internal/board"
)

// Boards is the ceiling, Wins is what takes the pot.
//
// Three and two: the first seat to win two boards is paid, and a match that
// reaches three boards without anybody getting there is void rather than
// decided on some tiebreak nobody agreed to.
const (
	Boards = 3
	Wins   = 2
)

// Seats is the size of a table. Two.
const Seats = board.Seats

// Move is one drop, in the form the log records it.
type Move struct {
	Board  uint8
	Seat   uint32
	Column uint8
}

// Result is how one board finished. A drawn board is Decided false and scores
// nothing for either seat.
type Result struct {
	Winner  uint32
	Decided bool
	// Moves is how many pieces were dropped on this board. It decides who
	// opens the third board, so it is part of the result and not a statistic.
	Moves int
}

// Other is the seat that is not this one.
func Other(seat uint32) uint32 { return 1 - seat }

// Opener is the seat that plays first on a board.
//
// Board one goes to the table's creator and board two to the other seat, which
// is symmetric. Board three is only ever reached at one win each or worse, and
// giving it to a fixed seat would hand somebody the opening of a solved game
// for free - so it is earned: the seat that won its board in fewer moves opens.
// With nothing to compare - no wins, or a tie on move count - it falls back to
// the creator.
func Opener(index int, creator uint32, prior []Result) (uint32, error) {
	if index < 0 || index >= Boards {
		return 0, fmt.Errorf("board %d: a match has boards 0 to %d", index, Boards-1)
	}
	if creator >= Seats {
		return 0, fmt.Errorf("creator is seat %d, but a table has seats 0 and %d", creator, Seats-1)
	}
	switch index {
	case 0:
		return creator, nil
	case 1:
		return Other(creator), nil
	}
	if len(prior) < index {
		return 0, fmt.Errorf("board %d needs %d earlier results, got %d", index, index, len(prior))
	}

	// Fewest moves in a board this seat won, or none.
	best := [Seats]int{}
	for _, r := range prior {
		if !r.Decided {
			continue
		}
		if r.Winner >= Seats {
			return 0, fmt.Errorf("a board was won by seat %d, which is not at this table", r.Winner)
		}
		if best[r.Winner] == 0 || r.Moves < best[r.Winner] {
			best[r.Winner] = r.Moves
		}
	}
	switch {
	case best[0] != 0 && best[1] != 0:
		if best[0] < best[1] {
			return 0, nil
		}
		if best[1] < best[0] {
			return 1, nil
		}
		return creator, nil // an exact tie; rare, and the creator holds it
	case best[0] != 0:
		return 0, nil
	case best[1] != 0:
		return 1, nil
	default:
		return creator, nil // two drawn boards; nobody earned anything
	}
}

// Match is a table mid-play. Use New.
type Match struct {
	creator uint32
	results []Result
	cur     *board.Board
	index   int
	turn    uint32
	done    bool
}

// New starts a match whose first board is opened by the creator.
func New(creator uint32) (*Match, error) {
	if creator >= Seats {
		return nil, fmt.Errorf("creator is seat %d, but a table has seats 0 and %d", creator, Seats-1)
	}
	m := &Match{creator: creator, cur: board.New()}
	turn, err := Opener(0, creator, nil)
	if err != nil {
		return nil, err
	}
	m.turn = turn
	return m, nil
}

// Index is which board is being played, Turn is who owes a move, Board is the
// grid as it stands and Results are the boards already finished.
func (m *Match) Index() int          { return m.index }
func (m *Match) Turn() uint32        { return m.turn }
func (m *Match) Board() *board.Board { return m.cur }
func (m *Match) Results() []Result   { return append([]Result(nil), m.results...) }

// Score is how many boards each seat has won.
func (m *Match) Score() [Seats]int {
	var s [Seats]int
	for _, r := range m.results {
		if r.Decided {
			s[r.Winner]++
		}
	}
	return s
}

// Done reports whether the match is over: a seat reached two wins, or three
// boards were played.
func (m *Match) Done() bool { return m.done }

// Outcome is who takes the pot. won false on a finished match means void -
// every seat takes back its own stake, which is a real outcome and not a
// pretend split.
func (m *Match) Outcome() (winner uint32, won bool, done bool) {
	if !m.done {
		return 0, false, false
	}
	s := m.Score()
	for seat := uint32(0); seat < Seats; seat++ {
		if s[seat] >= Wins {
			return seat, true, true
		}
	}
	return 0, false, true
}

// CanPlay reports whether a move would be accepted, without making it.
//
// It exists so a caller can check before committing to something it cannot take
// back. Recording a move in a signed log and then discovering the rules refuse
// it leaves the log one move ahead of the game, and nothing can reconcile the
// two afterwards.
func (m *Match) CanPlay(mv Move) error {
	if m.done {
		return fmt.Errorf("the match is over")
	}
	if int(mv.Board) != m.index {
		return fmt.Errorf("move is for board %d, the match is on board %d", mv.Board, m.index)
	}
	if mv.Seat != m.turn {
		return fmt.Errorf("seat %d moved, it is seat %d's turn", mv.Seat, m.turn)
	}
	// A copy, because board.Board is a value: probing must not drop a piece.
	probe := *m.cur
	if _, err := probe.Drop(int(mv.Column), mv.Seat); err != nil {
		return err
	}
	return nil
}

// Play applies one move.
//
// It refuses a move out of turn, on the wrong board, or into a full or unreal
// column. What it cannot refuse is a move nobody signed: that is the log's job,
// and a caller checks both.
func (m *Match) Play(mv Move) error {
	if err := m.CanPlay(mv); err != nil {
		return err
	}
	if _, err := m.cur.Drop(int(mv.Column), mv.Seat); err != nil {
		return err
	}
	if !m.cur.Over() {
		m.turn = Other(m.turn)
		return nil
	}

	winner, decided := m.cur.Winner()
	m.results = append(m.results, Result{Winner: winner, Decided: decided, Moves: m.cur.Moves()})
	if s := m.Score(); s[0] >= Wins || s[1] >= Wins || len(m.results) >= Boards {
		m.done = true
		return nil
	}

	m.index++
	m.cur = board.New()
	turn, err := Opener(m.index, m.creator, m.results)
	if err != nil {
		return err
	}
	m.turn = turn
	return nil
}

// Replay runs a whole move list and reports the match it produces.
//
// This is what a third party does with a transcript: the same function both
// peers ran, over moves it verified the signatures of, reaching the outcome
// from the moves alone rather than from anybody's claim about them.
func Replay(creator uint32, moves []Move) (*Match, error) {
	m, err := New(creator)
	if err != nil {
		return nil, err
	}
	for i, mv := range moves {
		if err := m.Play(mv); err != nil {
			return nil, fmt.Errorf("move %d: %w", i, err)
		}
	}
	return m, nil
}
