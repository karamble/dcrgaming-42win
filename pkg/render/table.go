package render

import (
	"fmt"
	"image/color"

	"github.com/karamble/dcrgaming-42win/assets/art"
	"github.com/karamble/dcrgaming-42win/internal/board"
	"github.com/karamble/dcrgaming-42win/internal/match"
	"github.com/karamble/dcrgaming-42win/internal/movelog"
)

// View is everything a table screen draws.
//
// A copy, always. It is filled from the game under its own lock and handed over
// detached, so drawing can never see a half-applied move and never has to take
// a lock the runtime is waiting on.
type View struct {
	Network                        string
	Entries                        []movelog.Entry
	Results                        []match.Result
	StakeCheck, BondCheck, Blocked string
	Bond                           int64
	Stale, Pending, Expired        bool
	MatchDeadline                  uint32
	RoundMessage                   string
	LastCol, LastRow               int
	HasLast                        bool
	Drop                           *Drop
	MatchID                        string
	Grid                           board.Board
	Seat                           uint32
	Board                          int
	Turn                           uint32
	Score                          [2]int
	Moves                          int
	Done                           bool
	Won                            bool
	Winner                         uint32
	Connected                      bool
	// Hover is the column the pointer is over, or -1.
	Hover int
	// Status is one line of what is happening, and Notice a warning that
	// outranks it - the payout disclosure, or a lost connection.
	Status string
	Notice string
	// Stake is what each seat put in, in atoms.
	Stake int64
	// Abandoned says the match was given up on, and settles void. Stalled is
	// the seat said to have stopped.
	Abandoned bool
	Stalled   uint32
	// Deadline is the height by which the seat to move owes one, and Height
	// where the chain stands. Both zero before the first move.
	Deadline uint32
	Height   uint32
	// CanAbandon offers the way out once that deadline has passed.
	CanAbandon bool
}

// AbandonRect is the button that gives up on a stalled match.
func AbandonRect() (x, y, w, h float64) { return PanelX, Height - 88, PanelW, 34 }

func TableBackRect() (x, y, w, h float64) { return 32, 20, 96, 34 }
func ReceiptRect() (x, y, w, h float64)   { return PanelX + 16, BoardY + BoardH - 48, PanelW - 32, 34 }

type Drop struct {
	Col, Row int
	Seat     uint32
	Progress float64
}

// DrawTable draws the whole screen.
func DrawTable(c Canvas, v View) {
	c.Rect(0, 0, Width, Height, Navy)
	if bg := art.Tabletop(); bg != nil {
		c.Image(bg, 0, 0, Width, Height)
		c.Rect(0, 0, Width, Height, color.RGBA{7, 16, 32, 175})
	}
	drawHeader(c, v)
	drawBoard(c, v)
	drawPanel(c, v)
	drawFooter(c, v)
}

func drawHeader(c Canvas, v View) {
	c.Rect(0, 0, Width, 74, Panel)
	x, y, w, h := TableBackRect()
	drawButton(c, x, y, w, h, "‹ Lobby", Well, Text)
	drawWordmark(c, 146, 12, 188, 48)

	dot, label := Muted, "offline"
	if v.Connected {
		dot, label = Turquoise, "bridge connected"
	}
	c.Circle(Width-40-float64(len(label))*7-18, 32, 5, dot)
	c.Text(label, Width-40-float64(len(label))*7, 24, 13, Muted)

	if v.MatchID != "" {
		c.Text(upper(v.Network)+"  /  "+short(v.MatchID), 358, 29, 12, Muted)
	}
}

