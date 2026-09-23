//go:build desktop

package main

import (
	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/inpututil"
	"github.com/karamble/dcrgaming-42win/pkg/render"
)

func (a *app) selectAll() {
	if a.focus >= 0 {
		a.selected = true
	}
}
func (a *app) deleteForward() {
	if a.focus < 0 {
		return
	}
	if a.selected || a.focus > fieldPort {
		a.clearField()
		return
	}
	v := a.field(a.focus)
	if a.cursor < len(v) {
		a.setField(a.focus, v[:a.cursor]+v[a.cursor+1:])
	}
}
func (a *app) editKeys() {
	if a.focus < 0 || a.focus > fieldPort {
		return
	}
	if inpututil.IsKeyJustPressed(ebiten.KeyLeft) {
		a.cursor = max(0, a.cursor-1)
		a.selected = false
	}
	if inpututil.IsKeyJustPressed(ebiten.KeyRight) {
		a.cursor = min(len(a.field(a.focus)), a.cursor+1)
		a.selected = false
	}
	if inpututil.IsKeyJustPressed(ebiten.KeyHome) {
		a.cursor = 0
		a.selected = false
	}
	if inpututil.IsKeyJustPressed(ebiten.KeyEnd) {
		a.cursor = len(a.field(a.focus))
		a.selected = false
	}
}

func rect(x, y, w, h float64) render.Control { return render.Control{X: x, Y: y, W: w, H: h} }

func (c *client) controls() []render.Control {
	var out []render.Control
	add := func(x, y, w, h float64) { out = append(out, rect(x, y, w, h)) }
	if c.app == nil {
		return nil
	}
	switch c.app.currentScreen() {
	case screenLobby:
		add(render.GearRect())
		add(render.CallToActionRect())
		add(render.HelpRect())
		for i := range c.app.lobby().Tables {
			local := i - c.app.tablePage*3
			if local < 0 || local >= 3 {
				continue
			}
			add(render.TableSelectRect(local))
		}
		if len(c.app.lobby().Tables) > 3 {
			add(render.MoreTablesRect())
		}
	case screenSettings:
		for i := 0; i < 3; i++ {
			add(render.NetworkRect(i))
			out[len(out)-1].Disabled = c.app.settings().Connected
		}
		for i := 0; i < fieldCount; i++ {
			add(render.FieldRect(i))
			out[len(out)-1].Disabled = c.app.settings().Connected
		}
		for i := 0; i < 2; i++ {
			add(render.PreferenceRect(i))
		}
		for i := 0; i < 3; i++ {
			add(render.ButtonRect(i))
			out[len(out)-1].Disabled = (i == 1 && !c.app.settings().Connected) || (i == 0 && c.app.settings().Busy)
		}
		add(render.CloseRect())
	case screenSeating:
		add(render.LobbyBackRect())
		add(render.FundRect())
		if v, ok := c.app.seating(); !ok || !v.CanFund {
			out[len(out)-1].Disabled = true
		}
		add(render.DetailsRect())
	case screenTable:
		add(render.TableBackRect())
		if c.displayed.Done {
			add(render.ReceiptRect())
		}
		if c.displayed.CanAbandon {
			add(render.AbandonRect())
		}
	case screenReceipt:
		add(render.TableBackRect())
		add(render.ExportRect())
	}
	return out
}

func (c *client) controlInput(x, y *float64, click *bool) {
	controls := c.controls()
	if len(controls) == 0 {
		return
	}
	if *click {
		c.controlFocus = -1
	}
	if inpututil.IsKeyJustPressed(ebiten.KeyTab) {
		direction := 1
		if ebiten.IsKeyPressed(ebiten.KeyShiftLeft) || ebiten.IsKeyPressed(ebiten.KeyShiftRight) {
			direction = -1
		}
		for i := 0; i < len(controls); i++ {
			c.controlFocus = (c.controlFocus + direction + len(controls)) % len(controls)
			if !controls[c.controlFocus].Disabled {
				break
			}
		}
		if c.app.currentScreen() == screenSettings {
			if c.controlFocus >= 3 && c.controlFocus < 8 {
				c.app.setFocus(c.controlFocus - 3)
			} else {
				c.app.setFocus(-1)
			}
		}
	}
	if c.controlFocus >= len(controls) {
		c.controlFocus = -1
	}
	if c.controlFocus >= 0 && !controls[c.controlFocus].Disabled && (inpututil.IsKeyJustPressed(ebiten.KeyEnter) || inpututil.IsKeyJustPressed(ebiten.KeySpace)) {
		b := controls[c.controlFocus]
		*x, *y, *click = b.X+b.W/2, b.Y+b.H/2, true
	}
}

func (c *client) drawFocus() {
	mx, my := ebiten.CursorPosition()
	for i, b := range c.controls() {
		if !b.Disabled && (i == c.controlFocus || b.Contains(float64(mx), float64(my))) {
			render.Focus(c.gpu, b, ebiten.IsMouseButtonPressed(ebiten.MouseButtonLeft))
		}
	}
}
