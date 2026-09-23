package render

import (
	"image"
	"image/color"
	"image/draw"
	"math"

	xdraw "golang.org/x/image/draw"
	"golang.org/x/image/font"
	"golang.org/x/image/font/gofont/gobold"
	"golang.org/x/image/font/gofont/goregular"
	"golang.org/x/image/font/opentype"
	xfixed "golang.org/x/image/math/fixed"
	"golang.org/x/image/vector"
)

// Raster is the offline canvas: the same drawing, into an image, with no
// display and no GPU.
//
// It exists so the screen can be inspected and regression-tested. A renderer
// only reachable by opening a window is one nobody checks.
type Raster struct {
	Target       *image.RGBA
	faces        map[int]font.Face
	normal, bold *opentype.Font
}

func NewRaster() *Raster {
	n, _ := opentype.Parse(goregular.TTF)
	b, _ := opentype.Parse(gobold.TTF)
	return &Raster{
		Target: image.NewRGBA(image.Rect(0, 0, int(Width), int(Height))),
		faces:  map[int]font.Face{},
		normal: n, bold: b,
	}
}

func (r *Raster) Rect(x, y, w, h float64, c color.RGBA) {
	if w <= 0 || h <= 0 {
		return
	}
	box := image.Rect(int(x), int(y), int(math.Ceil(x+w)), int(math.Ceil(y+h)))
	draw.Draw(r.Target, box, image.NewUniform(color.NRGBA(c)), image.Point{}, draw.Over)
}

func (r *Raster) Circle(x, y, radius float64, c color.RGBA) {
	if radius <= 0 {
		return
	}
	var p vector.Rasterizer
	box := image.Rect(int(math.Floor(x-radius-1)), int(math.Floor(y-radius-1)), int(math.Ceil(x+radius+1)), int(math.Ceil(y+radius+1)))
	p.Reset(box.Dx(), box.Dy())
	x -= float64(box.Min.X)
	y -= float64(box.Min.Y)
	const steps = 64
	p.MoveTo(float32(x+radius), float32(y))
	for i := 1; i <= steps; i++ {
		a := 2 * math.Pi * float64(i) / steps
		p.LineTo(float32(x+radius*math.Cos(a)), float32(y+radius*math.Sin(a)))
	}
	p.ClosePath()
	p.Draw(r.Target, box, image.NewUniform(color.NRGBA(c)), image.Point{})
}

func (r *Raster) Line(x0, y0, x1, y1, width float64, c color.RGBA) {
	dx, dy := x1-x0, y1-y0
	length := math.Hypot(dx, dy)
	if length == 0 {
		return
	}
	nx, ny := -dy/length*width/2, dx/length*width/2
	var p vector.Rasterizer
	box := image.Rect(int(math.Floor(math.Min(x0, x1)-math.Abs(nx)-1)), int(math.Floor(math.Min(y0, y1)-math.Abs(ny)-1)), int(math.Ceil(math.Max(x0, x1)+math.Abs(nx)+1)), int(math.Ceil(math.Max(y0, y1)+math.Abs(ny)+1)))
	p.Reset(box.Dx(), box.Dy())
	x0 -= float64(box.Min.X)
	x1 -= float64(box.Min.X)
	y0 -= float64(box.Min.Y)
	y1 -= float64(box.Min.Y)
	p.MoveTo(float32(x0+nx), float32(y0+ny))
	p.LineTo(float32(x1+nx), float32(y1+ny))
	p.LineTo(float32(x1-nx), float32(y1-ny))
	p.LineTo(float32(x0-nx), float32(y0-ny))
	p.ClosePath()
	p.Draw(r.Target, box, image.NewUniform(color.NRGBA(c)), image.Point{})
}

func (r *Raster) Text(s string, x, y, size float64, c color.RGBA) {
	d := &font.Drawer{
		Dst:  r.Target,
		Src:  image.NewUniform(color.NRGBA(c)),
		Face: r.face(size),
		Dot:  xfixed.P(int(x), int(y+size)),
	}
	d.DrawString(s)
}

func (r *Raster) TextWeight(s string, x, y, size float64, ink color.RGBA, bold bool) {
	d := font.Drawer{Dst: r.Target, Src: image.NewUniform(color.NRGBA(ink)), Face: textFace(size, bold), Dot: xfixed.P(int(x), int(y+size))}
	d.DrawString(s)
}

func (r *Raster) Clip(box image.Rectangle) Canvas {
	return &Raster{Target: r.Target.SubImage(box.Intersect(r.Target.Bounds())).(*image.RGBA), faces: r.faces, normal: r.normal, bold: r.bold}
}

func (r *Raster) face(size float64) font.Face {
	key := int(size * 10)
	if f, ok := r.faces[key]; ok {
		return f
	}
	src := r.normal
	if size >= 20 {
		src = r.bold
	}
	f, _ := opentype.NewFace(src, &opentype.FaceOptions{Size: size, DPI: 72, Hinting: font.HintingFull})
	r.faces[key] = f
	return f
}

func (r *Raster) Image(src image.Image, x, y, w, h float64) {
	if src == nil || w <= 0 || h <= 0 {
		return
	}
	box := image.Rect(int(x), int(y), int(math.Ceil(x+w)), int(math.Ceil(y+h)))
	xdraw.CatmullRom.Scale(r.Target, box, src, src.Bounds(), xdraw.Over, nil)
}
