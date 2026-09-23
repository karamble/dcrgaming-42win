package render

import (
	"github.com/karamble/dcrgaming-42win/assets/art"
	"image/color"
)

type Cover struct {
	Ready   bool
	Error   string
	Frame   uint64
	Reduced bool
}

func DrawCover(c Canvas, v Cover) {
	c.Rect(0, 0, Width, Height, Navy)
	Contain(c, art.Cover(), 0, 0, Width, Height)
	logoW := min(660.0, Width*.64)
	logoH := logoW / 3
	c.Rect(0, 0, Width, logoH+64, color.RGBA{4, 12, 26, 160})
	drawBrand(c, art.Logo(), (Width-logoW)/2, 12, logoW, logoH)
	tag := "TWO PLAYERS. FOUR IN A ROW."
	c.Text(tag, (Width-Measure(tag, 12))/2, logoH+28, 12, Turquoise)
	c.Rect(0, Height-96, Width, 96, color.RGBA{4, 12, 26, 225})
	label := "Preparing the table…"
	if v.Ready {
		label = "Press Enter, Space, or click to continue"
	}
	if v.Error != "" {
		label = v.Error + " · Continue with simplified visuals"
	}
	c.Text(fit(label, Width-80, 15), (Width-Measure(fit(label, Width-80, 15), 15))/2, Height-66, 15, Text)
	for i := 0; i < 4; i++ {
		ink := Muted
		if v.Ready || (!v.Reduced && int(v.Frame/12)%4 == i) {
			ink = Turquoise
		}
		c.Circle(Width/2-24+float64(i)*16, Height-29, 3, ink)
	}
}
