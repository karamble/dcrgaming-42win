package render

import (
	"image"
	"math"

	"github.com/karamble/dcrgaming-42win/assets/art"
)

// BrandName is presentation only. Bridge registration, protocol identifiers,
// profiles and executable names remain dcr4inarow for compatibility.
const BrandName = "FOUR2WIN"

func drawBrand(c Canvas, img image.Image, x, y, w, h float64) {
	if img != nil {
		Contain(c, img, x, y, w, h)
		return
	}
	size := math.Min(h*.65, w/6.5)
	c.Text(BrandName, x+(w-Measure(BrandName, size))/2, y+(h-size)/2, size, Text)
}

func drawWordmark(c Canvas, x, y, w, h float64) {
	drawBrand(c, art.Wordmark(), x, y, w, h)
}
