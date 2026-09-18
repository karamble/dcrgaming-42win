// Package render draws dcr4inarow.
//
// Drawing never touches game state: a screen is handed a detached [View] and
// reads nothing else. The same scene code runs on two canvases - an offline
// rasteriser for reproducible inspection, and a GPU one behind the desktop
// build tag - so what a test renders to a PNG is what a player sees.
package render

import (
	"image"
	"image/color"

	"github.com/karamble/dcr4inarow/internal/board"
)

// Canvas is every drawing operation this game needs. Deliberately small: a
// four-in-a-row board is rectangles, circles, lines and words.
type Canvas interface {
	Rect(x, y, w, h float64, c color.RGBA)
	Circle(x, y, r float64, c color.RGBA)
	Line(x0, y0, x1, y1, width float64, c color.RGBA)
	Text(s string, x, y, size float64, c color.RGBA)
	Image(src image.Image, x, y, w, h float64)
}

// The window, and where the board sits in it.
const (
	Width  = 960
	Height = 640

	BoardX = 40.0
	BoardY = 96.0
	Cell   = 72.0
	Hole   = 28.0

	BoardW = Cell * board.Cols
	BoardH = Cell * board.Rows

	PanelX = BoardX + BoardW + 32
	PanelW = Width - PanelX - 40
)

// Decred's colours: deep navy, blue and turquoise.
var (
	Navy      = color.RGBA{9, 20, 64, 255}
	Panel     = color.RGBA{15, 32, 67, 255}
	Well      = color.RGBA{6, 14, 44, 255}
	Hollow    = color.RGBA{4, 10, 32, 255}
	Turquoise = color.RGBA{46, 216, 163, 255}
	Blue      = color.RGBA{41, 112, 255, 255}
	Text      = color.RGBA{232, 240, 255, 255}
	Muted     = color.RGBA{122, 140, 176, 255}
	Warn      = color.RGBA{240, 176, 96, 255}

	// dim is drawn over the lobby when a panel is open. Translucent, so the
	// screen being configured stays visible behind it.
	dim = color.RGBA{4, 9, 26, 215}
)

// SeatColour is the disc a seat plays. Two seats, two colours, and they are the
// identity colours rather than red and yellow because a player should be able
// to tell at a glance which game they are in.
func SeatColour(seat uint32) color.RGBA {
	if seat == 0 {
		return Turquoise
	}
	return Blue
}

// centre is where a cell's disc sits. Row zero is the floor, so it is drawn at
// the bottom of the board and rows count upward against gravity.
func centre(col, row int) (x, y float64) {
	return BoardX + float64(col)*Cell + Cell/2,
		BoardY + float64(board.Rows-1-row)*Cell + Cell/2
}

// ColumnAt maps a pointer position to a column, or -1 for anywhere else.
//
// Exported because it is the whole of the game's input and worth testing
// without a window open.
func ColumnAt(x, y float64) int {
	if x < BoardX || x >= BoardX+BoardW || y < BoardY || y >= BoardY+BoardH {
		return -1
	}
	col := int((x - BoardX) / Cell)
	if col < 0 || col >= board.Cols {
		return -1
	}
	return col
}
