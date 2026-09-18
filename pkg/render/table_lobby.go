package render

import (
	"fmt"

	"github.com/karamble/dcr4inarow/internal/tablelobby"
)

// The seating screen's geometry. Exported where the client hit-tests it, so
// drawing and input cannot hold different opinions about where a control is.
const (
	railY     = 104.0
	railStepW = (Width - 80) / 6.0

	seatY = 190.0
	seatH = 150.0
	seatW = (Width - 80 - 340 - 24) / 2.0

	termsX = 80 + 2*seatW + 24
	termsW = Width - termsX - 40

	stepY = 364.0
	stepH = 118.0
)

// SeatCardRect is where seat i's card is drawn.
func SeatCardRect(i int) (x, y, w, h float64) {
	return 40 + float64(i)*(seatW+16), seatY, seatW, seatH
}

// FundRect is the stake-funding action, and LobbyBackRect returns to the
// opening screen.
func FundRect() (x, y, w, h float64) { return termsX, Height - 96, termsW, 40 }
func LobbyBackRect() (x, y, w, h float64) {
	return 40, Height - 96, 180, 40
}

// DrawTableLobby draws a table while it is being built.
func DrawTableLobby(c Canvas, v tablelobby.View) {
	c.Rect(0, 0, Width, Height, Navy)
	drawEmblem(c, 40, 20, 40)
	c.Text("dcr4inarow", 92, 28, 20, Text)
	if v.Demo {
		c.Text("FIXTURE · NO WALLET, NO PEER, NO MONEY", 260, 32, 12, Warn)
	} else if v.Match != "" {
		c.Text("table "+short(v.Match), 260, 32, 12, Muted)
	}
	c.Line(40, 76, Width-40, 76, 1, Panel)

	drawRail(c, v)
	for i := range v.Seats {
		if i >= 2 {
			break
		}
		drawSeatCard(c, v, i)
	}
	drawTerms(c, v)
	drawNextStep(c, v)

	// Actions.
	x, y, w, h := LobbyBackRect()
	drawButton(c, x, y, w, h, "< BACK", Panel, Text)
	fx, fy, fw, fh := FundRect()
	label, fill, ink := "WAITING FOR VERIFICATION", Panel, Muted
	switch {
	case v.Stage == tablelobby.StageReady:
		label, fill, ink = "ENTER · PLAY", Turquoise, Navy
	case v.CanFund:
		label, fill, ink = "F · REQUEST STAKE FUNDING", Turquoise, Navy
	}
	drawButton(c, fx, fy, fw, fh, label, fill, ink)
}

// drawRail is the numbered progress bar: done, current, and not yet.
func drawRail(c Canvas, v tablelobby.View) {
	for i, step := range tablelobby.Steps {
		x := 40 + float64(i)*railStepW
		colour := Muted
		switch {
		case v.Stale && i > 0:
		case v.Stage == tablelobby.StageClosed:
		case step < v.Stage:
			colour = Turquoise
		case step == v.Stage:
			colour = Warn
		}
		c.Rect(x, railY, railStepW-16, 3, colour)
		c.Circle(x+9, railY+26, 9, colour)
		c.Text(fmt.Sprint(i+1), x+6, railY+18, 12, Navy)
		c.Text(step.Label(), x+26, railY+18, 11, colour)
	}
	note := "SEATING"
	switch {
	case v.Stale:
		note = "EVIDENCE OUT OF DATE · NOTHING RECHECKED"
	case v.Stage == tablelobby.StageClosed:
		note = "TABLE CLOSED · DEPOSITS REMAIN RECOVERABLE"
	case v.Stage == tablelobby.StageReady:
		note = "ALL DEPOSITS VERIFIED ON CHAIN"
	}
	c.Text(note, 40, railY+46, 11, Muted)
}

