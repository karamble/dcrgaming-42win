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

	"github.com/decred/slog"
	"github.com/karamble/dcrgaming-42win/assets/art"
	"github.com/karamble/dcrgaming-42win/internal/appconfig"
	"github.com/karamble/dcrgaming-42win/internal/bridgeconn"
	"github.com/karamble/dcrgaming-42win/internal/clipboard"
	"github.com/karamble/dcrgaming-42win/internal/session"
	"github.com/karamble/dcrgaming-42win/internal/tablelobby"
	"github.com/karamble/dcrgaming-42win/internal/uiprefs"
	"github.com/karamble/dcrgaming-42win/pkg/render"
	"github.com/karamble/dcrgaming-sdk/pkg/identity"
	sdk "github.com/karamble/dcrgaming-sdk/pkg/runtime"
)

type screen int

const (
	screenLobby screen = iota
	screenSettings
	// screenSeating is the table being built: who has paid what, and what
	// the table is waiting for.
	screenSeating
	screenTable
	screenHelp
	screenReceipt
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
	details       bool
	tablePage     int
	prefs         uiprefs.Preferences
	chainFresh    bool
	tableErrors   map[string]string
	submitting    map[string]bool
	refreshFailed map[string]bool
	cached        render.View
	receipt       *session.Receipt
	exportStatus  string
	selected      bool
	cursor        int
	fieldErrors   map[int]string
	dir           string
	path          string
	sdkLog        slog.Logger

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
		tableErrors: map[string]string{}, submitting: map[string]bool{}, refreshFailed: map[string]bool{},
	}
	a.prefs, _ = uiprefs.Load(opts.AppData)
	if err != nil {
		a.status, a.warn = err.Error(), true
	}
	return a, nil
}

