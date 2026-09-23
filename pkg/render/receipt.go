package render

import "fmt"

func ExportRect() (x, y, w, h float64) { return Width - 332, Height - 85, 292, 44 }

func DrawReceipt(c Canvas, v View, status string) {
	c.Rect(0, 0, Width, Height, Navy)
	drawHeader(c, v)
	c.Text("Your match receipt", 40, 100, 32, Text)
	c.Text("SIGNED PLAY · OBSERVED FINANCIAL STATE", 40, 147, 12, Turquoise)
	rounded(c, 40, 185, Width-80, Height-295, 14, Panel)
	c.Text(fmt.Sprintf("%d – %d", v.Score[v.Seat], v.Score[1-v.Seat]), 64, 202, 44, Text)
	label := "Match in progress · incomplete transcript"
	if v.Done {
		label = "Match complete"
	}
	if v.Abandoned {
		label = "Match void · signed abandonment"
	}
	if v.Expired {
		label = "Match void · match deadline expired"
	}
	c.Text(label, 64, 262, 16, Text)
	y := 298.0
	for i, r := range v.Results {
		outcome := "Draw"
		if r.Decided {
			outcome = "Opponent won"
			if r.Winner == v.Seat {
				outcome = "You won"
			}
		}
		c.Text(fmt.Sprintf("Round %d    %s    ·    %d moves", i+1, outcome, r.Moves), 64, y, 14, Muted)
		y += 26
	}
	x := Width/2 + 30
	c.Text("Your stake  "+dcr(v.Stake)+" DCR", x, 207, 15, Text)
	c.Text("Admission bond  "+dcr(v.Bond)+" DCR", x, 236, 14, Muted)
	Wrap(c, FinancialStatus(v), x, 276, Width-x-64, 14, Turquoise, 3)
	Wrap(c, "Recovery: dcrpulse → Gaming → Recovery. Locks and fees apply; winnings are not guaranteed.", x, 345, Width-x-64, 12, Muted, 3)
	Wrap(c, "Export includes signed moves and a reference roster. Independent verification requires the trusted escrow roster. Financial observations are not payment confirmation.", 64, Height-186, Width-128, 12, Muted, 3)
	bx, by, bw, bh := ExportRect()
	drawButton(c, bx, by, bw, bh, "Export receipt + transcript", Turquoise, Navy)
	Wrap(c, status, 40, Height-85, Width-400, 12, Muted, 3)
}
