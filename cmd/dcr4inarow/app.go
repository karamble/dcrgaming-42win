//go:build desktop

package main

import (
	"context"
	"errors"
	"fmt"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/decred/dcrd/chaincfg/v3"
	"github.com/decred/dcrd/txscript/v4/stdaddr"
	"github.com/decred/slog"
	"github.com/karamble/dcr4inarow/assets/art"
	"github.com/karamble/dcr4inarow/internal/appconfig"
	"github.com/karamble/dcr4inarow/internal/bridgeconn"
	"github.com/karamble/dcr4inarow/internal/clipboard"
	"github.com/karamble/dcr4inarow/internal/session"
	"github.com/karamble/dcr4inarow/internal/tablelobby"
	"github.com/karamble/dcr4inarow/pkg/render"
	"github.com/karamble/dcrgaming-sdk/pkg/identity"
	sdk "github.com/karamble/dcrgaming-sdk/pkg/runtime"
	"github.com/karamble/dcrgaming-sdk/pkg/spend"
)

type screen int

const (
	screenLobby screen = iota
	screenSettings
	// screenSeating is the table being built: who has paid what, and what
	// the table is waiting for.
	screenSeating
	screenTable
)

// Field indices, in the order the settings screen draws them.
const (
	fieldHost = iota
	fieldPort
	fieldClientCert
	fieldClientKey
	fieldBridgeCert
	fieldCount
)

// app is the client: the settings, the connection, and which screen is up.
//
// Everything the screens read goes through its lock, and everything slow -
// dialling a bridge, waiting for a block - happens off the draw loop. A frame
// that blocked on a network call would freeze the window.
type app struct {
	dir    string
	path   string
	sdkLog slog.Logger

	mu        sync.Mutex
	cfg       bridgeconn.Config
	game      *session.Game
	rt        *sdk.Runtime
	cancel    context.CancelFunc
	connected bool
	busy      bool
	status    string
	warn      bool
	sid       string
	// height is the chain tip the follow loop last saw, so a screen can show
	// how long a stalled seat has left without asking the bridge to draw a
	// frame.
	height uint32

	screen screen
	focus  int
}

func newApp(opts *appconfig.Config, sdkLog slog.Logger) (*app, error) {
	cfg, err := bridgeconn.Load(opts.BridgeConfig)
	a := &app{
		dir: opts.AppData, path: opts.BridgeConfig,
		cfg: cfg, focus: -1, sdkLog: sdkLog,
	}
	if err != nil {
		a.status, a.warn = err.Error(), true
	}
	return a, nil
}

// lobby is what the opening screen draws.
func (a *app) lobby() render.Lobby {
	a.mu.Lock()
	defer a.mu.Unlock()
	return render.Lobby{
		Backdrop:   art.LobbyBackdrop(),
		Network:    a.cfg.Network,
		Connected:  a.connected,
		Configured: a.cfg.Complete(),
		Status:     a.status,
		Table:      a.sid,
		Build:      "DEVELOPMENT BUILD",
	}
}

// settings is what the settings panel draws. The private key's value is passed
// with Masked set, and the panel never renders it.
func (a *app) settings() render.Settings {
	a.mu.Lock()
	defer a.mu.Unlock()
	fields := []render.Field{
		{Label: "Bridge address", Value: a.cfg.Host, Placeholder: "127.0.0.1"},
		{Label: "Gaming port", Value: a.cfg.Port, Placeholder: "8443"},
		{Label: "Client certificate", Value: a.cfg.ClientCert, Placeholder: "Click here and paste the complete PEM text"},
		{Label: "Client private key", Value: a.cfg.ClientKey, Masked: true, Placeholder: "Click here and paste the complete PEM text"},
		{Label: "Bridge certificate", Value: a.cfg.BridgeCert, Placeholder: "Click here and paste the complete PEM text"},
	}
	if a.focus >= 0 && a.focus < len(fields) {
		fields[a.focus].Focused = true
	}
	return render.Settings{
		Network: a.cfg.Network, Fields: fields,
		Status: a.status, Warn: a.warn, Connected: a.connected,
	}
}

