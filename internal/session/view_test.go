package session

import (
	"github.com/decred/dcrd/dcrec/secp256k1/v4"
	"github.com/karamble/dcrgaming-42win/internal/match"
	"github.com/karamble/dcrgaming-42win/internal/movelog"
	"github.com/karamble/dcrgaming-sdk/pkg/membership"
	sdk "github.com/karamble/dcrgaming-sdk/pkg/runtime"
	"testing"
)

func TestViewUsesAcceptedMoneyAndLocalRawSpendState(t *testing.T) {
	key0, _ := secp256k1.GeneratePrivateKey()
	key1, _ := secp256k1.GeneratePrivateKey()
	chain, err := movelog.NewChain("match", movelog.Roster{0: key0.PubKey().SerializeCompressed(), 1: key1.PubKey().SerializeCompressed()})
	if err != nil {
		t.Fatal(err)
	}
	m, _ := match.New(0)
	entry := &table{chain: chain, play: m, seat: 0, paid: true, snap: sdk.TableSnapshot{Record: sdk.TableRecord{Terms: membership.Terms{BuyInAtoms: 100000, BondAtoms: 1000000}}, Deposits: []sdk.DepositStatus{{Seat: 0, Purpose: "stake", Check: "spending"}, {Seat: 1, Purpose: "stake", Check: "spent"}}}}
	g := &Game{tables: map[string]*table{"sid": entry}}
	v, ok := g.View("sid")
	if !ok || v.BuyIn != 100000 || v.Bond != 1000000 || v.StakeCheck != "spending" {
		t.Fatalf("wrong presentation data: %+v", v)
	}
}