func drawSeatCard(c Canvas, v tablelobby.View, i int) {
	s := v.Seats[i]
	x, y, w, h := SeatCardRect(i)
	c.Rect(x, y, w, h, Panel)
	c.Rect(x, y, 4, h, SeatColour(s.Number))

	c.Text(fmt.Sprintf("SEAT %02d", s.Number), x+20, y+14, 11, Muted)
	if s.You {
		c.Text("YOU", x+w-52, y+14, 11, Turquoise)
	}
	c.Text(s.Name, x+20, y+32, 19, Text)

	status, ink := s.StatusAt(v.Stale), Warn
	if !v.Stale && status == "ready" {
		ink = Turquoise
	}
	c.Text(status, x+20, y+62, 12, ink)

	// One dot per deposit this game has: entry, then stake.
	for j, purpose := range []string{tablelobby.PurposeSeatBond, tablelobby.PurposeStake} {
		dx := x + 20 + float64(j)*(w-40)/2
		d, ok := s.Deposit(purpose)
		colour := Muted
		switch {
		case v.Stale:
		case !ok:
		case d.Wrong():
			colour = Warn
		case d.Complete():
			colour = Turquoise
		default:
			colour = Warn
		}
		c.Circle(dx+4, y+100, 4, colour)
		c.Text(tablelobby.Name(purpose), dx+16, y+92, 11, colour)
		detail := "not posted"
		if ok {
			detail = d.Short()
			if v.Stale {
				detail = "recheck"
			}
		}
		c.Text(detail, dx+16, y+108, 11, Muted)

		// How far its confirmations have come.
		if p := v.Progress(d); p > 0 {
			c.Rect(dx+16, y+126, ((w-40)/2-32)*p, 3, colour)
		}
	}
}

func drawTerms(c Canvas, v tablelobby.View) {
	x, y, w, h := termsX, seatY, termsW, seatH+stepH+16
	c.Rect(x, y, w, h, Panel)
	c.Text("TABLE TERMS", x+20, y+14, 12, Muted)

	c.Text(tablelobby.DCR(v.Terms.BuyInAtoms)+" DCR", x+20, y+34, 22, Turquoise)
	c.Text("buy-in per seat", x+20, y+62, 11, Muted)

	rows := [][2]string{
		{"Pot", tablelobby.DCR(v.Terms.Pot()) + " DCR"},
		{"Admission bond", tablelobby.DCR(v.Terms.BondAtoms) + " DCR"},
		{"Total per seat", tablelobby.DCR(v.Terms.PerSeat()) + " DCR"},
	}
	for i, r := range rows {
		ty := y + 88 + float64(i)*20
		c.Text(r[0], x+20, ty, 12, Muted)
		c.Text(r[1], x+w-20-float64(len(r[1]))*7, ty, 12, Text)
	}
	c.Line(x+20, y+156, x+w-20, y+156, 1, Well)
	c.Text(fmt.Sprintf("Stake refund: %d blocks", v.Terms.RefundBlocks), x+20, y+166, 11, Muted)
	c.Text(fmt.Sprintf("Bond refund: %d blocks", v.Terms.BondBlocks), x+20, y+184, 11, Muted)
	if left, ok := v.BlocksLeft(); ok {
		c.Text(fmt.Sprintf("Admission closes in %d blocks", left), x+20, y+202, 11, Warn)
	} else if v.Terms.Until > 0 {
		c.Text(fmt.Sprintf("Admission closed at %d", v.Terms.Until), x+20, y+202, 11, Muted)
	}
	c.Text("Winner takes the pot. Bonds come back.", x+20, y+226, 11, Muted)
	c.Text("Payout needs both seats to sign; if it", x+20, y+244, 11, Muted)
	c.Text("fails you reclaim your own stake.", x+20, y+262, 11, Muted)
}

func drawNextStep(c Canvas, v tablelobby.View) {
	x, y, w, h := 40.0, stepY, 2*seatW+16, stepH
	c.Rect(x, y, w, h, Panel)
	c.Rect(x, y, 3, h, Warn)
	c.Text("NEXT STEP", x+20, y+14, 11, Muted)
	c.Text(v.NextStep, x+20, y+34, 19, Text)
	c.Text(v.NextDetail, x+20, y+68, 12, Muted)
	if v.Error != "" {
		c.Text(v.Error, x+20, y+86, 11, Warn)
	}
}