// lobby is what the opening screen draws.
func (a *app) lobby() render.Lobby {
	a.mu.Lock()
	defer a.mu.Unlock()
	table := a.sid
	if table == "" {
		table = a.cached.MatchID
	}
	return render.Lobby{
		Backdrop:   art.LobbyBackdrop(),
		Network:    a.cfg.Network,
		Connected:  a.connected && a.chainFresh,
		Configured: a.cfg.Complete(),
		Status:     a.status,
		Table:      table,
		Build:      "DEVELOPMENT BUILD",
		Tables:     a.tableNamesLocked(),
		TablePage:  a.tablePage,
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
	for i := range fields {
		fields[i].Error = a.fieldErrors[i]
	}
	if a.focus >= 0 && a.focus < len(fields) {
		fields[a.focus].Focused = true
		fields[a.focus].Selected = a.selected
		fields[a.focus].Cursor = a.cursor
	}
	return render.Settings{
		Network: a.cfg.Network, Fields: fields,
		Status: a.status, Warn: a.warn, Connected: a.connected,
		Muted: a.prefs.Muted, Reduced: a.prefs.ReducedMotion, Busy: a.busy,
	}
}

// table is what the board screen draws, or false when there is no table.
func (a *app) table() (render.View, bool) {
	a.mu.Lock()
	game, sid, network, connected, height := a.game, a.sid, a.cfg.Network, a.connected, a.height
	stale := !connected || !a.chainFresh || a.refreshFailed[sid]
	status, pending, cached := a.tableErrors[sid], a.submitting[sid], a.cached
	a.mu.Unlock()
	if game == nil || sid == "" {
		if cached.MatchID != "" {
			cached.Stale = true
			cached.Connected = false
			cached.CanAbandon = false
			return cached, true
		}
		return render.View{}, false
	}
	v, ok := game.View(sid)
	if !ok {
		return render.View{}, false
	}
	out := render.View{
		Network: network, Entries: v.Entries, Results: v.Results, Stake: v.BuyIn, Bond: v.Bond,
		StakeCheck: v.StakeCheck, BondCheck: v.BondCheck, Blocked: v.Blocked,
		Stale: stale, Pending: pending, Status: status, MatchDeadline: v.MatchDeadline, Expired: v.Expired,
		MatchID: v.MatchID, Grid: v.Grid, Seat: v.Seat, Board: v.Board,
		Turn: v.Turn, Score: v.Score, Moves: v.Moves, Done: v.Done,
		Won: v.Won, Winner: v.Winner, Connected: connected && !stale, Hover: -1,
		Abandoned: v.Abandoned, Stalled: v.Stalled, Deadline: v.Deadline,
		Height: height,
	}
	out.CanAbandon = !stale && !v.Done && !v.Abandoned &&
		((v.Turn != v.Seat && v.Deadline > 0 && height >= v.Deadline) || (v.MatchDeadline > 0 && height >= v.MatchDeadline))
	if !v.Done {
		out.Notice = "payout needs both seats to sign · " + network
	}
	if stale {
		out.Notice = "Connection unavailable · cached state · reconnect from Settings"
	}
	a.mu.Lock()
	a.cached = out
	a.mu.Unlock()
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
	if a.connected {
		return
	}
	delete(a.fieldErrors, i)
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
		if r >= ' ' && r < 127 && (len(value) < 64 || a.selected) {
			if a.selected {
				value = ""
				a.cursor = 0
				a.selected = false
			}
			a.cursor = min(a.cursor, len(value))
			value = value[:a.cursor] + string(r) + value[a.cursor:]
			a.cursor++
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
	if a.selected || (focus != fieldHost && focus != fieldPort) {
		a.setField(focus, "")
		a.cursor = 0
		a.selected = false
		a.say("Cleared the "+strings.ToLower(a.settings().Fields[focus].Label)+" field.", false)
		return
	}
	if v := a.field(focus); v != "" && a.cursor > 0 {
		a.cursor = min(a.cursor, len(v))
		a.setField(focus, v[:a.cursor-1]+v[a.cursor:])
		a.cursor--
	}
}

// clearField clears an explicit selection or a pasted credential.
func (a *app) clearField() {
	a.mu.Lock()
	focus := a.focus
	a.mu.Unlock()
	if focus < 0 {
		return
	}
	a.setField(focus, "")
	a.cursor = 0
	a.selected = false
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
		if len(text) > 64 {
			a.say("Address and port accept at most 64 characters.", true)
			return
		}
		if !a.selected {
			old := a.field(focus)
			a.cursor = min(a.cursor, len(old))
			text = old[:a.cursor] + text + old[a.cursor:]
		}
		if len(text) > 64 {
			return
		}
		a.cursor = len(text)
	}
	if len(text) > bridgeconn.MaxPEM {
		a.say("Credential exceeds 32 KiB.", true)
		return
	}
	a.selected = false
	a.setField(focus, text)
	a.say("Pasted into the "+strings.ToLower(a.settings().Fields[focus].Label)+" field.", false)
}

func (a *app) save() {
	if !a.validateFields() {
		return
	}
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
	if !a.validateFields() {
		return
	}
	a.mu.Lock()
	if a.busy || (a.connected && a.chainFresh) {
		a.mu.Unlock()
		return
	}
	if a.connected {
		a.mu.Unlock()
		a.disconnect()
		a.connect()
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

func (a *app) validateFields() bool {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.fieldErrors = a.cfg.FieldErrors()
	if len(a.fieldErrors) > 0 {
		a.status = "Check the highlighted fields, then connect again."
		a.warn = true
		return false
	}
	return true
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
	seed, err := identity.Load(a.dir)
	if err != nil {
		cancel()
		bridge.Close()
		return fmt.Errorf("could not open this profile's identity: %w", err)
	}
	// The runtime keeps its own records under the profile directory and takes
	// the chain from the bridge, which said which one it was on when it said
	// hello.
	rt, err := sdk.Open(sdk.Config{
		Rules: game, Bridge: bridge, Identity: seed, Dir: a.dir,
		SeatTags: session.SeatTags, Log: a.sdkLog,
	})
	if err != nil {
		cancel()
		bridge.Close()
		return err
	}
	game.Bind(rt)

	a.mu.Lock()
	a.game, a.rt, a.cancel, a.connected = game, rt, cancel, true
	a.mu.Unlock()

	go func() {
		if err := rt.Run(ctx); err != nil && ctx.Err() == nil {
			a.mu.Lock()
			if a.rt == rt {
				a.chainFresh = false
				a.status = "Bridge connection interrupted. Reconnect from Settings."
				a.warn = true
			}
			a.mu.Unlock()
		}
	}()
	go a.follow(ctx, rt)
	return nil
}

// follow keeps the runtime's idea of the chain current, which is what moves a
// table through its deadlines.
func (a *app) follow(ctx context.Context, rt *sdk.Runtime) {
	t := time.NewTicker(15 * time.Second)
	defer t.Stop()
	for {
		tip, err := rt.Chain(ctx)
		a.mu.Lock()
		if a.rt != rt || ctx.Err() != nil {
			a.mu.Unlock()
			return
		}
		if err == nil {
			a.height = uint32(tip.Height)
			a.chainFresh = true
		} else {
			a.chainFresh = false
		}
		a.mu.Unlock()
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
	_, _ = a.table() // capture the latest board before its session is closed
	a.mu.Lock()
	priorGame, priorSID := a.game, a.sid
	network := a.cfg.Network
	a.mu.Unlock()
	if priorGame != nil && priorSID != "" {
		if r, err := priorGame.Receipt(priorSID); err == nil {
			r.Network = network
			a.mu.Lock()
			a.receipt = &r
			a.mu.Unlock()
		}
	}
	a.mu.Lock()
	cancel, game := a.cancel, a.game
	a.cancel, a.rt, a.game, a.connected, a.sid = nil, nil, nil, false, ""
	a.chainFresh = false
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
			a.tableSay(sid, err.Error())
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
			a.tableSay(sid, err.Error())
			return
		}
		a.tableSay(sid, "Match void. Check settlement or recovery in dcrpulse.")
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
			a.tableSay(sid, err.Error())
			a.mu.Lock()
			a.refreshFailed[sid] = true
			a.mu.Unlock()
			continue
		}
		a.mu.Lock()
		a.refreshFailed[sid] = false
		a.mu.Unlock()
		if err := game.Start(ctx, sid); err != nil {
			a.tableSay(sid, err.Error())
		}
	}
}

// seating is the table-formation screen's view, or false when there is no table.
func (a *app) seating() (tablelobby.View, bool) {
	a.mu.Lock()
	game, sid, height, connected := a.game, a.sid, a.height, a.connected && a.chainFresh && !a.refreshFailed[a.sid]
	busy, status, details, network := a.busy, a.tableErrors[sid], a.details, a.cfg.Network
	a.mu.Unlock()
	if game == nil || sid == "" {
		return tablelobby.View{}, false
	}
	// Evidence is stale whenever the bridge is gone or the chain height is
	// unknown: checks made before that are not checks made now.
	v, ok := game.Lobby(sid, height, !connected || height == 0)
	v.Details, v.Network = details, network
	if busy {
		v.CanFund = false
		v.NextStep = "Awaiting dcrpulse approval"
		v.NextDetail = "A request is in progress. Review it in dcrpulse → Gaming."
	}
	if status != "" {
		v.Error = status
	}
	return v, ok
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
	if game == nil || sid == "" || busy {
		a.mu.Unlock()
		return
	}
	a.busy = true
	a.mu.Unlock()
	a.tableSay(sid, "Requesting the stake. Approve it in dcrpulse.")

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
				a.tableSay(sid, "A stake request was sent and its outcome is unknown. Do NOT fund again — "+
					"find it in dcrpulse → Gaming and reconcile it there.")
				return
			}
			a.tableSay(sid, err.Error())
			return
		}
		a.tableSay(sid, "")
	}()
}

