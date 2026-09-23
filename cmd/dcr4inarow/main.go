//go:build desktop

// Command dcr4inarow is the desktop client.
package main

import (
	"context"
	"errors"
	"fmt"
	"os"
	"time"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/inpututil"
	flags "github.com/jessevdk/go-flags"
	"github.com/karamble/dcrgaming-42win/assets/art"
	"github.com/karamble/dcrgaming-42win/internal/appconfig"
	"github.com/karamble/dcrgaming-42win/internal/bridgeconn"
	"github.com/karamble/dcrgaming-42win/internal/logging"
	"github.com/karamble/dcrgaming-42win/internal/session"
	"github.com/karamble/dcrgaming-42win/pkg/render"
)

// source is a table the board screen can draw and click on. The app provides
// one; so does the local fixture, and the screen cannot tell them apart.
type source interface {
	View() render.View
	Play(column int) error
}

type client struct {
	presenter              render.Presenter
	displayed              render.View
	sounds                 soundBank
	cover                  bool
	ready                  bool
	loadResult             chan bool
	loadError              string
	frame                  uint64
	controlFocus           int
	lastScreen             screen
	lastMouseX, lastMouseY int
	confirmVoid            bool
	app                    *app
	fixture                source
	gpu                    *render.GPU
	hover                  int
}

func (c *client) Update() error {
	c.frame++
	c.sounds.poll()
	if c.cover {
		if !c.ready {
			select {
			case ok := <-c.loadResult:
				c.ready = true
				if !ok {
					c.loadError = "Some artwork could not load"
				}
			default:
			}
		}
		if c.ready && (inpututil.IsKeyJustPressed(ebiten.KeyEnter) || inpututil.IsKeyJustPressed(ebiten.KeySpace) || inpututil.IsMouseButtonJustPressed(ebiten.MouseButtonLeft)) {
			c.cover = false
		}
		return nil
	}
	mx, my := ebiten.CursorPosition()
	x, y := float64(mx), float64(my)
	if mx != c.lastMouseX || my != c.lastMouseY {
		c.hover = render.ColumnAt(x, y)
		c.lastMouseX, c.lastMouseY = mx, my
	}
	click := inpututil.IsMouseButtonJustPressed(ebiten.MouseButtonLeft)

	if c.fixture != nil {
		c.boardKeys(&click)
		if click && c.hover >= 0 && !c.presenter.Busy() {
			if err := c.fixture.Play(c.hover); err != nil {
				return nil
			}
		}
		v := c.fixture.View()
		var cues []string
		c.displayed, cues = c.presenter.Update(v, time.Now(), false)
		for _, cue := range cues {
			if c.app == nil || !c.app.prefs.Muted {
				c.sounds.play(cue)
			}
		}
		return nil
	}

	c.app.adopt()
	c.app.followTable()
	if c.app.currentScreen() != c.lastScreen {
		c.lastScreen = c.app.currentScreen()
		c.controlFocus = -1
		c.confirmVoid = false
	}
	c.controlInput(&x, &y, &click)
	switch c.app.currentScreen() {
	case screenLobby:
		c.lobbyInput(x, y, click)
	case screenSettings:
		c.settingsInput(x, y, click)
	case screenSeating:
		c.seatingInput(x, y, click)
	case screenTable:
		c.boardKeys(&click)
		if c.controlFocus < 0 && c.hover >= 0 && (inpututil.IsKeyJustPressed(ebiten.KeyEnter) || inpututil.IsKeyJustPressed(ebiten.KeySpace)) {
			x = render.BoardX + (float64(c.hover)+.5)*render.Cell
			y = render.BoardY + render.Cell/2
		}
		c.tableInputAt(x, y, click)
	case screenHelp:
		if click || inpututil.IsKeyJustPressed(ebiten.KeyEscape) || inpututil.IsKeyJustPressed(ebiten.KeyEnter) {
			c.app.show(screenLobby)
		}
	case screenReceipt:
		if inpututil.IsKeyJustPressed(ebiten.KeyEscape) {
			c.app.show(screenTable)
		}
		if click {
			bx, by, bw, bh := render.TableBackRect()
			if render.Hit(x, y, bx, by, bw, bh) {
				c.app.show(screenLobby)
			}
			bx, by, bw, bh = render.ExportRect()
			if render.Hit(x, y, bx, by, bw, bh) {
				c.app.exportReceipt()
			}
		}
	}
	if c.app.currentScreen() == screenTable {
		if v, ok := c.app.table(); ok {
			var cues []string
			c.displayed, cues = c.presenter.Update(v, time.Now(), c.app.prefs.ReducedMotion)
			if !c.app.prefs.Muted {
				for _, cue := range cues {
					c.sounds.play(cue)
				}
			}
		}
	}
	return nil
}

