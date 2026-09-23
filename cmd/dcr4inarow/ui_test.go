//go:build desktop && dev

package main

import (
	"errors"
	"image"
	"image/png"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/karamble/dcrgaming-42win/assets/art"
	"github.com/karamble/dcrgaming-42win/internal/appconfig"
	"github.com/karamble/dcrgaming-42win/internal/bridgeconn"
	"github.com/karamble/dcrgaming-42win/pkg/render"
)

func TestTextEditingAndCachedResult(t *testing.T) {
	a := &app{cfg: bridgeconn.Defaults(), focus: -1}
	a.setFocus(fieldHost)
	a.selectAll()
	a.typed([]rune("192.0.2.1"))
	if a.field(fieldHost) != "192.0.2.1" {
		t.Fatal("selection was appended to")
	}
	a.cursor = 3
	a.backspace()
	if a.field(fieldHost) != "19.0.2.1" {
		t.Fatal("backspace ignored caret")
	}
	a.selectAll()
	a.deleteForward()
	if a.field(fieldHost) != "" {
		t.Fatal("delete did not replace selection")
	}
	a.setFocus(fieldClientKey)
	a.typed([]rune("secret"))
	if a.field(fieldClientKey) != "" {
		t.Fatal("credential accepted typing")
	}
	a.cached = render.View{MatchID: "finished", Done: true, CanAbandon: true, Connected: true}
	v, ok := a.table()
	if !ok || !v.Stale || v.Connected || v.CanAbandon {
		t.Fatal("cached result is actionable")
	}
	if !a.hasTable() {
		t.Fatal("cached result cannot be reopened")
	}
	a.returnToTable()
	if a.currentScreen() != screenTable {
		t.Fatal("cached result navigated to funding")
	}
}

func TestAudioFailureDisablesSoundOnly(t *testing.T) {
	s := soundBank{started: true, ready: make(chan audioInit, 1)}
	s.ready <- audioInit{err: errors.New("device unavailable")}
	s.play("turn")
	if !s.disabled {
		t.Fatal("unavailable audio was not disabled")
	}
	s.play("result")
}

// Runs the actual GPU client in a short-lived fixture window. It uses the same
// input handlers as mouse activation and captures only this application's pixels.
func TestDesktopWalkthrough(t *testing.T) {
	dir := t.TempDir()
	a, err := newApp(&appconfig.Config{Options: appconfig.Options{AppData: dir, BridgeConfig: filepath.Join(dir, "bridge.json")}}, nil)
	if err != nil {
		t.Fatal(err)
	}
	shots, err := os.MkdirTemp("/tmp", "dcr4inarow-ui-walkthrough-")
	if err != nil {
		t.Fatal(err)
	}
	_ = art.Preload()
	c := &client{app: a, gpu: render.NewGPU(), hover: -1, controlFocus: -1, cover: true, loadResult: make(chan bool, 1)}
	w := &walkthrough{client: c, t: t, shots: shots, start: time.Now()}
	c.sounds.prepare()
	ebiten.SetWindowTitle(render.BrandName + " · UI verification fixture")
	ebiten.SetWindowSize(960, 640)
	if err = ebiten.RunGame(w); err != nil && !errors.Is(err, ebiten.Termination) {
		t.Fatal(err)
	}
	if w.moves != 21 {
		t.Fatalf("only %d fixture moves played", w.moves)
	}
	for _, name := range []string{"cover-loading", "cover-ready", "lobby", "settings", "help", "board-opening", "board-won", "receipt"} {
		if _, err := os.Stat(filepath.Join(shots, name+".png")); err != nil {
			t.Error("missing GPU capture", name)
		}
	}
	t.Logf("GPU screenshots: %s", shots)
}

type walkthrough struct {
	*client
	t           *testing.T
	shots       string
	tick, moves int
	shot        string
	start       time.Time
	finalFrames int
}

func (w *walkthrough) Update() error {
	// Ebiten can run several updates before drawing. Do not skip a capture.
	if w.shot != "" {
		return nil
	}
	if time.Since(w.start) > 45*time.Second {
		return errors.New("UI walkthrough timed out")
	}
	w.tick++
	switch w.tick {
	case 1:
		w.shot = "cover-loading"
	case 3:
		w.loadResult <- true
		w.shot = "cover-ready"
	case 5:
		w.cover = false
		w.shot = "lobby"
	case 7:
		x, y, _, _ := render.GearRect()
		w.lobbyInput(x+5, y+5, true)
		if w.app.currentScreen() != screenSettings {
			return errors.New("settings button missed")
		}
		w.shot = "settings"
	case 9:
		x, y, _, _ := render.PreferenceRect(0)
		w.settingsInput(x+5, y+5, true)
		if !w.app.prefs.Muted {
			return errors.New("mute button missed")
		}
		w.settingsInput(x+5, y+5, true)
	case 11:
		w.app.show(screenLobby)
		x, y, _, _ := render.HelpRect()
		w.lobbyInput(x+5, y+5, true)
		w.shot = "help"
	case 13:
		l, err := openFixture()
		if err != nil {
			return err
		}
		w.fixture = l
		w.shot = "board-opening"
	}
	if w.tick > 15 && w.fixture != nil && !w.presenter.Busy() {
		if w.moves < 21 {
			cols := []int{0, 1, 0, 1, 0, 1, 0}
			if err := w.fixture.Play(cols[w.moves%7]); err != nil {
				return err
			}
			w.moves++
		} else {
			w.finalFrames++
			if w.finalFrames == 1 {
				w.shot = "board-won"
			}
			if w.finalFrames == 3 {
				v := w.fixture.View()
				w.fixture = nil
				w.app.cached = v
				w.app.show(screenReceipt)
				w.shot = "receipt"
			}
		}
	}
	if w.fixture == nil && w.moves == 21 {
		w.finalFrames++
		if w.finalFrames > 7 {
			return ebiten.Termination
		}
	}
	return w.client.Update()
}
func (w *walkthrough) Draw(dst *ebiten.Image) {
	w.client.Draw(dst)
	if w.shot == "" {
		return
	}
	im := image.NewRGBA(dst.Bounds())
	dst.ReadPixels(im.Pix)
	f, err := os.Create(filepath.Join(w.shots, w.shot+".png"))
	if err != nil {
		w.t.Error(err)
		return
	}
	if err = png.Encode(f, im); err != nil {
		w.t.Error(err)
	}
	f.Close()
	w.shot = ""
}
