// Command dcr4inarow-preview renders a table screen to PNG without a display.
//
// It exists so the screen can be looked at, and diffed, without opening a
// window. No bridge, no wallet and no network: the position is fabricated.
package main

import (
	"flag"
	"fmt"
	"image/png"
	"os"
	"strings"

	"github.com/karamble/dcrgaming-42win/assets/art"
	"github.com/karamble/dcrgaming-42win/internal/board"
	"github.com/karamble/dcrgaming-42win/internal/match"
	"github.com/karamble/dcrgaming-42win/internal/tablelobby"
	"github.com/karamble/dcrgaming-42win/pkg/render"
)

func main() {
	out := flag.String("output", "artifacts/dcr4inarow-table.png", "PNG output path")
	stage := flag.String("stage", "midgame",
		"which screen: lobby, lobby-connected, settings, opening, midgame, won, "+
			"cover[-loading|-error], help, receipt, pending, disconnected, round-win, lost, void, spending, spent, blocked, "+
			"or table-invitation|admission|roster|draw|stake|ready|closed|stale|details")
	hover := flag.Int("hover", 4, "column under the pointer, -1 for none")
	width := flag.Int("width", 960, "logical window width")
	height := flag.Int("height", 640, "logical window height")
	flag.Parse()
	render.SetSize(*width, *height)

	raster := render.NewRaster()
	switch {
	case strings.HasPrefix(*stage, "cover"):
		v := render.Cover{Ready: *stage != "cover-loading", Frame: 18}
		if *stage == "cover-error" {
			v.Error = "Artwork unavailable"
		}
		render.DrawCover(raster, v)
	case *stage == "help":
		render.DrawHelp(raster, "simnet")
	case *stage == "receipt":
		v, _ := scene("won", -1)
		v.StakeCheck = "spending"
		render.DrawReceipt(raster, v, "Export includes transcript and reference roster.")
	case len(*stage) > 6 && (*stage)[:6] == "table-":
		drawSeating(raster, (*stage)[6:])
	case *stage == "lobby" || *stage == "lobby-connected" || *stage == "settings":
		drawLobbyish(raster, *stage)
	default:
		base := *stage
		switch base {
		case "pending", "disconnected", "round-win", "void":
			base = "midgame"
		case "lost", "spending", "spent", "blocked":
			base = "won"
		}
		view, err := scene(base, *hover)
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
		switch *stage {
		case "pending":
			view.Pending = true
		case "disconnected":
			view.Stale = true
			view.Connected = false
			view.Notice = "Cached state · reconnect from Settings to recheck the bridge"
		case "round-win":
			view.RoundMessage = "You won round 1"
			grid := board.New()
			for i, col := range openerWinsIn7 {
				_, _ = grid.Drop(int(col), uint32(i%2))
			}
			view.Grid = *grid
			view.Board = 0
			view.Turn = 1
		case "lost":
			view.Seat = 1
		case "void":
			view.Done = true
			view.Won = false
			view.Abandoned = true
		case "spending", "spent":
			view.StakeCheck = *stage
		case "blocked":
			view.Blocked = "Approval is required in dcrpulse"
		}
		render.DrawTable(raster, view)
	}

	f, err := os.Create(*out)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	defer f.Close()
	if err := png.Encode(f, raster.Target); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	fmt.Println(*out)
}

// openerWinsIn7 is a vertical win for whoever opens a board.
var openerWinsIn7 = []uint8{0, 1, 0, 1, 0, 1, 0}

