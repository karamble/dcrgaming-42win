package tablelobby

import "fmt"

// Confirmation depths the SDK's escrow asks for. Mirrored here so a fixture
// shows the same numbers a live table does.
const (
	BondConfirmations  int64 = 2
	StakeConfirmations int64 = 1
)

// Fixture is a fictional table at a given stage.
//
// It exists so every state of this screen can be looked at without a bridge, a
// wallet or a peer - including the states that are awkward to reach on purpose,
// like a bond stuck one confirmation short or a table that aborted. A state
// nobody can render is a state nobody reviews.
func Fixture(stage Stage) View {
	terms := Terms{
		BuyInAtoms:   100_000,
		BondAtoms:    1_000_000,
		Seats:        2,
		RefundBlocks: 288,
		BondBlocks:   288,
		Until:        1_116_400,
	}
	v := View{
		Match: "641be69ec6100361ff3e35357355c9d1",
		Terms: terms,
		Stage: stage,
		Demo:  true,
		Seats: []Seat{
			{Number: 0, You: true, Name: "You"},
			{Number: 1, Name: "Blue Comet"},
		},
		Height: 1_116_124,
	}

	bond := func(conf int64, check string) Deposit {
		return Deposit{
			Purpose: PurposeSeatBond, Atoms: terms.BondAtoms,
			Confirmations: conf, RequiredConfirmations: BondConfirmations,
			Check: check, Local: check != "",
		}
	}
	stake := func(conf int64, check string) Deposit {
		return Deposit{
			Purpose: PurposeStake, Atoms: terms.BuyInAtoms,
			Confirmations: conf, RequiredConfirmations: StakeConfirmations,
			Check: check, Local: check != "",
		}
	}

	switch stage {
	case StageInvitation:
	case StageAdmission:
		v.Seats[0].Deposits = []Deposit{bond(1, CheckConfirming)}
	case StageRoster:
		v.Seats[0].Deposits = []Deposit{bond(BondConfirmations, CheckVerified)}
	case StageSeatDraw:
		for i := range v.Seats {
			v.Seats[i].Deposits = []Deposit{bond(BondConfirmations, CheckVerified)}
		}
	case StageStake:
		// Both bonds are down; this seat still owes its stake and the other
		// seat's is confirming. That is the state a player is most likely to
		// be looking at, and the one where the screen has to say clearly
		// that the payment is asked for exactly once.
		v.Seats[0].Deposits = []Deposit{bond(BondConfirmations, CheckVerified)}
		v.Seats[1].Deposits = []Deposit{
			bond(BondConfirmations, CheckVerified),
			stake(0, CheckConfirming),
		}
		v.CanFund = true
	case StageReady:
		for i := range v.Seats {
			v.Seats[i].Deposits = []Deposit{
				bond(BondConfirmations, CheckVerified),
				stake(StakeConfirmations, CheckVerified),
			}
		}
	case StageClosed:
		v.Seats[0].Deposits = []Deposit{bond(BondConfirmations, CheckVerified)}
		v.ClosedReason = "admission expired"
	}
	v.NextStep, v.NextDetail = Guidance(v)
	return v
}

// Guidance is what the table is waiting for and what to do about it.
//
// The second line matters as much as the first: the one way to lose money here
// is to treat a pending payment as a failed one and submit it twice, so
// anywhere a payment is outstanding the screen says not to.
func Guidance(v View) (step, detail string) {
	if v.Stale {
		return "Evidence is out of date",
			"The bridge is not connected. Nothing here has been checked since it went away."
	}
	switch v.Stage {
	case StageClosed:
		reason := v.ClosedReason
		if reason == "" {
			reason = "this table will not form"
		}
		return "Table closed · " + reason,
			"Deposits remain recoverable from dcrpulse → Gaming → Recovery, after their locks."
	case StageInvitation:
		return "Waiting for the table to be accepted",
			"Accept the invitation in dcrpulse. Nothing is owed until a seat is taken."
	case StageAdmission:
		return waitingFor(v, PurposeSeatBond, "admission bond"),
			"Approval happens in dcrpulse. A pending payment must not be submitted again."
	case StageRoster:
		return "Waiting for the other seat to join",
			"Both seats post an admission bond before the roster can be agreed."
	case StageSeatDraw:
		return "Waiting for the seat draw",
			fmt.Sprintf("Seats are drawn from the block after admission closes, at %d.", v.Terms.Until+1)
	case StageStake:
		if v.CanFund {
			return "Your stake has not been funded",
				fmt.Sprintf("Press F to request %s DCR. Approve it in dcrpulse; it is asked for once.", DCR(v.Terms.BuyInAtoms))
		}
		return waitingFor(v, PurposeStake, "stake"),
			"Approval happens in dcrpulse. A pending payment must not be submitted again."
	case StageReady:
		return "Ready to play",
			"Payout needs both seats to sign. If it fails, you reclaim your own stake after its lock."
	}
	return "", ""
}

// waitingFor names the seat holding up a purpose, and how far along it is.
func waitingFor(v View, purpose, label string) string {
	for _, s := range v.Seats {
		d, ok := s.Deposit(purpose)
		if ok && d.Complete() {
			continue
		}
		who := fmt.Sprintf("seat %d", s.Number)
		if s.You {
			who = "your"
		} else {
			who += "'s"
		}
		if !ok {
			return "Waiting for " + who + " " + label
		}
		if d.Wrong() {
			return who + " " + label + " needs attention · " + d.Check
		}
		return fmt.Sprintf("Waiting for %s %s · %s confirmations", who, label, d.Short())
	}
	return "Waiting for the " + label
}