func (c *client) boardKeys(click *bool) {
	if inpututil.IsKeyJustPressed(ebiten.KeyLeft) {
		c.controlFocus = -1
		c.hover = (max(c.hover, 0) + 6) % 7
	}
	if inpututil.IsKeyJustPressed(ebiten.KeyRight) {
		c.controlFocus = -1
		c.hover = (max(c.hover, 0) + 1) % 7
	}
	if c.controlFocus < 0 && (inpututil.IsKeyJustPressed(ebiten.KeyEnter) || inpututil.IsKeyJustPressed(ebiten.KeySpace)) {
		if c.presenter.Busy() {
			c.presenter.Dismiss()
		} else if c.hover >= 0 {
			*click = true
		}
	}
}

// seatingInput drives the table-formation screen. The only action it offers is
// the one the SDK offers: ask for the stake, once.
func (c *client) seatingInput(x, y float64, click bool) {
	if click {
		dx, dy, dw, dh := render.DetailsRect()
		if render.Hit(x, y, dx, dy, dw, dh) {
			c.app.details = !c.app.details
			return
		}
	}
	if inpututil.IsKeyJustPressed(ebiten.KeyEscape) {
		c.app.show(screenLobby)
		return
	}
	view, ok := c.app.seating()
	if !ok {
		return
	}
	if view.CanFund && inpututil.IsKeyJustPressed(ebiten.KeyF) {
		c.app.fund()
		return
	}
	if !click {
		return
	}
	if bx, by, bw, bh := render.LobbyBackRect(); render.Hit(x, y, bx, by, bw, bh) {
		c.app.show(screenLobby)
		return
	}
	if fx, fy, fw, fh := render.FundRect(); render.Hit(x, y, fx, fy, fw, fh) {
		switch {
		case view.CanFund:
			c.app.fund()
		}
	}
}

func (c *client) lobbyInput(x, y float64, click bool) {
	if inpututil.IsKeyJustPressed(ebiten.KeyEscape) {
		return
	}
	if !click {
		return
	}
	if hx, hy, hw, hh := render.HelpRect(); render.Hit(x, y, hx, hy, hw, hh) {
		c.app.show(screenHelp)
		return
	}
	if nx, ny, nw, nh := render.MoreTablesRect(); render.Hit(x, y, nx, ny, nw, nh) {
		count := len(c.app.lobby().Tables)
		if count > 3 {
			c.app.tablePage = (c.app.tablePage + 1) % ((count + 2) / 3)
		}
		return
	}
	for i, sid := range c.app.lobby().Tables {
		local := i - c.app.tablePage*3
		if local < 0 || local >= 3 {
			continue
		}
		tx, ty, tw, th := render.TableSelectRect(local)
		if render.Hit(x, y, tx, ty, tw, th) {
			c.app.mu.Lock()
			c.app.sid = sid
			c.app.mu.Unlock()
			c.presenter.Reset()
			c.app.show(screenSeating)
			return
		}
	}
	if gx, gy, gw, gh := render.GearRect(); render.Hit(x, y, gx, gy, gw, gh) {
		c.app.show(screenSettings)
		return
	}
	if bx, by, bw, bh := render.CallToActionRect(); render.Hit(x, y, bx, by, bw, bh) {
		if c.app.hasTable() {
			c.app.returnToTable()
			return
		}
		v := c.app.lobby()
		if v.Configured && !v.Connected {
			c.app.connect()
		} else if v.Connected {
			c.app.show(screenHelp)
		} else {
			c.app.show(screenSettings)
		}
	}
}

