// Package manifest is the frozen statement of what game is being played.
//
// Two peers about to put money on a table have to be playing the same game, and
// "the same game" is more than the same binary name: the board, the number of
// boards, who opens which one, how long a move may take, and how the pot is
// divided all have to match, or the first disagreement arrives after both
// stakes are locked.
//
// So it is hashed, signed and exchanged before the first move. A manifest that
// differs by one field produces a different hash and the table stops, which is
// cheap. Discovering it mid-match is not.
//
// Pure: no SDK, no chain, no I/O. The hash is a frozen input - changing the
// encoding or the domain tag invalidates every signature ever made over it.
package manifest

import (
	"bytes"
	"encoding/binary"
	"fmt"

	"github.com/decred/dcrd/crypto/blake256"
)

// Version is the manifest format itself. It is covered by the hash, so one
// version's bytes can never be read as another's.
const Version uint16 = 1

// tag domain-separates the manifest hash. Frozen.
var tag = []byte("dcr4inarow/manifest/v1")

// Opener rules, named rather than numbered so a change of scheme cannot be
// mistaken for a change of parameter.
const (
	// OpenerSeatThenEarned is: board 1 to seat 0, board 2 to seat 1, board 3
	// to whoever won its board in fewer moves, ties to seat 0.
	OpenerSeatThenEarned = "seat-then-earned/v1"
)

// Payout rules.
const (
	// PayoutWinnerTakesPot is: the seat with two board wins takes the whole
	// funded pot; anything else returns every seat its own stake.
	PayoutWinnerTakesPot = "winner-takes-pot/v1"
)

// Manifest is everything about the game that both seats must agree.
type Manifest struct {
	Version     uint16
	GameVersion int32
	// The board.
	Cols, Rows, Line uint8
	// The match.
	Boards, Wins uint8
	// The clocks, in blocks. Heights rather than time, because two machines
	// must never read a deadline differently.
	MoveDeadlineBlocks  uint32
	MatchDeadlineBlocks uint32
	// The rules that decide who opens and who is paid.
	OpenerRule string
	PayoutRule string
}

// Validate refuses a manifest that could not describe a playable table.
func (m Manifest) Validate() error {
	switch {
	case m.Version != Version:
		return fmt.Errorf("manifest is version %d, want %d", m.Version, Version)
	case m.GameVersion <= 0:
		return fmt.Errorf("manifest states no game version")
	case m.Cols < m.Line || m.Rows < m.Line || m.Line < 3:
		return fmt.Errorf("a %dx%d board cannot hold a line of %d", m.Cols, m.Rows, m.Line)
	case m.Boards == 0 || m.Wins == 0 || m.Wins > m.Boards:
		return fmt.Errorf("%d wins cannot be taken from %d boards", m.Wins, m.Boards)
	case m.MoveDeadlineBlocks == 0 || m.MatchDeadlineBlocks == 0:
		return fmt.Errorf("manifest states no deadlines")
	case m.MoveDeadlineBlocks >= m.MatchDeadlineBlocks:
		// A single move may not consume the whole match, or the match
		// ceiling is decoration.
		return fmt.Errorf("a move may take %d blocks of a %d-block match",
			m.MoveDeadlineBlocks, m.MatchDeadlineBlocks)
	case m.OpenerRule != OpenerSeatThenEarned:
		return fmt.Errorf("unknown opener rule %q", m.OpenerRule)
	case m.PayoutRule != PayoutWinnerTakesPot:
		return fmt.Errorf("unknown payout rule %q", m.PayoutRule)
	}
	return nil
}

// bytes is the canonical encoding the hash covers.
//
// Fixed field order and fixed integer widths, with the rule names length
// prefixed. Derived from a struct encoder it would be one field reordering away
// from a different hash on a different build.
func (m Manifest) bytes() ([]byte, error) {
	if err := m.Validate(); err != nil {
		return nil, err
	}
	var b bytes.Buffer
	b.Write(tag)
	_ = binary.Write(&b, binary.BigEndian, m.Version)
	_ = binary.Write(&b, binary.BigEndian, m.GameVersion)
	for _, v := range []uint8{m.Cols, m.Rows, m.Line, m.Boards, m.Wins} {
		_ = binary.Write(&b, binary.BigEndian, v)
	}
	_ = binary.Write(&b, binary.BigEndian, m.MoveDeadlineBlocks)
	_ = binary.Write(&b, binary.BigEndian, m.MatchDeadlineBlocks)
	for _, s := range []string{m.OpenerRule, m.PayoutRule} {
		_ = binary.Write(&b, binary.BigEndian, uint32(len(s)))
		b.WriteString(s)
	}
	return b.Bytes(), nil
}

// Hash is what a seat signs to say it is playing this game.
func (m Manifest) Hash() ([32]byte, error) {
	raw, err := m.bytes()
	if err != nil {
		return [32]byte{}, err
	}
	return blake256.Sum256(raw), nil
}

// Same reports whether two manifests are the same agreement.
func (m Manifest) Same(other Manifest) bool { return m == other }