func drawBoard(c Canvas, v View) {
	rounded(c, BoardX-10, BoardY-10, BoardW+20, BoardH+26, 18, Hollow)
	rounded(c, BoardX-10, BoardY-14, BoardW+20, BoardH+20, 18, color.RGBA{39, 66, 89, 255})
	rounded(c, BoardX-6, BoardY-10, BoardW+12, BoardH+12, 14, Panel)
	// The column under the pointer, lit after the board and before the discs
	// so it reads as a lit column rather than a rectangle over the pieces.
	if v.Hover >= 0 && v.Hover < board.Cols && !v.Done && v.Turn == v.Seat && !v.Pending && !v.Stale && v.RoundMessage == "" {
		c.Rect(BoardX+float64(v.Hover)*Cell, BoardY, Cell, BoardH, Well)
	}

	for col := 0; col < board.Cols; col++ {
		for row := 0; row < board.Rows; row++ {
			x, y := centre(col, row)
			cell := v.Grid.At(col, row)
			c.Circle(x, y+2, Hole+2, color.RGBA{48, 70, 91, 255})
			c.Circle(x, y, Hole, Hollow)
			if seat, taken := cell.Seat(); taken {
				if v.Drop == nil || v.Drop.Col != col || v.Drop.Row != row {
					disc(c, x, y, Hole-2, seat)
				}
				continue
			}
			c.Circle(x, y, Hole, Hollow)
		}
	}

	// A ghost of where this player's disc would land.
	if v.Hover >= 0 && v.Hover < board.Cols && !v.Done && v.Turn == v.Seat && !v.Pending && !v.Stale && v.RoundMessage == "" && v.Drop == nil {
		if row, ok := landing(v.Grid, v.Hover); ok {
			x, y := centre(v.Hover, row)
			ring(c, x, y, Hole-7, 2, SeatColour(v.Seat))
			c.Circle(x, y, 3, SeatColour(v.Seat))
		}
	}
	if v.HasLast && v.Drop == nil {
		x, y := centre(v.LastCol, v.LastRow)
		ring(c, x, y, Hole+3, 2, Text)
	}
	if v.Drop != nil {
		d := v.Drop
		x, y := centre(d.Col, d.Row)
		start := BoardY - Hole
		disc(c, x, start+(y-start)*d.Progress*d.Progress, Hole-2, d.Seat)
	}
	if cells := winning(v.Grid); len(cells) == 4 {
		for _, p := range cells {
			x, y := centre(p[0], p[1])
			ring(c, x, y, Hole+3, 3, Turquoise)
		}
		x0, y0 := centre(cells[0][0], cells[0][1])
		x1, y1 := centre(cells[3][0], cells[3][1])
		c.Line(x0, y0, x1, y1, 3, Text)
	}
	for col := 0; col < 7; col++ {
		x, _ := centre(col, 0)
		c.Text(fmt.Sprint(col+1), x-4, BoardY+BoardH+20, 12, Muted)
	}
}

func disc(c Canvas, x, y, r float64, seat uint32) {
	ink := SeatColour(seat)
	c.Circle(x, y+3, r, shade(ink, .35))
	c.Circle(x, y-1, r, ink)
	c.Circle(x, y, r*.77, shade(ink, .85))
	ring(c, x, y-1, r*.78, 1.5, shade(ink, .55))
	if seat == 0 {
		ring(c, x, y, 5, 2, Text)
	} else {
		c.Line(x-5, y-5, x+5, y+5, 2, Text)
		c.Line(x-5, y+5, x+5, y-5, 2, Text)
	}
	c.Line(x-r*.36, y-r*.6, x+r*.25, y-r*.6, 2, color.RGBA{230, 255, 255, 130})
}

func winning(g board.Board) [][2]int {
	for x := 0; x < 7; x++ {
		for y := 0; y < 6; y++ {
			if _, ok := g.At(x, y).Seat(); !ok {
				continue
			}
			for _, d := range [][2]int{{1, 0}, {0, 1}, {1, 1}, {1, -1}} {
				var line [][2]int
				for n := 0; n < 4; n++ {
					a, b := x+n*d[0], y+n*d[1]
					if a < 0 || a >= 7 || b < 0 || b >= 6 || g.At(a, b) != g.At(x, y) {
						break
					}
					line = append(line, [2]int{a, b})
				}
				if len(line) == 4 {
					return line
				}
			}
		}
	}
	return nil
}

// landing is the row a disc dropped into this column would come to rest in.
func landing(g board.Board, col int) (int, bool) {
	for row := 0; row < board.Rows; row++ {
		if g.At(col, row) == board.Empty {
			return row, true
		}
	}
	return 0, false
}