func (a *app) tableSay(sid, message string) { a.mu.Lock(); a.tableErrors[sid] = message; a.mu.Unlock() }

// Called with app lock held; the session never calls back into the app.
func (a *app) tableNamesLocked() []string {
	if a.game == nil {
		return nil
	}
	return a.game.Tables()
}

func (a *app) openReceipt() {
	a.mu.Lock()
	game, sid := a.game, a.sid
	a.mu.Unlock()
	if game != nil && sid != "" {
		r, err := game.Receipt(sid)
		if err != nil {
			a.tableSay(sid, err.Error())
			return
		}
		a.mu.Lock()
		a.receipt = &r
		r.Network = a.cfg.Network
		a.mu.Unlock()
	}
	a.show(screenReceipt)
}

func (a *app) exportReceipt() {
	a.openReceipt()
	a.mu.Lock()
	r := a.receipt
	a.mu.Unlock()
	if r == nil {
		return
	}
	path, err := r.Export(filepath.Join(a.dir, "exports"))
	note := "Saved: " + path
	if err != nil {
		note = "Could not export receipt: " + err.Error()
	}
	a.mu.Lock()
	a.exportStatus = note
	a.mu.Unlock()
}

func (a *app) returnToTable() {
	a.mu.Lock()
	cached := a.game == nil && a.cached.MatchID != ""
	a.mu.Unlock()
	if cached {
		a.show(screenTable)
	} else {
		a.show(screenSeating)
	}
}
