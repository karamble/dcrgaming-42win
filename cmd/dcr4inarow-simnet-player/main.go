// Command dcr4inarow-simnet-player is one seat, driven over HTTP.
//
// It is the desktop client with the window taken off: the same session rules,
// the same runtime, the same bridge connection, and a small control surface so
// an acceptance script can play a match between two of them and read what each
// one believes. It exists because the thing worth testing is two independent
// processes with independent wallets, and a window cannot be scripted.
package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/karamble/dcr4inarow/internal/bridgeconn"
	"github.com/karamble/dcr4inarow/internal/session"
	"github.com/karamble/dcrgaming-sdk/pkg/identity"
	sdk "github.com/karamble/dcrgaming-sdk/pkg/runtime"
)

type player struct {
	game *session.Game
	rt   *sdk.Runtime

	mu        sync.Mutex
	connected bool
	sid       string
	height    int64
	lastErr   string
}

type status struct {
	Connected bool   `json:"connected"`
	Match     string `json:"match"`
	Seat      uint32 `json:"seat"`
	Turn      uint32 `json:"turn"`
	Board     int    `json:"board"`
	Score     []int  `json:"score"`
	Moves     int    `json:"moves"`
	Head      string `json:"head"`
	Done      bool   `json:"done"`
	Won       bool   `json:"won"`
	Winner    uint32 `json:"winner"`
	Abandoned bool   `json:"abandoned"`
	Ready     bool   `json:"ready"`
	Agreed    bool   `json:"agreed"`
	Playable  bool   `json:"playable"`
	Paid      bool   `json:"paid"`
	Blocked   string `json:"blocked"`
	Why       string `json:"why"`
	CanFund   bool   `json:"canFund"`
	Funded    bool   `json:"funded"`
	Height    int64  `json:"height"`
	Error     string `json:"error"`
}

