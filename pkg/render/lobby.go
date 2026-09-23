package render

import (
	"image"
	"image/color"

	"github.com/karamble/dcrgaming-42win/assets/art"
)

// Lobby is the first screen: what this is, whether a bridge is reachable, and
// the one way forward.
type Lobby struct {
	Tables    []string
	TablePage int
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
var (
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
		Contain(c, v.Backdrop, 0, 0, Width, Height)
	}

	// Header.
	logo := v.Logo
	if logo == nil {
		logo = art.Logo()
	}
	drawBrand(c, logo, 40, 10, 470, 158)
	if v.Build != "" {
		c.Text(v.Build, Width-110-float64(len(v.Build))*7, 36, 12, Muted)
	}
	drawGear(c)

	// Hero.
	c.Text("TWO PLAYERS. FOUR IN A ROW.", 64, 179, 12, Turquoise)
	c.Text("Make your", 64, 212, 32, Text)
	c.Text("next move count.", 64, 252, 32, Text)
	Wrap(c, "A familiar game. A real stake. Connect four and win two rounds to take the match.", 64, 294, 420, 16, Muted, 3)

	// The state that decides what the button does.
	rounded(c, 64, 365, 420, 66, 8, Panel)
	c.Rect(64, 376, 3, 44, Turquoise)
	head, detail := "BRIDGE NOT SET UP", "Paste the credentials dcrpulse issued for this game"
	switch {
	case v.Connected:
		head = "BRIDGE CONNECTED  ·  " + upper(v.Network)
		detail = "Accept an invitation in dcrpulse to take a seat"
	case v.Configured:
		head = "BRIDGE NOT CONNECTED"
		detail = "Credentials are saved. Connect from settings."
	}
	c.Text(head, 84, 377, 12, Text)
	c.Text(fit(detail, 380, 12), 84, 401, 12, Muted)

	// What the game is, in three words each.
	c.Text("EVERY MOVE SIGNED  /  STAKED ON DECRED", 64, 445, 11, Muted)

	drawButton(c, ctaX, ctaY, ctaW, ctaH, label(v), Turquoise, Navy)
	hx, hy, hw, hh := HelpRect()
	drawButton(c, hx, hy, hw, hh, "How to create or join", Panel, Text)
	for i, sid := range v.Tables {
		local := i - v.TablePage*3
		if local < 0 || local >= 3 {
			continue
		}
		x, y, w, h := TableSelectRect(local)
		drawButton(c, x, y, w, h, "Table  "+short(sid), Panel, Text)
	}
	if len(v.Tables) > 3 {
		x, y, w, h := MoreTablesRect()
		drawButton(c, x, y, w, h, "More tables  →", Well, Text)
	}

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
		Wrap(c, v.Status, 64, Height-105, Width-128, 12, Warn, 2)
	}
}

func label(v Lobby) string {
	switch {
	case v.Table != "":
		return "RETURN TO TABLE  >"
	case v.Connected:
		return "Create / join in dcrpulse  →"
	case v.Configured:
		return "Connect to bridge  →"
	default:
		return "Set up bridge  →"
	}
}

func HelpRect() (x, y, w, h float64)             { return ctaX + ctaW + 16, ctaY, 220, ctaH }
func TableSelectRect(i int) (x, y, w, h float64) { return Width - 296, 140 + float64(i)*48, 256, 38 }

func MoreTablesRect() (x, y, w, h float64) { return Width - 296, 286, 256, 34 }

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
	rounded(c, x, y, w, h, 7, fill)
	c.Text(fit(text, w-24, 14), x+12, y+h/2-9, 14, ink)
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