// table is what the board screen draws, or false when there is no table.
func (a *app) table() (render.View, bool) {
	a.mu.Lock()
	game, sid, network, connected, height := a.game, a.sid, a.cfg.Network, a.connected, a.height
	a.mu.Unlock()
	if game == nil || sid == "" {
		return render.View{}, false
	}
	v, ok := game.View(sid)
	if !ok {
		return render.View{}, false
	}
	out := render.View{
		MatchID: v.MatchID, Grid: v.Grid, Seat: v.Seat, Board: v.Board,
		Turn: v.Turn, Score: v.Score, Moves: v.Moves, Done: v.Done,
		Won: v.Won, Winner: v.Winner, Connected: connected, Hover: -1,
		Abandoned: v.Abandoned, Stalled: v.Stalled, Deadline: v.Deadline,
		Height: height,
	}
	out.CanAbandon = !v.Done && !v.Abandoned && v.Turn != v.Seat &&
		v.Deadline > 0 && height >= v.Deadline
	if !v.Done {
		out.Notice = "payout needs both seats to sign · " + network
	}
	return out, true
}

func (a *app) say(status string, warn bool) {
	a.mu.Lock()
	a.status, a.warn = status, warn
	a.mu.Unlock()
}

// setField writes one settings field.
func (a *app) setField(i int, value string) {
	a.mu.Lock()
	defer a.mu.Unlock()
	switch i {
	case fieldHost:
		a.cfg.Host = strings.TrimSpace(value)
	case fieldPort:
		a.cfg.Port = strings.TrimSpace(value)
	case fieldClientCert:
		a.cfg.ClientCert = value
	case fieldClientKey:
		a.cfg.ClientKey = value
	case fieldBridgeCert:
		a.cfg.BridgeCert = value
	}
}

func (a *app) field(i int) string {
	a.mu.Lock()
	defer a.mu.Unlock()
	switch i {
	case fieldHost:
		return a.cfg.Host
	case fieldPort:
		return a.cfg.Port
	case fieldClientCert:
		return a.cfg.ClientCert
	case fieldClientKey:
		return a.cfg.ClientKey
	case fieldBridgeCert:
		return a.cfg.BridgeCert
	}
	return ""
}

// typed is only accepted by the address and port. A PEM is pasted, never typed,
// and a field that accepted keystrokes into a credential would invite somebody
// to try.
func (a *app) typed(runes []rune) {
	a.mu.Lock()
	focus := a.focus
	a.mu.Unlock()
	if focus != fieldHost && focus != fieldPort {
		return
	}
	value := a.field(focus)
	for _, r := range runes {
		if r >= ' ' && r < 127 && len(value) < 64 {
			value += string(r)
		}
	}
	a.setField(focus, value)
}

// backspace removes one character from a typed field.
//
// A pasted credential is thousands of characters and cannot be edited a
// character at a time, so backspace clears it outright - which is what a person
// pressing it on a PEM field is asking for.
func (a *app) backspace() {
	a.mu.Lock()
	focus := a.focus
	a.mu.Unlock()
	if focus < 0 {
		return
	}
	if focus != fieldHost && focus != fieldPort {
		a.setField(focus, "")
		a.say("Cleared the "+strings.ToLower(a.settings().Fields[focus].Label)+" field.", false)
		return
	}
	if v := a.field(focus); v != "" {
		a.setField(focus, v[:len(v)-1])
	}
}

// clearField empties whatever is focused. Delete and Ctrl+A both do it, so a
// field can be replaced without selecting anything.
func (a *app) clearField() {
	a.mu.Lock()
	focus := a.focus
	a.mu.Unlock()
	if focus < 0 {
		return
	}
	a.setField(focus, "")
	a.say("Cleared the "+strings.ToLower(a.settings().Fields[focus].Label)+" field.", false)
}

func (a *app) paste() {
	a.mu.Lock()
	focus := a.focus
	a.mu.Unlock()
	if focus < 0 {
		return
	}
	text, err := clipboard.Paste()
	if err != nil {
		a.say(err.Error(), true)
		return
	}
	if focus == fieldHost || focus == fieldPort {
		text = strings.TrimSpace(text)
	}
	a.setField(focus, text)
	a.say("Pasted into the "+strings.ToLower(a.settings().Fields[focus].Label)+" field.", false)
}

func (a *app) save() {
	a.mu.Lock()
	cfg := a.cfg
	a.mu.Unlock()
	if err := bridgeconn.Save(a.path, cfg); err != nil {
		a.say(err.Error(), true)
		return
	}
	a.say("Settings saved. The private key is stored unencrypted; protect this file.", false)
}

