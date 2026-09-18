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

	"github.com/karamble/dcr4inarow/assets/art"
	"github.com/karamble/dcr4inarow/internal/match"
	"github.com/karamble/dcr4inarow/internal/tablelobby"
	"github.com/karamble/dcr4inarow/pkg/render"
)

func main() {
	out := flag.String("output", "artifacts/dcr4inarow-table.png", "PNG output path")
	stage := flag.String("stage", "midgame",
		"which screen: lobby, lobby-connected, settings, opening, midgame, won, "+
			"or table-invitation|admission|roster|draw|stake|ready|closed|stale")
	hover := flag.Int("hover", 4, "column under the pointer, -1 for none")
	flag.Parse()

	raster := render.NewRaster()
	switch {
	case len(*stage) > 6 && (*stage)[:6] == "table-":
		drawSeating(raster, (*stage)[6:])
	case *stage == "lobby" || *stage == "lobby-connected" || *stage == "settings":
		drawLobbyish(raster, *stage)
	default:
		view, err := scene(*stage, *hover)
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
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
		Stake:     5_000_000,
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
