package render

import "fmt"

// Settings is the bridge connection screen, drawn as a panel over the lobby.
type Settings struct {
	Network string
	Fields  []Field
	// Status is the last thing that happened, and Warn colours it as a
	// problem rather than progress.
	Status    string
	Warn      bool
	Connected bool
}

// Field is one editable row.
type Field struct {
	Label       string
	Value       string
	Placeholder string
	// Masked hides the value. A private key is shown to nobody, including
	// the person who pasted it.
	Masked  bool
	Focused bool
}

// The settings panel, and the rows and buttons inside it.
const (
	panX, panY = 140.0, 56.0
	panW, panH = Width - 2*panX, Height - 2*panY

	rowX     = panX + 200
	rowW     = panW - 232
	rowH     = 38.0
	rowTop   = panY + 150
	rowPitch = 46.0

	btnY = panY + panH - 108
	btnW = (panW - 64 - 32) / 3
	btnH = 44.0

	netX, netY = panX + 200, panY + 104
	netW, netH = 108.0, 28.0
)

// FieldRect is where row i is drawn.
func FieldRect(i int) (x, y, w, h float64) {
	return rowX, rowTop + float64(i)*rowPitch, rowW, rowH
}

// ButtonRect is where button i is drawn: connect, disconnect, save.
func ButtonRect(i int) (x, y, w, h float64) {
	return panX + 32 + float64(i)*(btnW+16), btnY, btnW, btnH
}

// NetworkRect is where network choice i is drawn.
func NetworkRect(i int) (x, y, w, h float64) {
	return netX + float64(i)*(netW+12), netY, netW, netH
}

// CloseRect is the panel's close button.
func CloseRect() (x, y, w, h float64) { return panX + panW - 56, panY + 20, 32, 32 }

// ButtonLabels are the three actions, in the order ButtonRect indexes them.
var ButtonLabels = []string{"CONNECT", "DISCONNECT", "SAVE SETTINGS"}

// DrawSettings draws the panel over whatever is behind it.
func DrawSettings(c Canvas, v Settings) {
	// Dim what is underneath rather than hide it: the lobby is still the
	// thing being configured.
	c.Rect(0, 0, Width, Height, dim)
	c.Rect(panX, panY, panW, panH, Panel)

	c.Text("BRIDGE CONNECTION", panX+32, panY+24, 22, Turquoise)
	x, y, w, h := CloseRect()
	c.Line(x+8, y+8, x+w-8, y+h-8, 2, Muted)
	c.Line(x+w-8, y+8, x+8, y+h-8, 2, Muted)

	c.Text("Register this game as dcr4inarow in dcrpulse, then paste what it issues.",
		panX+32, panY+62, 13, Muted)

	// Network. It decides whether the money is real, so it is a choice on
	// the screen rather than a default nobody saw.
	c.Text("Network", panX+32, netY+6, 14, Text)
	for i, n := range networkNames {
		nx, ny, nw, nh := NetworkRect(i)
		fill, ink := Well, Muted
		if n == v.Network {
			fill, ink = Turquoise, Navy
		}
		c.Rect(nx, ny, nw, nh, fill)
		c.Text(n, nx+12, ny+6, 13, ink)
	}

	for i, f := range v.Fields {
		fx, fy, fw, fh := FieldRect(i)
		c.Text(f.Label, panX+32, fy+10, 14, Text)
		fill := Well
		if f.Focused {
			fill = Hollow
		}
		c.Rect(fx, fy, fw, fh, fill)
		if f.Focused {
			c.Rect(fx, fy, 3, fh, Turquoise)
		}
		text, ink := f.Placeholder, Muted
		if shown := display(f); shown != "" {
			text, ink = shown, Turquoise
		}
		c.Text(text, fx+14, fy+11, 13, ink)
	}

	c.Text("Tab: next field   ·   Ctrl+V: paste   ·   Del or Ctrl+A: clear the field",
		panX+32, btnY-26, 12, Muted)

	for i, name := range ButtonLabels {
		bx, by, bw, bh := ButtonRect(i)
		fill, ink := Well, Turquoise
		if i == 0 && !v.Connected {
			fill, ink = Turquoise, Navy
		}
		if i == 1 && !v.Connected {
			ink = Muted
		}
		c.Rect(bx, by, bw, bh, fill)
		c.Text(name, bx+16, by+bh/2-8, 14, ink)
	}

	status, ink := v.Status, Muted
	if status == "" {
		status = "Not connected. Paste the credentials supplied by dcrpulse."
	}
	if v.Warn {
		ink = Warn
	} else if v.Connected {
		ink = Turquoise
	}
	c.Text(status, panX+32, panY+panH-44, 13, ink)
}

var networkNames = []string{"mainnet", "testnet3", "simnet"}

// display is what a field shows.
//
// A pasted credential is thousands of characters of base64 and showing it
// would be noise, so a PEM is reported by what it is and how big it is. A
// private key is not shown at all, to anybody, ever.
func display(f Field) string {
	if f.Value == "" {
		return ""
	}
	if f.Masked {
		return fmt.Sprintf("•••••••••••••  %d bytes pasted", len(f.Value))
	}
	if len(f.Value) > 48 {
		return fmt.Sprintf("%s  ·  %d bytes", firstLine(f.Value), len(f.Value))
	}
	return f.Value
}

func firstLine(s string) string {
	for i, r := range s {
		if r == '\n' || r == '\r' {
			return s[:i]
		}
	}
	if len(s) > 44 {
		return s[:44]
	}
	return s
}
