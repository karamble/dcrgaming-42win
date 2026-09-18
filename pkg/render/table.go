package render

import (
	"fmt"

	"github.com/karamble/dcr4inarow/internal/board"
)

// View is everything a table screen draws.
//
// A copy, always. It is filled from the game under its own lock and handed over
// detached, so drawing can never see a half-applied move and never has to take
// a lock the runtime is waiting on.
type View struct {
	MatchID   string
	Grid      board.Board
	Seat      uint32
	Board     int
	Turn      uint32
	Score     [2]int
	Moves     int
	Done      bool
	Won       bool
	Winner    uint32
	Connected bool
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
func AbandonRect() (x, y, w, h float64) { return PanelX, Height - 96, PanelW, 36 }

// DrawTable draws the whole screen.
func DrawTable(c Canvas, v View) {
	c.Rect(0, 0, Width, Height, Navy)
	drawHeader(c, v)
	drawBoard(c, v)
	drawPanel(c, v)
	drawFooter(c, v)
}

func drawHeader(c Canvas, v View) {
	c.Rect(0, 0, Width, 64, Panel)
	c.Text("dcr4inarow", 40, 20, 24, Text)

	dot, label := Muted, "offline"
	if v.Connected {
		dot, label = Turquoise, "bridge connected"
	}
	c.Circle(Width-40-float64(len(label))*7-18, 32, 5, dot)
	c.Text(label, Width-40-float64(len(label))*7, 24, 13, Muted)

	if v.MatchID != "" {
		c.Text("table "+short(v.MatchID), 210, 26, 13, Muted)
	}
}

func drawBoard(c Canvas, v View) {
	c.Rect(BoardX, BoardY, BoardW, BoardH, Panel)
	// The column under the pointer, lit after the board and before the discs
	// so it reads as a lit column rather than a rectangle over the pieces.
	if v.Hover >= 0 && v.Hover < board.Cols && !v.Done {
		c.Rect(BoardX+float64(v.Hover)*Cell, BoardY, Cell, BoardH, Well)
	}

	for col := 0; col < board.Cols; col++ {
		for row := 0; row < board.Rows; row++ {
			x, y := centre(col, row)
			cell := v.Grid.At(col, row)
			if seat, taken := cell.Seat(); taken {
				c.Circle(x, y, Hole, SeatColour(seat))
				continue
			}
			c.Circle(x, y, Hole, Hollow)
		}
	}

	// A ghost of where this player's disc would land.
	if v.Hover >= 0 && v.Hover < board.Cols && !v.Done && v.Turn == v.Seat {
		if row, ok := landing(v.Grid, v.Hover); ok {
			x, y := centre(v.Hover, row)
			c.Circle(x, y, Hole-6, SeatColour(v.Seat))
		}
	}
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
	x, y := PanelX, BoardY
	c.Rect(x, y, PanelW, BoardH, Panel)

	c.Text(fmt.Sprintf("BOARD %d OF %d", v.Board+1, 3), x+20, y+18, 13, Muted)

	for seat := uint32(0); seat < 2; seat++ {
		top := y + 52 + float64(seat)*76
		c.Circle(x+40, top+18, 14, SeatColour(seat))
		name := fmt.Sprintf("seat %d", seat)
		if seat == v.Seat {
			name += "  (you)"
		}
		c.Text(name, x+68, top+8, 15, Text)
		c.Text(fmt.Sprintf("%d of 2 boards", v.Score[seat]), x+68, top+28, 13, Muted)
		if !v.Done && v.Turn == seat {
			c.Rect(x, top-4, 4, 44, SeatColour(seat))
		}
	}

	line := y + 210
	c.Line(x+20, line, x+PanelW-20, line, 1, Well)
	c.Text("OPENS", x+20, line+16, 12, Muted)
	c.Text("board 1  seat 0", x+20, line+34, 13, Text)
	c.Text("board 2  seat 1", x+20, line+54, 13, Text)
	c.Text("board 3  fewer moves", x+20, line+74, 13, Text)

	if v.Stake > 0 {
		c.Text("STAKE", x+20, line+108, 12, Muted)
		c.Text(fmt.Sprintf("%s DCR each", dcr(v.Stake)), x+20, line+126, 13, Text)
	}
	c.Text(fmt.Sprintf("%d moves signed", v.Moves), x+20, y+BoardH-30, 12, Muted)

	// How long the seat to move has left, which is the only clock in this
	// game and is measured in blocks rather than minutes.
	if v.Deadline > 0 && !v.Done && v.Turn != v.Seat {
		left := int64(v.Deadline) - int64(v.Height)
		note, ink := fmt.Sprintf("%d blocks to move", left), Muted
		if left <= 0 {
			note, ink = "overdue", Warn
		}
		c.Text(note, x+20, y+BoardH-52, 12, ink)
	}
}

func drawFooter(c Canvas, v View) {
	if v.CanAbandon && !v.Abandoned {
		x, y, w, h := AbandonRect()
		drawButton(c, x, y, w, h, "END THE MATCH  ·  VOID", Warn, Navy)
	}
	status := v.Status
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
	c.Text(status, 40, Height-64, 16, Text)
	if v.Notice != "" {
		c.Text(v.Notice, 40, Height-38, 12, Warn)
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
