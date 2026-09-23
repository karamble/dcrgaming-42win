package render

func DrawHelp(c Canvas, network string) {
	c.Rect(0, 0, Width, Height, Navy)
	drawWordmark(c, 40, 12, 196, 44)
	c.Text("Your next match starts in dcrpulse", 40, 70, 30, Text)
	c.Text(upper(network)+" · YOUR WALLET STAYS IN DCRPULSE", 40, 119, 12, Turquoise)
	steps := [][2]string{
		{"01  Connect your bridge", "In dcrpulse → Gaming, register FOUR2WIN using game ID dcr4inarow. Copy the three credentials into Settings and connect using the same network."},
		{"02  Create or accept a table", "Use dcrpulse's table invitation flow. Review the stake, admission bond, fees and locks before approving any payment. Return here to watch both seats become ready."},
		{"03  Approve your stake", "When prompted here, request stake funding once. Approve it in dcrpulse → Gaming; this game shows the chain confirmations and starts when both players are ready."},
		{"04  Finish, settle, or recover", "Payout needs both players to cooperate. Check approvals in dcrpulse. If settlement cannot finish, use Gaming → Recovery after the locks mature, less fees. A stalled opponent does not forfeit their stake."},
	}
	for i, s := range steps {
		y := 166 + float64(i)*95
		c.Text(s[0], 40, y, 18, Turquoise)
		Wrap(c, s[1], 40, y+29, Width-80, 14, Muted, 3)
	}
	c.Text("Enter, Escape, or click to return", 40, Height-40, 13, Text)
}