func drawPanel(c Canvas, v View) {
	x, y := PanelX, BoardY-10
	rounded(c, x, y, PanelW, BoardH+10, 14, Panel)
	c.Text(fmt.Sprintf("ROUND %d / 3 · FIRST TO TWO", v.Board+1), x+20, y+16, 12, Muted)
	status, ink := "Your turn", Turquoise
	if v.Turn != v.Seat {
		status, ink = "Opponent's turn", Text
	}
	if v.Pending {
		status, ink = "Submitting move…", Warn
	}
	if v.Done {
		status, ink = "Match complete", Text
		if v.Won {
			if v.Winner == v.Seat {
				status = "You win!"
			} else {
				status = "Opponent wins"
			}
		} else {
			status = "Match void"
		}
	}
	if v.Stale {
		status, ink = "Connection unavailable", Warn
	}
	if v.RoundMessage != "" {
		status = v.RoundMessage
	}
	c.Text(fit(status, PanelW-40, 22), x+20, y+39, 22, ink)
	for seat := uint32(0); seat < 2; seat++ {
		top := y + 86 + float64(seat)*62
		disc(c, x+35, top+15, 13, seat)
		name := "Opponent"
		if seat == v.Seat {
			name = "You"
		}
		bold(c, name, x+59, top, 16, Text)
		c.Text(fmt.Sprintf("seat %d", seat), x+59, top+22, 11, Muted)
		c.Text(fmt.Sprint(v.Score[seat]), x+PanelW-50, top-5, 30, SeatColour(seat))
	}
	for i := 0; i < 3; i++ {
		px := x + 24 + float64(i)*38
		col := Well
		if i < len(v.Results) && v.Results[i].Decided {
			col = SeatColour(v.Results[i].Winner)
		}
		rounded(c, px, y+218, 28, 7, 3, col)
	}
	c.Text("ROUND WINS", x+144, y+214, 10, Muted)
	c.Line(x+20, y+244, x+PanelW-20, y+244, 1, Well)
	c.Text("GROSS POT", x+20, y+259, 11, Muted)
	c.Text(dcr(2*v.Stake)+" DCR", x+20, y+278, 22, Turquoise)
	c.Text("Your stake  "+dcr(v.Stake)+" DCR", x+20, y+311, 12, Text)
	if v.Done {
		Wrap(c, FinancialStatus(v), x+20, y+338, PanelW-40, 12, Muted, 2)
	} else {
		deadline := "Opens: seat 0, then seat 1"
		if v.Board == 2 {
			deadline = "Decider opens: earned by fewer moves"
		}
		if v.Deadline > 0 {
			deadline = fmt.Sprintf("Move: %d blocks · Match: %d blocks", max(0, int64(v.Deadline)-int64(v.Height)), max(0, int64(v.MatchDeadline)-int64(v.Height)))
		}
		Wrap(c, deadline, x+20, y+338, PanelW-40, 12, Muted, 2)
	}
	if v.Done {
		bx, by, bw, bh := ReceiptRect()
		drawButton(c, bx, by, bw, bh, "Match receipt  →", Turquoise, Navy)
	}
	if BoardH > 510 {
		c.Line(x+20, y+397, x+PanelW-20, y+397, 1, Well)
		c.Text(fmt.Sprintf("%d SIGNED MOVES", v.Moves), x+20, y+411, 11, Muted)
		for i, r := range v.Results {
			outcome := "Draw"
			if r.Decided {
				outcome = "Opponent"
				if r.Winner == v.Seat {
					outcome = "You"
				}
			}
			c.Text(fmt.Sprintf("Round %d   %s   ·   %d moves", i+1, outcome, r.Moves), x+20, y+438+float64(i)*24, 13, Text)
		}
	}
}

// FinancialStatus never promotes a pending spend to confirmed winnings.
func FinancialStatus(v View) string {
	if v.Stale {
		return "Financial evidence out of date. Reconnect to recheck."
	}
	if v.Blocked != "" {
		return "Settlement blocked: " + v.Blocked
	}
	switch v.StakeCheck {
	case "spending":
		return "Stake spend in progress · check dcrpulse"
	case "spent":
		return "Stake reported spent · see dcrpulse for payment details"
	case "unavailable":
		return "Financial evidence unavailable · check dcrpulse"
	case "missing", "mismatch":
		return "Stake could not be verified · review dcrpulse"
	case "", "unchecked":
		return "Awaiting current financial evidence"
	}
	if v.Done {
		return "Settlement pending · check approvals in dcrpulse"
	}
	return "Stake locked for this match"
}

func drawFooter(c Canvas, v View) {
	if v.CanAbandon && !v.Abandoned {
		x, y, w, h := AbandonRect()
		drawButton(c, x, y, w, h, "END THE MATCH  ·  VOID", Warn, Navy)
	}
	status := v.Status
	if v.RoundMessage != "" {
		opener := "Opponent opens next"
		if v.Turn == v.Seat {
			opener = "You open next"
		}
		status = v.RoundMessage + " · " + opener
	}
	if status == "" {
		switch {
		case v.Abandoned:
			status = fmt.Sprintf("seat %d stopped playing · the match is void", v.Stalled)
		case v.Done && v.Won:
			status = fmt.Sprintf("seat %d wins the match", v.Winner)
		case v.Done:
			status = "no result: every seat takes back its own stake"
		case v.Turn == v.Seat:
			status = "your move"
		default:
			status = fmt.Sprintf("waiting for seat %d", v.Turn)
		}
	}
	Wrap(c, status, 40, Height-77, PanelX-64, 15, Text, 2)
	if v.Notice != "" {
		Wrap(c, v.Notice, 40, Height-30, Width-80, 11, Warn, 1)
	}
}

// dcr renders atoms as a decimal amount. Exact: atoms are integers and a float
// would round somebody's stake.
func dcr(atoms int64) string {
	whole, frac := atoms/1e8, atoms%1e8
	return fmt.Sprintf("%d.%08d", whole, frac)
}

func short(id string) string {
	if len(id) <= 12 {
		return id
	}
	return id[:12]
}
