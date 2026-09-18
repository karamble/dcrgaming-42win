package tablelobby

import "testing"

func verified(purpose string, required int64) Deposit {
	return Deposit{
		Purpose: purpose, Confirmations: required, RequiredConfirmations: required,
		Check: CheckVerified, Local: true,
	}
}

// Completeness is a conjunction, and every term has to be able to fail it on
// its own. A screen that reports money as confirmed when it is not is the one
// thing this model must never do.
func TestADepositIsOnlyCompleteWhenEveryTermHolds(t *testing.T) {
	good := verified(PurposeSeatBond, 2)
	if !good.Complete() {
		t.Fatal("a verified, locally checked, fully confirmed deposit is not complete")
	}
	for name, mut := range map[string]func(*Deposit){
		"not verified":      func(d *Deposit) { d.Check = CheckConfirming },
		"not checked here":  func(d *Deposit) { d.Local = false },
		"no depth policy":   func(d *Deposit) { d.RequiredConfirmations = 0 },
		"short of depth":    func(d *Deposit) { d.Confirmations = 1 },
		"missing entirely":  func(d *Deposit) { d.Check = CheckMissing },
		"script mismatched": func(d *Deposit) { d.Check = CheckMismatch },
	} {
		t.Run(name, func(t *testing.T) {
			d := good
			mut(&d)
			if d.Complete() {
				t.Fatalf("%s still counted as complete", name)
			}
		})
	}
}

// A peer saying its money is down is not evidence that it is.
func TestAnnouncedEvidenceIsNotLocalEvidence(t *testing.T) {
	announced := verified(PurposeStake, 1)
	announced.Local = false
	if announced.Complete() {
		t.Fatal("a deposit this peer never checked itself counted as complete")
	}
	if got := announced.Short(); got == "verified" {
		t.Fatal("an unchecked deposit is displayed as verified")
	}
}

func TestSeatStatusNamesWhatItIsWaitingFor(t *testing.T) {
	var s Seat
	if got := s.Status(); got != "waiting for entry" {
		t.Fatalf("a seat with nothing posted says %q", got)
	}
	s.Deposits = []Deposit{{
		Purpose: PurposeSeatBond, Check: CheckConfirming, Local: true,
		Confirmations: 1, RequiredConfirmations: 2,
	}}
	if got := s.Status(); got != "entry confirming · 1 of 2" {
		t.Fatalf("a confirming bond says %q", got)
	}
	s.Deposits = []Deposit{verified(PurposeSeatBond, 2)}
	if got := s.Status(); got != "waiting for stake" {
		t.Fatalf("a seat past its bond says %q", got)
	}
	s.Deposits = append(s.Deposits, verified(PurposeStake, 1))
	if got := s.Status(); got != "ready" {
		t.Fatalf("a fully funded seat says %q", got)
	}
}

// Checks made before the bridge went away are not checks made now.
func TestStaleEvidenceReportsNothingAsCurrent(t *testing.T) {
	v := Fixture(StageReady)
	v.Stale = true
	if got := v.Seats[0].StatusAt(v.Stale); got != "recheck required" {
		t.Fatalf("a stale seat says %q", got)
	}
	if p := v.Progress(v.Seats[0].Deposits[0]); p != 0 {
		t.Fatalf("a stale deposit shows progress %v", p)
	}
	step, _ := Guidance(v)
	if step != "Evidence is out of date" {
		t.Fatalf("stale guidance says %q", step)
	}
}

// Every stage has to say something, and anywhere money is outstanding it has
// to say not to pay twice.
func TestEveryStageGuidesAndWarnsWherePaymentIsPending(t *testing.T) {
	for _, stage := range append(append([]Stage{}, Steps...), StageClosed) {
		v := Fixture(stage)
		step, detail := Guidance(v)
		if step == "" || detail == "" {
			t.Fatalf("stage %s guides with %q / %q", stage.Label(), step, detail)
		}
		if stage == StageAdmission || stage == StageStake {
			if !contains(detail, "must not be submitted again") && !contains(detail, "asked for once") {
				t.Fatalf("stage %s does not warn against paying twice: %q", stage.Label(), detail)
			}
		}
	}
}

func contains(s, sub string) bool {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}

func TestTermsArithmeticIsExact(t *testing.T) {
	terms := Terms{BuyInAtoms: 100_000, BondAtoms: 1_000_000, Seats: 2}
	if got := terms.Pot(); got != 200_000 {
		t.Fatalf("pot is %d", got)
	}
	if got := terms.PerSeat(); got != 1_100_000 {
		t.Fatalf("per seat is %d", got)
	}
	if got := DCR(1_100_000); got != "0.01100000" {
		t.Fatalf("DCR renders %q", got)
	}
	if got := DCR(0); got != "0.00000000" {
		t.Fatalf("zero renders %q", got)
	}
}

func TestAdmissionDeadlineCountsDownAndThenStops(t *testing.T) {
	v := Fixture(StageAdmission)
	left, ok := v.BlocksLeft()
	if !ok || left != v.Terms.Until-v.Height {
		t.Fatalf("blocks left is %d (%v)", left, ok)
	}
	v.Height = v.Terms.Until
	if _, ok := v.BlocksLeft(); ok {
		t.Fatal("a closed admission window still reports time remaining")
	}
}

func TestTheRailCoversEveryStageButClosed(t *testing.T) {
	if len(Steps) != 6 {
		t.Fatalf("the rail has %d steps", len(Steps))
	}
	for _, s := range Steps {
		if s.Label() == "UNKNOWN" {
			t.Fatalf("stage %d has no label", s)
		}
	}
	if StageClosed.Label() != "CLOSED" {
		t.Fatal("a closed table has no label")
	}
}
