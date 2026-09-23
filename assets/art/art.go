// Package art holds the generated artwork the client draws.
//
// Embedded rather than loaded from disk: a screen that needed a file beside the
// binary is a screen that breaks on somebody else's machine. Everything here is
// optional - every screen is drawn to read correctly with none of it.
package art

import (
	"bytes"
	_ "embed"
	"image"
	"image/png"
	"sync"
)

//go:embed lobby-v2.png
var lobbyBackdrop []byte

//go:embed cover-v4.png
var coverBytes []byte

//go:embed tabletop-v2.png
var tableBytes []byte

//go:embed four2win-logo-v1.png
var logoBytes []byte

//go:embed four2win-wordmark-v1.png
var wordmarkBytes []byte

var logoOnce, wordmarkOnce sync.Once
var logoImage, wordmarkImage image.Image

// Logo is the illustrated title used on the cover and in the lobby.
func Logo() image.Image {
	logoOnce.Do(func() { logoImage = decodeLogo(logoBytes) })
	return logoImage
}

// Wordmark keeps small headers legible without the decorative board and discs.
func Wordmark() image.Image {
	wordmarkOnce.Do(func() { wordmarkImage = decodeLogo(wordmarkBytes) })
	return wordmarkImage
}

// Ignore transparent canvas margins for layout, retaining the original pixels
// and alpha. Both renderers use the resulting image bounds when scaling.
func decodeLogo(data []byte) image.Image {
	img, err := png.Decode(bytes.NewReader(data))
	if err != nil {
		return nil
	}
	var visible image.Rectangle
	for y := img.Bounds().Min.Y; y < img.Bounds().Max.Y; y++ {
		for x := img.Bounds().Min.X; x < img.Bounds().Max.X; x++ {
			if _, _, _, a := img.At(x, y).RGBA(); a != 0 {
				visible = visible.Union(image.Rect(x, y, x+1, y+1))
			}
		}
	}
	if visible.Empty() {
		return nil
	}
	if sub, ok := img.(interface {
		SubImage(image.Rectangle) image.Image
	}); ok {
		return sub.SubImage(visible)
	}
	return img
}

var coverOnce, tableOnce sync.Once
var coverImage, tableImage image.Image

func Cover() image.Image {
	coverOnce.Do(func() { coverImage, _ = png.Decode(bytes.NewReader(coverBytes)) })
	return coverImage
}

func Tabletop() image.Image {
	tableOnce.Do(func() { tableImage, _ = png.Decode(bytes.NewReader(tableBytes)) })
	return tableImage
}

// Preload prepares CPU assets without touching a GPU or the bridge.
func Preload() bool {
	return LobbyBackdrop() != nil && Tabletop() != nil && Logo() != nil && Wordmark() != nil
}

var (
	once     sync.Once
	backdrop image.Image
)

// LobbyBackdrop is the plate behind the opening screen, or nil if it cannot be
// decoded. A caller draws over nil without checking.
func LobbyBackdrop() image.Image {
	once.Do(func() {
		img, err := png.Decode(bytes.NewReader(lobbyBackdrop))
		if err == nil {
			backdrop = img
		}
	})
	return backdrop
}
