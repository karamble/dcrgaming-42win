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

//go:embed lobby-backdrop.png
var lobbyBackdrop []byte

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
