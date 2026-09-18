package audit_test

import (
	"testing"

	"github.com/decred/dcrd/dcrec/secp256k1/v4"
	"github.com/karamble/dcr4inarow/internal/audit"
	"github.com/karamble/dcr4inarow/internal/match"
	"github.com/karamble/dcr4inarow/internal/movelog"
	"github.com/karamble/dcrgaming-sdk/pkg/forfeit"
)

const matchID = "3f0c7a1e55d9b84406e2c1fd7ab399215c6e80d4488f13ba0dd5e97c22461af8"

// openerWinsIn7 is a vertical win for whoever opens the board.
var openerWinsIn7 = []uint8{0, 1, 0, 1, 0, 1, 0}

func stand(t *testing.T) (*movelog.Chain, [2]*forfeit.LogKey, movelog.Roster) {
	t.Helper()
	var keys [2]*forfeit.LogKey
	for i := range keys {
		p, err := secp256k1.GeneratePrivateKey()
		if err != nil {
			t.Fatalf("generate key: %v", err)
		}
		k, err := forfeit.LogKeyFrom(p, matchID)
		if err != nil {
			t.Fatalf("log key: %v", err)
		}
		b, err := movelog.OpenBook(t.TempDir() + "/book.jsonl")
		if err != nil {
			t.Fatalf("open book: %v", err)
		}
		t.Cleanup(func() { b.Close() })
		k.Remember(b)
		keys[i] = k
	}
	roster := movelog.Roster{
		0: keys[0].Public().SerializeCompressed(),
		1: keys[1].Public().SerializeCompressed(),
	}
	c, err := movelog.NewChain(matchID, roster)
	if err != nil {
		t.Fatalf("new chain: %v", err)
	}
	return c, keys, roster
}

// playBoard drives the rules and the log together, the way a peer does: the
// rules say whose move it is, the log signs it, and neither is asked to trust
// the other.
func playBoard(t *testing.T, c *movelog.Chain, keys [2]*forfeit.LogKey, m *match.Match, cols []uint8) {
	t.Helper()
	for i, col := range cols {
		seat, boardIndex := m.Turn(), uint8(m.Index())
		if _, err := c.Move(keys[seat], seat, boardIndex, col, 100); err != nil {
			t.Fatalf("sign move %d: %v", i, err)
		}
		if err := m.Play(match.Move{Board: boardIndex, Seat: seat, Column: col}); err != nil {
			t.Fatalf("play move %d: %v", i, err)
		}
	}
}

func TestAWholeMatchVerifiesFromItsTranscript(t *testing.T) {
	c, keys, roster := stand(t)
	m, err := match.New(0)
	if err != nil {
		t.Fatalf("new match: %v", err)
	}
	playBoard(t, c, keys, m, openerWinsIn7) // seat 0 takes its own board
	playBoard(t, c, keys, m, openerWinsIn7) // seat 1 takes its own board
	playBoard(t, c, keys, m, openerWinsIn7) // the decider, opened by the creator
	if !m.Done() {
		t.Fatal("three boards did not finish the match")
	}

	blob, err := c.Marshal()
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}

	// Everything a stranger gets: the transcript, the roster the escrow
	// committed to, and which seat created the table.
	audited, err := audit.Match(blob, roster, 0)
	if err != nil {
		t.Fatalf("audit: %v", err)
	}
	winner, won, done := audited.Outcome()
	if !done || !won || winner != 0 {
		t.Fatalf("the audit reached (%d, %v, %v), want seat 0 paid", winner, won, done)
	}
	if got, want := len(audited.Results()), len(m.Results()); got != want {
		t.Fatalf("the audit saw %d boards, the players played %d", got, want)
	}
}

// A transcript can be perfectly signed and still describe a move nobody was
// entitled to make. The signatures are not the rules.
func TestASignedMoveOutOfTurnIsStillRefused(t *testing.T) {
	c, keys, roster := stand(t)
	for i := 0; i < 2; i++ {
		if _, err := c.Move(keys[0], 0, 0, uint8(i), 100); err != nil {
			t.Fatalf("sign move %d: %v", i, err)
		}
	}
	blob, err := c.Marshal()
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	if _, err := audit.Match(blob, roster, 0); err == nil {
		t.Fatal("a seat moved twice in a row and the audit accepted it")
	}
}

// The roster has to come from the escrow. Checking a transcript against seats
// swapped round is checking it against the wrong agreement.
func TestTheWrongRosterDoesNotVerify(t *testing.T) {
	c, keys, _ := stand(t)
	if _, err := c.Move(keys[0], 0, 0, 3, 100); err != nil {
		t.Fatalf("sign move: %v", err)
	}
	blob, err := c.Marshal()
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	swapped := movelog.Roster{
		0: keys[1].Public().SerializeCompressed(),
		1: keys[0].Public().SerializeCompressed(),
	}
	if _, err := audit.Match(blob, swapped, 0); err == nil {
		t.Fatal("a transcript verified against a roster that seats the players the other way round")
	}
}