func main() {
	appdata := flag.String("appdata", "", "profile directory holding bridge.json")
	listen := flag.String("listen", "127.0.0.1:19801", "control address")
	flag.Parse()
	if *appdata == "" {
		die("-appdata is required")
	}

	cfg, err := bridgeconn.Load(filepath.Join(*appdata, "bridge.json"))
	if err != nil {
		die(err.Error())
	}
	p := &player{}
	if err := p.start(*appdata, cfg); err != nil {
		die(err.Error())
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/status", p.status)
	mux.HandleFunc("/fund", p.fund)
	mux.HandleFunc("/move", p.move)
	mux.HandleFunc("/settle", p.settle)
	mux.HandleFunc("/abandon", p.abandon)
	srv := &http.Server{Addr: *listen, Handler: mux, ReadHeaderTimeout: 5 * time.Second}
	if err := srv.ListenAndServe(); err != nil {
		die(err.Error())
	}
}

func die(msg string) {
	fmt.Fprintln(os.Stderr, msg)
	os.Exit(1)
}

func (p *player) start(dir string, cfg bridgeconn.Config) error {
	game, err := session.New(filepath.Join(dir, "books"))
	if err != nil {
		return err
	}
	ctx := context.Background()
	bridge, err := bridgeconn.Connect(ctx, cfg, game.Identity())
	if err != nil {
		return err
	}
	seed, err := identity.Load(dir)
	if err != nil {
		return err
	}
	// The harness mines on demand, so follow the tip closely.
	rt, err := sdk.Open(sdk.Config{
		Rules: game, Bridge: bridge, Identity: seed, Dir: dir,
		SeatTags: session.SeatTags, TickEvery: time.Second,
	})
	if err != nil {
		return err
	}
	game.Bind(rt)
	p.game, p.rt = game, rt

	go func() { _ = rt.Run(ctx) }()
	go p.follow(ctx)

	p.mu.Lock()
	p.connected = true
	p.mu.Unlock()
	return nil
}

// follow keeps the runtime's view of the chain current and notices a table the
// moment one is seated.
func (p *player) follow(ctx context.Context) {
	t := time.NewTicker(time.Second)
	defer t.Stop()
	for range t.C {
		if tip, err := p.rt.Chain(ctx); err == nil {
			p.mu.Lock()
			p.height = tip.Height
			p.mu.Unlock()
		}
		for _, sid := range p.game.Tables() {
			p.mu.Lock()
			p.sid = sid
			p.mu.Unlock()
			// Every table, every tick: the deposit checks, the rules
			// handshake and the settlement watch all happen here.
			if err := p.game.Prepare(ctx, sid); err != nil {
				p.mu.Lock()
				p.lastErr = err.Error()
				p.mu.Unlock()
			} else if err := p.game.Start(ctx, sid); err != nil {
				p.mu.Lock()
				p.lastErr = err.Error()
				p.mu.Unlock()
			}
			break
		}
	}
}

func (p *player) table() (string, bool) {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.sid, p.sid != ""
}

func (p *player) status(w http.ResponseWriter, _ *http.Request) {
	p.mu.Lock()
	out := status{Connected: p.connected, Match: p.sid, Height: p.height, Error: p.lastErr}
	p.mu.Unlock()

	if out.Match != "" {
		if v, ok := p.game.View(out.Match); ok {
			out.Seat, out.Turn, out.Board = v.Seat, v.Turn, v.Board
			out.Score = []int{v.Score[0], v.Score[1]}
			out.Moves, out.Head = v.Moves, v.Head
			out.Done, out.Won, out.Winner = v.Done, v.Won, v.Winner
			out.Abandoned = v.Abandoned
			out.Ready = p.game.Ready(out.Match)
			out.Agreed = p.game.StartAgreed(out.Match)
			out.Playable, out.Why = p.game.Playable(out.Match)
			out.Paid = p.game.Paid(out.Match)
			out.Blocked = v.Blocked
			_, _, funded := p.rt.Funded(out.Match, v.Seat)
			out.Funded = funded
			out.CanFund, _ = p.game.CanFund(out.Match)
		}
	}
	writeJSON(w, out)
}

// fund starts the payment and answers. Fund blocks until a person approves in
// dcrpulse, and the approval is the next thing the harness does, so answering
// only after it returned would deadlock the two against each other.
func (p *player) fund(w http.ResponseWriter, r *http.Request) {
	p.queue(w, func(sid string) error { return p.game.Fund(context.Background(), sid) })
}

func (p *player) settle(w http.ResponseWriter, r *http.Request) {
	p.act(w, func(sid string) error { return p.game.Settle(r.Context(), sid) })
}

func (p *player) abandon(w http.ResponseWriter, r *http.Request) {
	p.act(w, func(sid string) error { return p.game.Abandon(r.Context(), sid) })
}

func (p *player) move(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Column uint8 `json:"column"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	p.act(w, func(sid string) error { return p.game.Play(r.Context(), sid, body.Column) })
}

// queue runs an action that waits on a person, and answers straight away. Its
// outcome shows up in the status report rather than in this response.
func (p *player) queue(w http.ResponseWriter, do func(string) error) {
	sid, ok := p.table()
	if !ok {
		http.Error(w, "no table", http.StatusConflict)
		return
	}
	go func() {
		err := do(sid)
		p.mu.Lock()
		if err != nil {
			p.lastErr = err.Error()
		} else {
			p.lastErr = ""
		}
		p.mu.Unlock()
	}()
	writeJSON(w, map[string]bool{"queued": true})
}

func (p *player) act(w http.ResponseWriter, do func(string) error) {
	sid, ok := p.table()
	if !ok {
		http.Error(w, "no table", http.StatusConflict)
		return
	}
	if err := do(sid); err != nil {
		p.mu.Lock()
		p.lastErr = err.Error()
		p.mu.Unlock()
		http.Error(w, err.Error(), http.StatusConflict)
		return
	}
	p.mu.Lock()
	p.lastErr = ""
	p.mu.Unlock()
	writeJSON(w, map[string]bool{"ok": true})
}

func writeJSON(w http.ResponseWriter, v any) {
	w.Header().Set("content-type", "application/json")
	_ = json.NewEncoder(w).Encode(v)
}
