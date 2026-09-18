package render

import (
	"image"
	"image/color"
)

// Lobby is the first screen: what this is, whether a bridge is reachable, and
// the one way forward.
type Lobby struct {
	// Logo and Backdrop are optional artwork. Everything reads correctly
	// without them, because a screen that needs its art to make sense is one
	// that breaks the first time an asset is missing.
	Logo     image.Image
	Backdrop image.Image

	Network   string
	Connected bool
	// Configured says credentials have been pasted, which is what separates
	// "not set up yet" from "set up and refused".
	Configured bool
	Status     string
	// Table is the session id of a table in progress, if there is one.
	Table string
	Build string
}

// Geometry the client hit-tests against. Exported so input and drawing cannot
// drift apart into two different opinions about where a button is.
const (
	gearX, gearY, gearSize = Width - 80.0, 24.0, 44.0
	ctaX, ctaY             = 64.0, 470.0
	ctaW, ctaH             = 300.0, 52.0
)

// GearRect is the settings button.
func GearRect() (x, y, w, h float64) { return gearX, gearY, gearSize, gearSize }

// CallToActionRect is the primary button.
func CallToActionRect() (x, y, w, h float64) { return ctaX, ctaY, ctaW, ctaH }

// Hit reports whether a point is inside a rectangle.
func Hit(px, py, x, y, w, h float64) bool {
	return px >= x && px < x+w && py >= y && py < y+h
}

// DrawLobby draws the opening screen.
func DrawLobby(c Canvas, v Lobby) {
	c.Rect(0, 0, Width, Height, Navy)
	if v.Backdrop != nil {
		c.Image(v.Backdrop, 0, 0, Width, Height)
	}

	// Header.
	if v.Logo != nil {
		c.Image(v.Logo, 40, 20, 300, 68)
	} else {
		drawEmblem(c, 40, 20, 56)
		c.Text("dcr4inarow", 110, 32, 28, Text)
	}
	if v.Build != "" {
		c.Text(v.Build, Width-110-float64(len(v.Build))*7, 36, 12, Muted)
	}
	drawGear(c)
	c.Line(40, 100, Width-40, 100, 1, Panel)

	// Hero.
	c.Text("TWO SEATS.  THREE BOARDS.", 64, 140, 13, Turquoise)
	c.Text("FOUR IN A ROW,", 64, 172, 40, Text)
	c.Text("SIGNED EVERY MOVE.", 64, 224, 40, Text)
	c.Text("Every move is signed and chained. Anyone can replay the match", 64, 292, 15, Muted)
	c.Text("from its transcript. Lie about one move and you publish your key.", 64, 316, 15, Muted)

	// The state that decides what the button does.
	c.Rect(64, 360, 3, 56, Turquoise)
	head, detail := "BRIDGE NOT SET UP", "Paste the credentials dcrpulse issued for this game"
	switch {
	case v.Connected:
		head = "BRIDGE CONNECTED  ·  " + upper(v.Network)
		detail = "Accept an invitation in dcrpulse to take a seat"
	case v.Configured:
		head = "BRIDGE NOT CONNECTED"
		detail = "Credentials are saved. Connect from settings."
	}
	c.Text(head, 84, 366, 13, Text)
	c.Text(detail, 84, 388, 13, Muted)

	// What the game is, in three words each.
	for i, feature := range []string{"BEST OF THREE", "EVERY MOVE SIGNED", "STAKED ON DECRED"} {
		c.Text(feature, 64+float64(i)*240, 432, 12, Muted)
	}

	drawButton(c, ctaX, ctaY, ctaW, ctaH, label(v), Turquoise, Navy)

	// Footer.
	c.Rect(0, Height-44, Width, 44, Panel)
	left := "NO TABLE"
	if v.Table != "" {
		left = "TABLE " + short(v.Table)
	}
	c.Text(left, 40, Height-30, 12, Turquoise)
	c.Text("payout needs both seats to sign", Width/2-100, Height-30, 12, Muted)
	c.Text("DECRED  ·  P2P", Width-140, Height-30, 12, Muted)

	if v.Status != "" {
		c.Text(v.Status, 64, 538, 13, Warn)
	}
}

func label(v Lobby) string {
	switch {
	case v.Table != "":
		return "RETURN TO TABLE  >"
	case v.Connected:
		return "WAITING FOR AN INVITATION"
	default:
		return "BRIDGE SETTINGS  >"
	}
}

// drawEmblem is the mark: four discs of one colour on a diagonal, which is the
// only thing that wins a board.
//
// Drawn rather than generated. It is four circles and a badge - geometry with
// one correct answer - and a generated mark that came back with three discs
// would be a logo that contradicts the name of the game.
func drawEmblem(c Canvas, x, y, size float64) {
	c.Rect(x, y, size, size, Blue)
	c.Rect(x+3, y+3, size-6, size-6, Navy)
	step := (size - 20) / 3
	for i := 0; i < 4; i++ {
		at := 10 + float64(i)*step
		c.Circle(x+at, y+at, 5.5, Turquoise)
	}
}

func drawGear(c Canvas) {
	c.Rect(gearX, gearY, gearSize, gearSize, Panel)
	cx, cy := gearX+gearSize/2, gearY+gearSize/2
	c.Circle(cx, cy, 12, Turquoise)
	c.Circle(cx, cy, 5, Panel)
	for _, d := range [][2]float64{{0, -16}, {0, 16}, {-16, 0}, {16, 0}} {
		c.Rect(cx+d[0]-3, cy+d[1]-3, 6, 6, Turquoise)
	}
}

func drawButton(c Canvas, x, y, w, h float64, text string, fill, ink color.RGBA) {
	c.Rect(x, y, w, h, fill)
	c.Text(text, x+20, y+h/2-9, 15, ink)
}

func upper(s string) string {
	out := []rune(s)
	for i, r := range out {
		if r >= 'a' && r <= 'z' {
			out[i] = r - 32
		}
	}
	return string(out)
}
