package render

import (
	"image/color"
	"strings"
	"testing"

	"github.com/karamble/dcrgaming-42win/internal/board"
	"github.com/karamble/dcrgaming-42win/internal/tablelobby"
)

func TestColumnAtMapsThePointerToAColumn(t *testing.T) {
	for col := 0; col < board.Cols; col++ {
		x := BoardX + float64(col)*Cell + Cell/2
		y := BoardY + BoardH/2
		if got := ColumnAt(x, y); got != col {
			t.Fatalf("the middle of column %d read as %d", col, got)
		}
	}
	for _, p := range [][2]float64{
		{BoardX - 1, BoardY + 10},
		{BoardX + BoardW, BoardY + 10},
		{BoardX + 10, BoardY - 1},
		{BoardX + 10, BoardY + BoardH},
	} {
		if got := ColumnAt(p[0], p[1]); got != -1 {
			t.Fatalf("a pointer at %v outside the board read as column %d", p, got)
		}
	}
}

func TestTheGhostLandsOnTopOfTheStack(t *testing.T) {
	var g board.Board
	if row, ok := landing(g, 3); !ok || row != 0 {
		t.Fatalf("an empty column lands at row %d (%v), want the floor", row, ok)
	}
	for i := 0; i < 2; i++ {
		if _, err := g.Drop(3, uint32(i)); err != nil {
			t.Fatalf("drop: %v", err)
		}
	}
	if row, ok := landing(g, 3); !ok || row != 2 {
		t.Fatalf("a column holding two lands at row %d (%v), want row 2", row, ok)
	}
	for i := 0; i < board.Rows-2; i++ {
		if _, err := g.Drop(3, 0); err != nil {
			t.Fatalf("drop: %v", err)
		}
	}
	if _, ok := landing(g, 3); ok {
		t.Fatal("a full column still offered a landing row")
	}
}

// Rows count from the floor up, against the screen's own direction. Getting
// this backwards draws every board upside down.
func TestRowZeroIsDrawnAtTheBottom(t *testing.T) {
	_, bottom := centre(0, 0)
	_, top := centre(0, board.Rows-1)
	if bottom <= top {
		t.Fatalf("row 0 is drawn at y=%v and the top row at y=%v", bottom, top)
	}
}

// A screen that renders nothing at all still passes every geometry test, so
// check that something was actually drawn, and in the seats' own colours.
func TestATableIsDrawnInBothSeatsColours(t *testing.T) {
	var g board.Board
	if _, err := g.Drop(0, 0); err != nil {
		t.Fatalf("drop: %v", err)
	}
	if _, err := g.Drop(1, 1); err != nil {
		t.Fatalf("drop: %v", err)
	}
	r := NewRaster()
	DrawTable(r, View{Grid: g, Seat: 0, Turn: 0, Hover: -1, MatchID: "abc", Stake: 5_000_000})

	seen := map[color.RGBA]bool{}
	bounds := r.Target.Bounds()
	for y := bounds.Min.Y; y < bounds.Max.Y; y++ {
		for x := bounds.Min.X; x < bounds.Max.X; x++ {
			c := r.Target.RGBAAt(x, y)
			seen[color.RGBA{c.R, c.G, c.B, 255}] = true
		}
	}
	for _, want := range []color.RGBA{Turquoise, Blue, Navy, Panel} {
		if !seen[want] {
			t.Fatalf("nothing on the screen was %v", want)
		}
	}
}

// The client hit-tests against these rectangles and the panel draws with them.
// Two things that disagree about where a button is put the money behind the
// wrong one.
func TestSettingsControlsSitInsideThePanelAndDoNotOverlap(t *testing.T) {
	type box struct {
		name       string
		x, y, w, h float64
	}
	var boxes []box
	add := func(name string, x, y, w, h float64) { boxes = append(boxes, box{name, x, y, w, h}) }

	for i := 0; i < 5; i++ {
		x, y, w, h := FieldRect(i)
		add("field", x, y, w, h)
	}
	for i := range ButtonLabels {
		x, y, w, h := ButtonRect(i)
		add("button", x, y, w, h)
	}
	for i := 0; i < 3; i++ {
		x, y, w, h := NetworkRect(i)
		add("network", x, y, w, h)
	}
	cx, cy, cw, ch := CloseRect()
	add("close", cx, cy, cw, ch)

	for _, b := range boxes {
		if b.x < panX || b.y < panY || b.x+b.w > panX+panW || b.y+b.h > panY+panH {
			t.Fatalf("%s at %v,%v %vx%v falls outside the panel", b.name, b.x, b.y, b.w, b.h)
		}
	}
	for i := range boxes {
		for j := i + 1; j < len(boxes); j++ {
			a, b := boxes[i], boxes[j]
			if a.x < b.x+b.w && b.x < a.x+a.w && a.y < b.y+b.h && b.y < a.y+a.h {
				t.Fatalf("%s and %s overlap", a.name, b.name)
			}
		}
	}
}

