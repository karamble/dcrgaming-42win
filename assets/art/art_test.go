package art

import (
	"bytes"
	"image"
	"image/color"
	"image/png"
	"testing"
)

func TestBrandAssetsHaveRealTransparency(t *testing.T) {
	for name, img := range map[string]image.Image{"logo": Logo(), "wordmark": Wordmark()} {
		if img == nil {
			t.Fatalf("%s failed to decode", name)
		}
		b := img.Bounds()
		transparent, nearOpaque := 0, 0
		for y := b.Min.Y; y < b.Max.Y; y++ {
			for x := b.Min.X; x < b.Max.X; x++ {
				_, _, _, a := img.At(x, y).RGBA()
				if a == 0 {
					transparent++
				} else if a >= 250*257 {
					// Generated lettering retains very slight translucency.
					nearOpaque++
				}
			}
		}
		if transparent < b.Dx()*b.Dy()/10 || nearOpaque < b.Dx()*b.Dy()/10 {
			t.Fatalf("%s must contain transparent space and near-opaque artwork", name)
		}
		t.Logf("%s visible bounds: %v", name, b)
	}
	if !Preload() {
		t.Fatal("preload failed")
	}
}

func TestDecodeLogoBoundsAndFallback(t *testing.T) {
	img := image.NewNRGBA(image.Rect(0, 0, 20, 20))
	img.SetNRGBA(4, 6, color.NRGBA{G: 255, A: 255})
	img.SetNRGBA(12, 15, color.NRGBA{B: 255, A: 128})
	var data bytes.Buffer
	if err := png.Encode(&data, img); err != nil {
		t.Fatal(err)
	}
	got := decodeLogo(data.Bytes())
	if got == nil || got.Bounds() != image.Rect(4, 6, 13, 16) {
		t.Fatalf("transparent padding not excluded: %v", got)
	}
	_, _, _, alpha := got.At(12, 15).RGBA()
	if alpha != 128*257 {
		t.Fatal("partial alpha changed")
	}
	if decodeLogo([]byte("invalid PNG")) != nil {
		t.Fatal("invalid data should fall back")
	}
	data.Reset()
	if err := png.Encode(&data, image.NewNRGBA(image.Rect(0, 0, 2, 2))); err != nil {
		t.Fatal(err)
	}
	if decodeLogo(data.Bytes()) != nil {
		t.Fatal("empty artwork should fall back")
	}
}