func scene(stage string, hover int) (render.View, error) {
	m, err := match.New(0)
	if err != nil {
		return render.View{}, err
	}
	play := func(cols []uint8) error {
		for _, col := range cols {
			mv := match.Move{Board: uint8(m.Index()), Seat: m.Turn(), Column: col}
			if err := m.Play(mv); err != nil {
				return err
			}
		}
		return nil
	}

	switch stage {
	case "opening":
	case "midgame":
		if err := play(openerWinsIn7); err != nil {
			return render.View{}, err
		}
		if err := play([]uint8{3, 2, 3, 4, 2, 5}); err != nil {
			return render.View{}, err
		}
	case "won":
		for i := 0; i < 3; i++ {
			if err := play(openerWinsIn7); err != nil {
				return render.View{}, err
			}
		}
	default:
		return render.View{}, fmt.Errorf("unknown stage %q", stage)
	}

	winner, won, done := m.Outcome()
	view := render.View{
		StakeCheck: "verified",
		Results:    m.Results(), Network: "SIMNET", Bond: 1000000,
		MatchID:   "641be69ec6100361ff3e35357355c9d1",
		Grid:      *m.Board(),
		Seat:      0,
		Board:     m.Index(),
		Turn:      m.Turn(),
		Score:     m.Score(),
		Moves:     0,
		Done:      done,
		Won:       won,
		Winner:    winner,
		Connected: true,
		Hover:     hover,
		Stake:     100_000,
	}
	for _, r := range m.Results() {
		view.Moves += r.Moves
	}
	view.Moves += m.Board().Moves()
	if !done {
		view.Notice = "payout needs both seats to sign. if it fails you reclaim your own stake after its lock, less fees."
	}
	return view, nil
}

// drawLobbyish renders the screens that are not a board.
func drawLobbyish(c render.Canvas, stage string) {
	lobby := render.Lobby{
		Backdrop: art.LobbyBackdrop(),
		Network:  "mainnet",
		Build:    "DEVELOPMENT BUILD",
		Status:   "No bridge credentials yet. Open settings to paste them.",
	}
	if stage != "lobby" {
		lobby.Connected = true
		lobby.Configured = true
		lobby.Status = ""
	}
	render.DrawLobby(c, lobby)
	if stage != "settings" {
		return
	}
	render.DrawSettings(c, render.Settings{
		Network: "mainnet",
		Fields: []render.Field{
			{Label: "Bridge address", Value: "127.0.0.1", Placeholder: "127.0.0.1"},
			{Label: "Gaming port", Value: "8443", Placeholder: "8443"},
			{Label: "Client certificate", Value: certSample, Placeholder: "Click here and paste the complete PEM text"},
			{Label: "Client private key", Value: certSample, Masked: true, Placeholder: "Click here and paste the complete PEM text"},
			{Label: "Bridge certificate", Placeholder: "Click here and paste the complete PEM text", Focused: true},
		},
		Status: "Paste the bridge certificate to finish.",
	})
}

const certSample = "-----BEGIN CERTIFICATE-----\nMIIBkTCB+wIJAJ2m0p0Y3JmpMA0GCSqGSIb3DQEBCwUAMBQxEjAQBgNVBAMMCWRj\ncjRpbmFyb3cwHhcNMjYwOTE4\n-----END CERTIFICATE-----\n"

// drawSeating renders one stage of the table seating screen from a fixture.
func drawSeating(c render.Canvas, name string) {
	details := name == "details"
	if details {
		name = "stake"
	}
	stages := map[string]tablelobby.Stage{
		"invitation": tablelobby.StageInvitation,
		"admission":  tablelobby.StageAdmission,
		"roster":     tablelobby.StageRoster,
		"draw":       tablelobby.StageSeatDraw,
		"stake":      tablelobby.StageStake,
		"ready":      tablelobby.StageReady,
		"closed":     tablelobby.StageClosed,
	}
	stale := name == "stale"
	if stale {
		name = "stake"
	}
	stage, ok := stages[name]
	if !ok {
		fmt.Fprintf(os.Stderr, "unknown table stage %q\n", name)
		os.Exit(1)
	}
	v := tablelobby.Fixture(stage)
	v.Details = details
	v.Network = "SIMNET"
	if stale {
		v.Stale = true
		v.NextStep, v.NextDetail = tablelobby.Guidance(v)
	}
	if stale {
		v.CanFund = false
		v.NextStep, v.NextDetail = tablelobby.Guidance(v)
	}
	render.DrawTableLobby(c, v)
}