// connect dials the bridge and starts a runtime on it.
//
// Runs off the draw loop, and refuses to start a second attempt while one is in
// flight: two runtimes over one profile would fight over its stores, and the
// SDK refuses that anyway.
func (a *app) connect() {
	a.mu.Lock()
	if a.busy || a.connected {
		a.mu.Unlock()
		return
	}
	a.busy = true
	cfg := a.cfg
	a.mu.Unlock()
	a.say("Connecting…", false)

	go func() {
		defer func() {
			a.mu.Lock()
			a.busy = false
			a.mu.Unlock()
		}()
		if err := a.start(cfg); err != nil {
			a.say(err.Error(), true)
			return
		}
		// Credentials that just proved themselves against the bridge are
		// remembered without being asked. Making this a separate button was
		// a trap: the obvious action is Connect, and somebody who only ever
		// presses it re-pastes three PEM blocks on every launch - and
		// --connect at startup has nothing to connect with.
		note := "Connected. Accept an invitation in dcrpulse to take a seat."
		if err := bridgeconn.Save(a.path, cfg); err != nil {
			note = "Connected, but the credentials could not be saved: " + err.Error()
		}
		a.say(note, false)
	}()
}

func (a *app) start(cfg bridgeconn.Config) error {
	game, err := session.New(filepath.Join(a.dir, "books"))
	if err != nil {
		return err
	}
	ctx, cancel := context.WithCancel(context.Background())

	bridge, err := bridgeconn.Connect(ctx, cfg, game.Identity())
	if err != nil {
		cancel()
		return err
	}
	store, err := spend.FileStore(filepath.Join(a.dir, "spends.json"))
	if err != nil {
		cancel()
		bridge.Close()
		return fmt.Errorf("could not open the payment record: %w", err)
	}
	book, err := spend.OpenBook(store)
	if err != nil {
		cancel()
		bridge.Close()
		return fmt.Errorf("could not open the payment record: %w", err)
	}
	tables, err := sdk.NewFileTableStore(filepath.Join(a.dir, "tables"))
	if err != nil {
		cancel()
		bridge.Close()
		return fmt.Errorf("could not open the table record: %w", err)
	}
	seed, err := identity.Load(a.dir)
	if err != nil {
		cancel()
		bridge.Close()
		return fmt.Errorf("could not open this profile's identity: %w", err)
	}
	rt, err := sdk.New(sdk.Config{
		Rules: game, Bridge: bridge, Book: book, Identity: seed,
		SeatTags: session.SeatTags, Params: params(cfg.Network), Tables: tables,
		Log: a.sdkLog,
	})
	if err != nil {
		cancel()
		bridge.Close()
		return err
	}
	if _, err := rt.ResumeWithReport(); err != nil {
		cancel()
		rt.Close()
		bridge.Close()
		return fmt.Errorf("could not resume saved tables: %w", err)
	}
	game.Bind(rt)

	a.mu.Lock()
	a.game, a.rt, a.cancel, a.connected = game, rt, cancel, true
	a.mu.Unlock()

	go func() { _ = rt.Run(ctx) }()
	go a.follow(ctx, rt)
	return nil
}

// follow keeps the runtime's idea of the chain current, which is what moves a
// table through its deadlines.
func (a *app) follow(ctx context.Context, rt *sdk.Runtime) {
	t := time.NewTicker(15 * time.Second)
	defer t.Stop()
	for {
		if tip, err := rt.Chain(ctx); err == nil {
			rt.Tick(ctx, tip.Height)
			a.mu.Lock()
			a.height = uint32(tip.Height)
			a.mu.Unlock()
		}
		a.prepareTables(ctx)
		a.settleFinished(ctx)
		select {
		case <-ctx.Done():
			return
		case <-t.C:
		}
	}
}

func (a *app) disconnect() {
	a.mu.Lock()
	cancel, game := a.cancel, a.game
	a.cancel, a.rt, a.game, a.connected, a.sid = nil, nil, nil, false, ""
	a.mu.Unlock()
	if cancel != nil {
		cancel()
	}
	if game != nil {
		_ = game.Close()
	}
	a.say("Disconnected.", false)
}

// adopt notices a table the runtime has seated, so the lobby can offer it.
func (a *app) adopt() {
	a.mu.Lock()
	game, rt, sid := a.game, a.rt, a.sid
	a.mu.Unlock()
	if game == nil || rt == nil || sid != "" {
		return
	}
	_ = rt
	for _, open := range game.Tables() {
		a.mu.Lock()
		a.sid = open
		a.mu.Unlock()
		return
	}
}

// params is the chain the bridge said it is on.
//
// Chosen from the authenticated network rather than compiled in: the scripts
// and addresses a table builds depend on it, and a game that assumed mainnet
// would build unspendable ones everywhere else.
func params(network string) stdaddr.AddressParams {
	switch network {
	case "testnet3":
		return chaincfg.TestNet3Params()
	case "simnet":
		return chaincfg.SimNetParams()
	default:
		return chaincfg.MainNetParams()
	}
}

