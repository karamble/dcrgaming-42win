//go:build desktop

// Command dcr4inarow is the desktop client.
package main

import (
	"context"
	"errors"
	"fmt"
	"os"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/inpututil"
	flags "github.com/jessevdk/go-flags"
	"github.com/karamble/dcr4inarow/internal/appconfig"
	"github.com/karamble/dcr4inarow/internal/bridgeconn"
	"github.com/karamble/dcr4inarow/internal/logging"
	"github.com/karamble/dcr4inarow/internal/session"
	"github.com/karamble/dcr4inarow/internal/tablelobby"
	"github.com/karamble/dcr4inarow/pkg/render"
)

// source is a table the board screen can draw and click on. The app provides
// one; so does the local fixture, and the screen cannot tell them apart.
type source interface {
	View() render.View
	Play(column int) error
}

type client struct {
	app     *app
	fixture source
	gpu     *render.GPU
	hover   int
}

func (c *client) Update() error {
	mx, my := ebiten.CursorPosition()
	x, y := float64(mx), float64(my)
	c.hover = render.ColumnAt(x, y)
	click := inpututil.IsMouseButtonJustPressed(ebiten.MouseButtonLeft)

	if c.fixture != nil {
		if click && c.hover >= 0 {
			if err := c.fixture.Play(c.hover); err != nil {
				return nil
			}
		}
		return nil
	}

	c.app.adopt()
	c.app.followTable()
	switch c.app.currentScreen() {
	case screenLobby:
		c.lobbyInput(x, y, click)
	case screenSettings:
		c.settingsInput(x, y, click)
	case screenSeating:
		c.seatingInput(x, y, click)
	case screenTable:
		c.tableInput(click)
	}
	return nil
}

// seatingInput drives the table-formation screen. The only action it offers is
// the one the SDK offers: ask for the stake, once.
func (c *client) seatingInput(x, y float64, click bool) {
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
		case view.Stage == tablelobby.StageReady:
			c.app.show(screenTable)
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
	if gx, gy, gw, gh := render.GearRect(); render.Hit(x, y, gx, gy, gw, gh) {
		c.app.show(screenSettings)
		return
	}
	if bx, by, bw, bh := render.CallToActionRect(); render.Hit(x, y, bx, by, bw, bh) {
		if c.app.hasTable() {
			c.app.show(screenSeating)
			return
		}
		c.app.show(screenSettings)
	}
}

func (c *client) settingsInput(x, y float64, click bool) {
	if inpututil.IsKeyJustPressed(ebiten.KeyEscape) {
		c.app.show(screenLobby)
		return
	}
	if inpututil.IsKeyJustPressed(ebiten.KeyTab) {
		c.app.nextField()
	}
	if ctrl() && inpututil.IsKeyJustPressed(ebiten.KeyV) {
		c.app.paste()
	}
	if ctrl() && inpututil.IsKeyJustPressed(ebiten.KeyA) {
		c.app.clearField()
	}
	if inpututil.IsKeyJustPressed(ebiten.KeyDelete) {
		c.app.clearField()
	}
	if inpututil.IsKeyJustPressed(ebiten.KeyBackspace) {
		c.app.backspace()
	}
	c.app.typed(ebiten.AppendInputChars(nil))

	if !click {
		return
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

func (c *client) tableInput(click bool) {
	if inpututil.IsKeyJustPressed(ebiten.KeyEscape) {
		c.app.show(screenLobby)
		return
	}
	if !click {
		return
	}
	mx, my := ebiten.CursorPosition()
	if view, ok := c.app.table(); ok && view.CanAbandon {
		if x, y, w, h := render.AbandonRect(); render.Hit(float64(mx), float64(my), x, y, w, h) {
			c.app.abandon()
			return
		}
	}
	if c.hover >= 0 {
		c.app.play(c.hover)
	}
}

func ctrl() bool {
	return ebiten.IsKeyPressed(ebiten.KeyControlLeft) || ebiten.IsKeyPressed(ebiten.KeyControlRight)
}

func (c *client) Draw(screen *ebiten.Image) {
	c.gpu.Target = screen

	if c.fixture != nil {
		view := c.fixture.View()
		view.Hover = c.hover
		render.DrawTable(c.gpu, view)
		return
	}

	switch c.app.currentScreen() {
	case screenSeating:
		if view, ok := c.app.seating(); ok {
			render.DrawTableLobby(c.gpu, view)
			return
		}
		render.DrawLobby(c.gpu, c.app.lobby())
	case screenTable:
		if view, ok := c.app.table(); ok {
			view.Hover = c.hover
			render.DrawTable(c.gpu, view)
			return
		}
		render.DrawLobby(c.gpu, c.app.lobby())
	case screenSettings:
		render.DrawLobby(c.gpu, c.app.lobby())
		render.DrawSettings(c.gpu, c.app.settings())
	default:
		render.DrawLobby(c.gpu, c.app.lobby())
	}
}

func (c *client) Layout(int, int) (int, int) { return render.Width, render.Height }

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

	c := &client{gpu: render.NewGPU(), hover: -1}
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

	ebiten.SetWindowSize(render.Width, render.Height)
	ebiten.SetWindowTitle("dcr4inarow")
	ebiten.SetWindowResizingMode(ebiten.WindowResizingModeEnabled)
	if err := ebiten.RunGame(c); err != nil && !errors.Is(err, ebiten.Termination) {
		log.Errorf("%v", err)
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

// play sends this peer's move without blocking the frame that asked for it.
func (a *app) play(column int) {
	a.mu.Lock()
	game, sid := a.game, a.sid
	a.mu.Unlock()
	if game == nil || sid == "" {
		return
	}
	go func() {
		if err := game.Play(context.Background(), sid, uint8(column)); err != nil {
			a.say(err.Error(), true)
			return
		}
		a.say("", false)
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
	return a.sid != ""
}

func (a *app) setFocus(i int) {
	a.mu.Lock()
	a.focus = i
	a.mu.Unlock()
}

func (a *app) nextField() {
	a.mu.Lock()
	a.focus = (a.focus + 1) % fieldCount
	a.mu.Unlock()
}

func (a *app) setNetwork(n string) {
	a.mu.Lock()
	a.cfg.Network = n
	a.mu.Unlock()
	a.say("Network set to "+n+". Connect to prove the bridge agrees.", false)
}