func (c *client) settingsInput(x, y float64, click bool) {
	if inpututil.IsKeyJustPressed(ebiten.KeyEscape) {
		c.app.show(screenLobby)
		return
	}
	if ctrl() && inpututil.IsKeyJustPressed(ebiten.KeyV) {
		c.app.paste()
	}
	if ctrl() && inpututil.IsKeyJustPressed(ebiten.KeyA) {
		c.app.selectAll()
	}
	if inpututil.IsKeyJustPressed(ebiten.KeyDelete) {
		c.app.deleteForward()
	}
	if inpututil.IsKeyJustPressed(ebiten.KeyBackspace) {
		c.app.backspace()
	}
	c.app.typed(ebiten.AppendInputChars(nil))
	c.app.editKeys()

	if !click {
		return
	}
	for i := 0; i < 2; i++ {
		px, py, pw, ph := render.PreferenceRect(i)
		if render.Hit(x, y, px, py, pw, ph) {
			if i == 0 {
				c.app.prefs.Muted = !c.app.prefs.Muted
			} else {
				c.app.prefs.ReducedMotion = !c.app.prefs.ReducedMotion
			}
			if err := c.app.prefs.Save(c.app.dir); err != nil {
				c.app.say("Could not save display preferences.", true)
			}
			return
		}
	}
	if cx, cy, cw, ch := render.CloseRect(); render.Hit(x, y, cx, cy, cw, ch) {
		c.app.show(screenLobby)
		return
	}
	for i := range bridgeconn.Networks {
		if nx, ny, nw, nh := render.NetworkRect(i); render.Hit(x, y, nx, ny, nw, nh) {
			c.app.setNetwork(bridgeconn.Networks[i])
			return
		}
	}
	for i := 0; i < fieldCount; i++ {
		if fx, fy, fw, fh := render.FieldRect(i); render.Hit(x, y, fx, fy, fw, fh) {
			c.app.setFocus(i)
			return
		}
	}
	for i := range render.ButtonLabels {
		if bx, by, bw, bh := render.ButtonRect(i); render.Hit(x, y, bx, by, bw, bh) {
			switch i {
			case 0:
				c.app.connect()
			case 1:
				c.app.disconnect()
			case 2:
				c.app.save()
			}
			return
		}
	}
}

func (c *client) tableInputAt(x, y float64, click bool) {
	if inpututil.IsKeyJustPressed(ebiten.KeyEscape) {
		if c.confirmVoid {
			c.confirmVoid = false
			return
		}
		c.app.show(screenLobby)
		return
	}
	if !click {
		return
	}
	if bx, by, bw, bh := render.TableBackRect(); render.Hit(x, y, bx, by, bw, bh) {
		c.app.show(screenLobby)
		return
	}
	if c.displayed.Done {
		if bx, by, bw, bh := render.ReceiptRect(); render.Hit(x, y, bx, by, bw, bh) {
			c.app.openReceipt()
			return
		}
	}
	if view, ok := c.app.table(); ok && view.CanAbandon {
		if bx, by, w, h := render.AbandonRect(); render.Hit(x, y, bx, by, w, h) {
			if !c.confirmVoid {
				c.confirmVoid = true
				return
			}
			c.confirmVoid = false
			c.app.abandon()
			return
		}
	}
	if c.hover >= 0 && render.ColumnAt(x, y) >= 0 && !c.presenter.Busy() && c.controlFocus < 0 && !c.confirmVoid {
		c.app.play(c.hover)
	}
}

func ctrl() bool {
	return ebiten.IsKeyPressed(ebiten.KeyControlLeft) || ebiten.IsKeyPressed(ebiten.KeyControlRight)
}

func (c *client) Draw(screen *ebiten.Image) {
	c.gpu.Target = screen
	if c.cover {
		reduced := false
		if c.app != nil {
			reduced = c.app.prefs.ReducedMotion
		}
		render.DrawCover(c.gpu, render.Cover{Ready: c.ready, Error: c.loadError, Frame: c.frame, Reduced: reduced})
		return
	}

	if c.fixture != nil {
		view := c.displayed
		view.Hover = c.hover
		render.DrawTable(c.gpu, view)
		return
	}

	switch c.app.currentScreen() {
	case screenSeating:
		if view, ok := c.app.seating(); ok {
			render.DrawTableLobby(c.gpu, view)
			break
		}
		render.DrawLobby(c.gpu, c.app.lobby())
	case screenTable:
		if view, ok := c.app.table(); ok {
			if c.displayed.MatchID == view.MatchID {
				view = c.displayed
			}
			view.Hover = c.hover
			if c.confirmVoid {
				view.Status = "End as void? No opponent stake is forfeited. Press End again to confirm; Esc cancels."
			}
			render.DrawTable(c.gpu, view)
			break
		}
		render.DrawLobby(c.gpu, c.app.lobby())
	case screenSettings:
		render.DrawLobby(c.gpu, c.app.lobby())
		render.DrawSettings(c.gpu, c.app.settings())
	case screenHelp:
		render.DrawHelp(c.gpu, c.app.lobby().Network)
	case screenReceipt:
		if v, ok := c.app.table(); ok {
			render.DrawReceipt(c.gpu, v, c.app.exportStatus)
		}
	default:
		render.DrawLobby(c.gpu, c.app.lobby())
	}
	c.drawFocus()
}