// settleFinished proposes a payout for any table whose match is over.
//
// Off the draw loop and off the runtime's delivery path, because it asks the
// bridge and a person has to answer. Settle is idempotent, so running on a
// timer costs nothing but the check.
func (a *app) settleFinished(ctx context.Context) {
	a.mu.Lock()
	game := a.game
	a.mu.Unlock()
	if game == nil {
		return
	}
	for _, sid := range game.Tables() {
		view, ok := game.View(sid)
		if !ok || !view.Done {
			continue
		}
		if err := game.Settle(ctx, sid); err != nil {
			a.say(err.Error(), true)
		}
	}
}

// abandon gives up on a stalled match, off the frame that asked for it.
func (a *app) abandon() {
	a.mu.Lock()
	game, sid := a.game, a.sid
	a.mu.Unlock()
	if game == nil || sid == "" {
		return
	}
	go func() {
		if err := game.Abandon(context.Background(), sid); err != nil {
			a.say(err.Error(), true)
			return
		}
		a.say("The match was given up on. Every seat takes back its own stake.", false)
	}()
}

// prepareTables verifies each open table's admission bonds and refreshes its
// deposits, which is what makes a seated table playable.
func (a *app) prepareTables(ctx context.Context) {
	a.mu.Lock()
	game := a.game
	a.mu.Unlock()
	if game == nil {
		return
	}
	// Every table, every tick. Stopping once a table is ready would stop the
	// deposit checks, the settlement watch and the notice that the table was
	// closed in dcrpulse - all of which are learned here and nowhere else.
	for _, sid := range game.Tables() {
		if err := game.Prepare(ctx, sid); err != nil {
			a.say(err.Error(), true)
			continue
		}
		if err := game.Start(ctx, sid); err != nil {
			a.say(err.Error(), true)
		}
	}
}

// seating is the table-formation screen's view, or false when there is no table.
func (a *app) seating() (tablelobby.View, bool) {
	a.mu.Lock()
	game, sid, height, connected := a.game, a.sid, a.height, a.connected
	a.mu.Unlock()
	if game == nil || sid == "" {
		return tablelobby.View{}, false
	}
	// Evidence is stale whenever the bridge is gone or the chain height is
	// unknown: checks made before that are not checks made now.
	return game.Lobby(sid, height, !connected || height == 0)
}

// follow the table through its own lifecycle: the seating screen while it
// forms, the board once it is verified and playable.
func (a *app) followTable() {
	a.mu.Lock()
	game, sid, current := a.game, a.sid, a.screen
	a.mu.Unlock()
	if game == nil || sid == "" {
		return
	}
	// Playable, not Ready. Ready means the admission bonds are confirmed;
	// opening the board on that would push the player off the only screen
	// with a Fund action on it, before either seat had staked anything.
	playable, _ := game.Playable(sid)
	switch {
	case playable && current == screenSeating:
		a.show(screenTable)
	case !playable && current == screenTable && !game.Paid(sid):
		a.show(screenSeating)
	}
}

// fund asks the bridge for this seat's stake, off the frame that asked for it.
//
// Asked once. The runtime keeps the request open if the bridge could not
// answer, so a second press while one is outstanding must not become a second
// payment - which is why the button is drawn from CanFund, and CanFund is false
// as soon as the runtime has a record of this seat's stake.
func (a *app) fund() {
	a.mu.Lock()
	game, sid, busy := a.game, a.sid, a.busy
	a.mu.Unlock()
	if game == nil || sid == "" || busy {
		return
	}
	a.mu.Lock()
	a.busy = true
	a.mu.Unlock()
	a.say("Requesting the stake. Approve it in dcrpulse.", false)

	go func() {
		defer func() {
			a.mu.Lock()
			a.busy = false
			a.mu.Unlock()
		}()
		if err := game.Fund(context.Background(), sid); err != nil {
			if errors.Is(err, sdk.ErrUnresolvedPayment) {
				// The request went out and its answer never came back. It
				// may already have been paid, so it must not be asked
				// again: an operator reconciles it against the bridge's own
				// record instead.
				a.say("A stake request was sent and its outcome is unknown. Do NOT fund again — "+
					"find it in dcrpulse → Gaming and reconcile it there.", true)
				return
			}
			a.say(err.Error(), true)
			return
		}
		a.say("Stake requested. It is asked for once; approve it in dcrpulse.", false)
	}()
}
