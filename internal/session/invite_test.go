package session_test

import (
	"context"
	"testing"

	"github.com/karamble/dcr4inarow/internal/session"
	"github.com/karamble/dcrgaming-sdk/pkg/gaming/schema"
	"github.com/karamble/dcrgaming-sdk/pkg/membership"
)

func playable() membership.Terms {
	return membership.Terms{
		Game: session.GameID, GameVer: session.GameVer, SID: "abcdef01",
		BuyInAtoms: 100_000, Seats: 2, CSVBlocks: session.RefundBlocks,
		Until: 1_116_400, BondAtoms: 1_000_000, BondLockBlocks: session.BondLockBlocks,
	}
}

func TestATableThisGameCanPlayIsAccepted(t *testing.T) {
	g, err := session.New(t.TempDir())
	if err != nil {
		t.Fatalf("new game: %v", err)
	}
	defer g.Close()
	got, err := g.ResolveInvite(context.Background(), schema.Invite{}, playable())
	if err != nil {
		t.Fatalf("a playable table was refused: %v", err)
	}
	if got != playable() {
		t.Fatal("the terms came back changed; the economics are the operator's")
	}
}

// Accepting an invitation pays the seat bond without asking again, so a table
// that cannot work must be refused here, while refusing is still free.
func TestAnUnplayableTableIsRefusedBeforeTheBondIsPaid(t *testing.T) {
	g, err := session.New(t.TempDir())
	if err != nil {
		t.Fatalf("new game: %v", err)
	}
	defer g.Close()

	for name, mut := range map[string]func(*membership.Terms){
		"three seats":              func(tm *membership.Terms) { tm.Seats = 3 },
		"one seat":                 func(tm *membership.Terms) { tm.Seats = 1 },
		"refund matures mid-match": func(tm *membership.Terms) { tm.CSVBlocks = session.MatchDeadlineBlocks },
		"refund lock too short":    func(tm *membership.Terms) { tm.CSVBlocks = session.MatchDeadlineBlocks + 1 },
		"admission lock too short": func(tm *membership.Terms) { tm.BondLockBlocks = 1 },
		"buy-in below the dust and fee": func(tm *membership.Terms) {
			tm.BuyInAtoms = session.MinBuyInAtoms - 1
		},
		"buy-in above this build's ceiling": func(tm *membership.Terms) {
			tm.BuyInAtoms = session.MaxBuyInAtoms + 1
		},
	} {
		t.Run(name, func(t *testing.T) {
			terms := playable()
			mut(&terms)
			if _, err := g.ResolveInvite(context.Background(), schema.Invite{}, terms); err == nil {
				t.Fatalf("a table with %s was accepted", name)
			}
		})
	}
}