func (c *client) Layout(w, h int) (int, int) {
	render.SetSize(w, h)
	return int(render.Width), int(render.Height)
}

func main() {
	cfg, err := appconfig.Load(os.Args[1:])
	if err != nil {
		// go-flags carries the help text in the error rather than printing
		// it, so asking for help and getting silence is the default unless
		// this prints it.
		var fe *flags.Error
		if errors.As(err, &fe) && fe.Type == flags.ErrHelp {
			fmt.Println(fe.Message)
			return
		}
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	if cfg.DebugLevel == "show" {
		fmt.Println("Supported subsystems:", logging.Subsystems())
		return
	}

	logs, err := logging.Open(cfg.LogDir, cfg.DebugLevel, cfg.LogSize, cfg.MaxLogFiles, os.Stdout)
	if err != nil {
		fmt.Fprintln(os.Stderr, "could not open the log:", err)
		os.Exit(1)
	}
	defer logs.Close()
	bridgeconn.UseLogger(logs.Logger("BRDG"))
	session.UseLogger(logs.Logger("SESS"))
	log := logs.Logger("FOUR")

	if cfg.Created {
		log.Infof("wrote a new configuration at %s", cfg.ConfigFile)
	}
	log.Infof("%s starting, profile %s", appconfig.AppName, cfg.AppData)

	c := &client{gpu: render.NewGPU(), hover: -1, controlFocus: -1}
	c.sounds.prepare()
	if !cfg.SkipCover && !cfg.Settings && !cfg.DevBoard {
		_ = art.Cover()
		c.cover = true
		c.loadResult = make(chan bool, 1)
		go func() { c.loadResult <- art.Preload() }()
	}
	if cfg.DevBoard {
		fixture, err := openFixture()
		if err != nil {
			log.Errorf("%v", err)
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
		c.fixture = fixture
	} else {
		a, err := newApp(cfg, logs.Logger("SDK"))
		if err != nil {
			log.Errorf("%v", err)
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
		c.app = a
		if cfg.Settings {
			a.show(screenSettings)
		}
		if cfg.Connect {
			a.connect()
		}
	}

	ebiten.SetWindowSize(1280, 800)
	ebiten.SetWindowSizeLimits(960, 640, -1, -1)
	ebiten.SetWindowTitle(render.BrandName)
	ebiten.SetWindowResizingMode(ebiten.WindowResizingModeEnabled)
	if err := ebiten.RunGame(c); err != nil && !errors.Is(err, ebiten.Termination) {
		log.Errorf("%v", err)
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

// play sends this peer's move without blocking the frame that asked for it.
func (a *app) play(column int) {
	v, ok := a.table()
	if !ok || v.Stale || v.Pending || v.Done || v.Turn != v.Seat {
		return
	}
	a.mu.Lock()
	game, sid := a.game, a.sid
	if game == nil || sid == "" {
		a.mu.Unlock()
		return
	}
	if a.submitting[sid] {
		a.mu.Unlock()
		return
	}
	a.submitting[sid] = true
	a.mu.Unlock()
	go func() {
		defer func() { a.mu.Lock(); a.submitting[sid] = false; a.mu.Unlock() }()
		if err := game.Play(context.Background(), sid, uint8(column)); err != nil {
			a.tableSay(sid, err.Error())
			return
		}
		a.tableSay(sid, "")
	}()
}

func (a *app) currentScreen() screen {
	a.mu.Lock()
	defer a.mu.Unlock()
	return a.screen
}

func (a *app) show(s screen) {
	a.mu.Lock()
	a.screen = s
	if s != screenSettings {
		a.focus = -1
	}
	a.mu.Unlock()
}

func (a *app) hasTable() bool {
	a.mu.Lock()
	defer a.mu.Unlock()
	return a.sid != "" || a.cached.MatchID != ""
}

func (a *app) setFocus(i int) {
	a.mu.Lock()
	a.focus = i
	a.selected = false
	switch i {
	case fieldHost:
		a.cursor = len(a.cfg.Host)
	case fieldPort:
		a.cursor = len(a.cfg.Port)
	}
	a.mu.Unlock()
}

func (a *app) nextField() {
	a.mu.Lock()
	a.focus = (a.focus + 1) % fieldCount
	a.mu.Unlock()
}

func (a *app) setNetwork(n string) {
	a.mu.Lock()
	if a.connected {
		a.mu.Unlock()
		a.say("Disconnect before changing the bridge network.", true)
		return
	}
	a.cfg.Network = n
	a.mu.Unlock()
	a.say("Network set to "+n+". Connect to prove the bridge agrees.", false)
}