// A private key is shown to nobody, including the person who pasted it.
func TestAMaskedFieldNeverShowsItsValue(t *testing.T) {
	secret := "-----BEGIN EC PRIVATE KEY-----\nMHcCAQEEIMitIsASecret\n-----END EC PRIVATE KEY-----\n"
	shown := display(Field{Value: secret, Masked: true})
	if shown == "" {
		t.Fatal("a pasted key showed nothing at all, so nobody can tell it arrived")
	}
	if strings.Contains(shown, "MHcCAQEEIMitIsASecret") || strings.Contains(shown, "BEGIN") {
		t.Fatalf("a masked field rendered its value: %q", shown)
	}
}

func TestAPastedCertificateIsSummarisedRatherThanDumped(t *testing.T) {
	cert := "-----BEGIN CERTIFICATE-----\n" + strings.Repeat("QUJD", 400) + "\n-----END CERTIFICATE-----\n"
	shown := display(Field{Value: cert})
	if len(shown) > 80 {
		t.Fatalf("a certificate rendered %d characters into one row", len(shown))
	}
	if !strings.Contains(shown, "BEGIN CERTIFICATE") {
		t.Fatalf("the summary does not say what was pasted: %q", shown)
	}
}

// The lobby's own controls, for the same reason.
func TestLobbyControlsAreOnTheScreen(t *testing.T) {
	for name, rect := range map[string]func() (float64, float64, float64, float64){
		"gear": GearRect, "call to action": CallToActionRect,
	} {
		x, y, w, h := rect()
		if x < 0 || y < 0 || x+w > Width || y+h > Height {
			t.Fatalf("the %s is off the screen", name)
		}
		if !Hit(x+w/2, y+h/2, x, y, w, h) {
			t.Fatalf("the middle of the %s does not hit it", name)
		}
		if Hit(x-1, y-1, x, y, w, h) {
			t.Fatalf("a point outside the %s hits it", name)
		}
	}
}

// The seating screen's controls are hit-tested by the client against the same
// rectangles it draws with. Two opinions about where FUND is would put money
// behind the wrong one.
func TestSeatingControlsAreOnTheScreenAndDoNotOverlap(t *testing.T) {
	type box struct {
		name       string
		x, y, w, h float64
	}
	var boxes []box
	for i := 0; i < 2; i++ {
		x, y, w, h := SeatCardRect(i)
		boxes = append(boxes, box{"seat card", x, y, w, h})
	}
	fx, fy, fw, fh := FundRect()
	boxes = append(boxes, box{"fund", fx, fy, fw, fh})
	bx, by, bw, bh := LobbyBackRect()
	boxes = append(boxes, box{"back", bx, by, bw, bh})

	for _, b := range boxes {
		if b.x < 0 || b.y < 0 || b.x+b.w > Width || b.y+b.h > Height {
			t.Fatalf("the %s is off the screen at %v,%v %vx%v", b.name, b.x, b.y, b.w, b.h)
		}
		if !Hit(b.x+b.w/2, b.y+b.h/2, b.x, b.y, b.w, b.h) {
			t.Fatalf("the middle of the %s does not hit it", b.name)
		}
	}
	for i := range boxes {
		for j := i + 1; j < len(boxes); j++ {
			a, b := boxes[i], boxes[j]
			if a.x < b.x+b.w && b.x < a.x+a.w && a.y < b.y+b.h && b.y < a.y+a.h {
				t.Fatalf("%s and %s overlap", a.name, b.name)
			}
		}
	}
}

// Every stage has to draw, and a stale one must not paint anything as done.
func TestEverySeatingStageDraws(t *testing.T) {
	for _, stage := range append(append([]tablelobby.Stage{}, tablelobby.Steps...), tablelobby.StageClosed) {
		v := tablelobby.Fixture(stage)
		r := NewRaster()
		DrawTableLobby(r, v)
		if !drawn(r, Turquoise) && !drawn(r, Warn) && !drawn(r, Muted) {
			t.Fatalf("stage %s drew nothing", stage.Label())
		}
	}

	// Turquoise is also the emblem, the YOU tag and the buy-in figure, so the
	// claim worth testing is not that a stale table has none of it but that
	// going stale visibly takes it away: the rail and every verified dot stop
	// being painted as done.
	ready := tablelobby.Fixture(tablelobby.StageReady)
	fresh := NewRaster()
	DrawTableLobby(fresh, ready)

	ready.Stale = true
	ready.NextStep, ready.NextDetail = tablelobby.Guidance(ready)
	gone := NewRaster()
	DrawTableLobby(gone, ready)

	before, after := count(fresh, Turquoise), count(gone, Turquoise)
	if after >= before {
		t.Fatalf("a stale table paints as much verification as a fresh one (%d vs %d)", after, before)
	}
}

func count(r *Raster, want color.RGBA) int {
	n := 0
	b := r.Target.Bounds()
	for y := b.Min.Y; y < b.Max.Y; y++ {
		for x := b.Min.X; x < b.Max.X; x++ {
			c := r.Target.RGBAAt(x, y)
			if c.R == want.R && c.G == want.G && c.B == want.B {
				n++
			}
		}
	}
	return n
}

func drawn(r *Raster, want color.RGBA) bool { return count(r, want) > 0 }
