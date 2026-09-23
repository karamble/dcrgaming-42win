package render

import (
	"image/color"
	"strings"
	"testing"
	"time"

	"github.com/karamble/dcrgaming-42win/internal/match"
	"github.com/karamble/dcrgaming-42win/internal/movelog"
)

func sequence(t *testing.T, cols []uint8) View {
	t.Helper()
	m, _ := match.New(0)
	v := View{MatchID: "test", Seat: 0, Hover: -1}
	for i, col := range cols {
		e := movelog.Entry{Column: col, Seat: m.Turn(), Board: uint8(m.Index()), Seq: uint64(i)}
		v.Entries = append(v.Entries, e)
		if err := m.Play(match.Move{Column: col, Seat: e.Seat, Board: e.Board}); err != nil {
			t.Fatal(err)
		}
	}
	v.Grid, v.Board, v.Turn, v.Score, v.Results = *m.Board(), m.Index(), m.Turn(), m.Score(), m.Results()
	v.Winner, v.Won, v.Done = m.Outcome()
	return v
}

func TestRoundWinningMoveIsPresentedEvenWhenNextMoveArrived(t *testing.T) {
	cols := []uint8{0, 1, 0, 1, 0, 1, 0, 3}
	now := time.Unix(0, 0)
	p := Presenter{}
	p.Update(sequence(t, cols[:6]), now, false)
	v, cues := p.Update(sequence(t, cols), now, false)
	if v.Board != 0 || v.Grid.Moves() != 7 || v.Drop == nil || v.RoundMessage == "" || len(cues) != 1 {
		t.Fatalf("winning board was skipped: %+v", v)
	}
	v, cues = p.Update(sequence(t, cols), now.Add(time.Second), false)
	if v.Board != 0 || v.Drop != nil || v.RoundMessage == "" || len(cues) != 0 {
		t.Fatal("result not held or sound repeated")
	}
	v, _ = p.Update(sequence(t, cols), now.Add(2*time.Second), false)
	if v.Board != 1 || v.Grid.Moves() != 1 {
		t.Fatal("queued next move lost")
	}
	v, _ = p.Update(sequence(t, cols), now.Add(3*time.Second), false)
	if p.Busy() || v.Board != 1 || v.RoundMessage != "" {
		t.Fatal("presentation did not catch up")
	}
}

func TestReconnectIsSilentAndReducedMotionHasNoDrop(t *testing.T) {
	now := time.Unix(0, 0)
	p := Presenter{}
	empty := sequence(t, nil)
	one := sequence(t, []uint8{3})
	p.Update(empty, now, false)
	v, _ := p.Update(one, now, true)
	if v.Drop != nil {
		t.Fatal("reduced motion animated")
	}
	one.Stale = true
	p.Update(one, now, false)
	two := sequence(t, []uint8{3, 4})
	v, cues := p.Update(two, now, false)
	if p.Busy() || v.Drop != nil || len(cues) > 0 {
		t.Fatal("reconnect replayed historical effects")
	}
}

func TestDismissRoundDoesNotHighlightAnEmptyCell(t *testing.T) {
	cols := []uint8{0, 1, 0, 1, 0, 1, 0}
	now := time.Unix(0, 0)
	p := Presenter{}
	p.Update(sequence(t, cols[:5]), now, false)
	p.Update(sequence(t, cols[:6]), now, false)
	p.Update(sequence(t, cols[:6]), now.Add(time.Second), false)
	p.Update(sequence(t, cols), now.Add(2*time.Second), false)
	p.Dismiss()
	v, _ := p.Update(sequence(t, cols), now.Add(3*time.Second), false)
	if v.HasLast || v.Board != 1 {
		t.Fatal("dismiss left the previous board highlight")
	}
}

func TestFinancialWordsDoNotPromiseConfirmation(t *testing.T) {
	for _, state := range []string{"spending", "spent", "unavailable", "verified"} {
		text := FinancialStatus(View{Done: true, StakeCheck: state})
		if strings.Contains(text, "confirmed") || strings.Contains(text, "received") {
			t.Fatal(text)
		}
	}
	if !strings.Contains(FinancialStatus(View{StakeCheck: "spent", Stale: true}), "out of date") {
		t.Fatal("stale spend still reported current")
	}
	if !strings.Contains(FinancialStatus(View{StakeCheck: "spending"}), "progress") {
		t.Fatal("spending promoted to spent")
	}
}

func TestAdaptiveGeometryAndLandingAcrossSizes(t *testing.T) {
	defer SetSize(960, 640)
	for _, size := range [][2]int{{960, 640}, {1280, 800}, {1920, 1080}} {
		SetSize(size[0], size[1])
		if PanelX+PanelW > Width-39 || BoardY+BoardH > Height-90 {
			t.Fatal("layout overflow", size)
		}
		for col := 0; col < 7; col++ {
			x, y := centre(col, 0)
			if ColumnAt(x, y) != col {
				t.Fatal("hit target drift", size, col)
			}
		}
		for i := 0; i < 5; i++ {
			x, y, w, h := FieldRect(i)
			if x < panX || y < panY || x+w > panX+panW || y+h > panY+panH {
				t.Fatal("field escaped settings")
			}
		}
	}
}

func TestTextClippingDoesNotPaintOutsideField(t *testing.T) {
	SetSize(960, 640)
	r := NewRaster()
	r.Rect(0, 0, Width, Height, Navy)
	clip(r, 100, 100, 30, 30).Text("A very long credential label", 100, 100, 16, Text)
	for y := 95; y < 135; y++ {
		for x := 95; x < 400; x++ {
			if x >= 100 && x < 130 && y >= 100 && y < 130 {
				continue
			}
			if r.Target.RGBAAt(x, y) != (color.RGBA{Navy.R, Navy.G, Navy.B, 255}) {
				t.Fatal("text escaped clipping", x, y)
			}
		}
	}
}
