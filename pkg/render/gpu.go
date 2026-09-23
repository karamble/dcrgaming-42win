//go:build desktop

package render

import (
	"image"
	"image/color"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/text"
	"github.com/hajimehoshi/ebiten/v2/vector"
	"golang.org/x/image/font"
	"golang.org/x/image/font/gofont/gobold"
	"golang.org/x/image/font/gofont/goregular"
	"golang.org/x/image/font/opentype"
)

// GPU is the same canvas on the desktop. Its drawing has to match Raster's, or
// a preview stops being evidence of anything.
type GPU struct {
	Target       *ebiten.Image
	faces        map[int]font.Face
	sprites      map[image.Image]*ebiten.Image
	normal, bold *opentype.Font
}

func NewGPU() *GPU {
	n, _ := opentype.Parse(goregular.TTF)
	b, _ := opentype.Parse(gobold.TTF)
	return &GPU{faces: map[int]font.Face{}, sprites: map[image.Image]*ebiten.Image{}, normal: n, bold: b}
}

func (g *GPU) Rect(x, y, w, h float64, c color.RGBA) {
	if w > 0 && h > 0 {
		vector.DrawFilledRect(g.Target, float32(x), float32(y), float32(w), float32(h), color.NRGBA(c), false)
	}
}

func (g *GPU) Circle(x, y, r float64, c color.RGBA) {
	vector.DrawFilledCircle(g.Target, float32(x), float32(y), float32(r), color.NRGBA(c), true)
}

func (g *GPU) Line(x0, y0, x1, y1, width float64, c color.RGBA) {
	vector.StrokeLine(g.Target, float32(x0), float32(y0), float32(x1), float32(y1), float32(width), color.NRGBA(c), true)
}

func (g *GPU) Text(s string, x, y, size float64, c color.RGBA) {
	key := int(size * 10)
	face, ok := g.faces[key]
	if !ok {
		src := g.normal
		if size >= 20 {
			src = g.bold
		}
		face, _ = opentype.NewFace(src, &opentype.FaceOptions{Size: size, DPI: 72, Hinting: font.HintingFull})
		g.faces[key] = face
	}
	text.Draw(g.Target, s, face, int(x), int(y+size), color.NRGBA(c))
}

func (g *GPU) TextWeight(s string, x, y, size float64, ink color.RGBA, bold bool) {
	text.Draw(g.Target, s, textFace(size, bold), int(x), int(y+size), color.NRGBA(ink))
}

func (g *GPU) Clip(box image.Rectangle) Canvas {
	return &GPU{Target: g.Target.SubImage(box.Intersect(g.Target.Bounds())).(*ebiten.Image), faces: g.faces, sprites: g.sprites, normal: g.normal, bold: g.bold}
}

func (g *GPU) Image(src image.Image, x, y, w, h float64) {
	if src == nil || w <= 0 || h <= 0 {
		return
	}
	texture, ok := g.sprites[src]
	if !ok {
		texture = ebiten.NewImageFromImage(src)
		g.sprites[src] = texture
	}
	op := &ebiten.DrawImageOptions{Filter: ebiten.FilterLinear}
	op.GeoM.Scale(w/float64(src.Bounds().Dx()), h/float64(src.Bounds().Dy()))
	op.GeoM.Translate(x, y)
	g.Target.DrawImage(texture, op)
}
