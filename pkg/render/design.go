package render

import (
	"image"
	"image/color"
	"math"
	"strings"

	"golang.org/x/image/font"
	"golang.org/x/image/font/gofont/gobold"
	"golang.org/x/image/font/gofont/goregular"
	"golang.org/x/image/font/opentype"
)

// SetSize is called only on the UI thread, before input or rendering. Logical
// pixels stay readable while the board and layout use the available space.
func SetSize(w, h int) {
	Width, Height = float64(max(960, w)), float64(max(640, h))
	Cell = math.Min(108, math.Min(float64(Height-220)/6, float64(Width-420)/7))
	BoardW, BoardH = 7*Cell, 6*Cell
	PanelW = math.Min(420, Width-BoardW-112)
	BoardX, BoardY, Hole = (Width-BoardW-32-PanelW)/2, math.Max(110, (Height-BoardH-80)/2), Cell*.385
	PanelX = BoardX + BoardW + 32
	gearX = float64(Width) - 80
	ctaY = float64(Height) - 170
	railStepW = float64(Width-80) / 6
	seatW = float64(Width-444) / 2
	termsX, termsW = 80+2*seatW+24, 300
	panX, panY, panW, panH = float64(Width-740)/2, float64(Height-600)/2, 740, 600
	rowX, rowW, rowTop = panX+200, panW-232, panY+150
	btnY, btnW = panY+panH-108, (panW-96)/3
	netX, netY = panX+200, panY+104
}

var uiFaces = map[int]font.Face{}

func textFace(size float64, bold bool) font.Face {
	k := int(size * 10)
	if bold {
		k += 10000
	}
	if f := uiFaces[k]; f != nil {
		return f
	}
	data := goregular.TTF
	if bold {
		data = gobold.TTF
	}
	f, _ := opentype.Parse(data)
	face, _ := opentype.NewFace(f, &opentype.FaceOptions{Size: size, DPI: 72, Hinting: font.HintingFull})
	uiFaces[k] = face
	return face
}

func Measure(s string, size float64) float64 {
	return float64(font.MeasureString(textFace(size, size >= 20), s)) / 64
}

func bold(c Canvas, s string, x, y, size float64, ink color.RGBA) {
	if styled, ok := c.(interface {
		TextWeight(string, float64, float64, float64, color.RGBA, bool)
	}); ok {
		styled.TextWeight(s, x, y, size, ink, true)
	} else {
		c.Text(s, x, y, size, ink)
	}
}

func clip(c Canvas, x, y, w, h float64) Canvas {
	if clipped, ok := c.(interface{ Clip(image.Rectangle) Canvas }); ok {
		return clipped.Clip(image.Rect(int(x), int(y), int(x+w), int(y+h)))
	}
	return c
}

func fit(s string, width, size float64) string {
	if Measure(s, size) <= width {
		return s
	}
	r := []rune(s)
	for len(r) > 0 && Measure(string(r)+"…", size) > width {
		r = r[:len(r)-1]
	}
	return string(r) + "…"
}

func Wrap(c Canvas, s string, x, y, w, size float64, ink color.RGBA, maxLines int) float64 {
	words := strings.Fields(s)
	line := ""
	row := 0
	for i, word := range words {
		trial := strings.TrimSpace(line + " " + word)
		if Measure(trial, size) > w && line != "" {
			if row == maxLines-1 {
				c.Text(fit(line+" "+strings.Join(words[i:], " "), w, size), x, y, size, ink)
				return y + size + 6
			}
			c.Text(fit(line, w, size), x, y, size, ink)
			y += size + 6
			row++
			line = word
		} else {
			line = trial
		}
	}
	if line != "" {
		c.Text(fit(line, w, size), x, y, size, ink)
		y += size + 6
	}
	return y
}

func rounded(c Canvas, x, y, w, h, r float64, ink color.RGBA) {
	r = math.Min(r, math.Min(w, h)/2)
	c.Rect(x+r, y, w-2*r, h, ink)
	c.Rect(x, y+r, w, h-2*r, ink)
	for _, p := range [][2]float64{{x + r, y + r}, {x + w - r, y + r}, {x + r, y + h - r}, {x + w - r, y + h - r}} {
		c.Circle(p[0], p[1], r, ink)
	}
}

func ring(c Canvas, x, y, r, width float64, ink color.RGBA) {
	for i := 0; i < 48; i++ {
		a, b := float64(i)*math.Pi/24, float64(i+1)*math.Pi/24
		c.Line(x+r*math.Cos(a), y+r*math.Sin(a), x+r*math.Cos(b), y+r*math.Sin(b), width, ink)
	}
}

func shade(v color.RGBA, f float64) color.RGBA {
	return color.RGBA{uint8(float64(v.R) * f), uint8(float64(v.G) * f), uint8(float64(v.B) * f), v.A}
}

// Contain preserves a generated image's aspect ratio; letterboxing is intentional.
func Contain(c Canvas, img image.Image, x, y, w, h float64) {
	if img == nil {
		return
	}
	s := math.Min(w/float64(img.Bounds().Dx()), h/float64(img.Bounds().Dy()))
	iw, ih := float64(img.Bounds().Dx())*s, float64(img.Bounds().Dy())*s
	c.Image(img, x+(w-iw)/2, y+(h-ih)/2, iw, ih)
}

type Control struct {
	X, Y, W, H float64
	Label      string
	Disabled   bool
}

func (b Control) Contains(x, y float64) bool { return Hit(x, y, b.X, b.Y, b.W, b.H) }
func Focus(c Canvas, b Control, pressed bool) {
	ink := Turquoise
	if pressed {
		ink = Text
	}
	c.Line(b.X, b.Y, b.X+b.W, b.Y, 2, ink)
	c.Line(b.X, b.Y+b.H, b.X+b.W, b.Y+b.H, 2, ink)
	c.Line(b.X, b.Y, b.X, b.Y+b.H, 2, ink)
	c.Line(b.X+b.W, b.Y, b.X+b.W, b.Y+b.H, 2, ink)
}
