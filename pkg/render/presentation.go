package render

import (
	"fmt"
	"time"

	"github.com/karamble/dcrgaming-42win/internal/board"
	"github.com/karamble/dcrgaming-42win/internal/match"
)

type frame struct {
	grid        board.Board
	index       int
	score       [2]int
	turn        uint32
	results     []match.Result
	col, row    int
	seat        uint32
	round, done bool
	winner      uint32
	won         bool
}

// Presenter is presentation-only. A reconnect establishes a baseline; committed
// entries received together are replayed in order without touching the session.
type Presenter struct {
	id               string
	seen             int
	stale            bool
	queue            []frame
	since            time.Time
	lastCol, lastRow int
	hasLast          bool
}

func (p *Presenter) Reset()     { *p = Presenter{} }
func (p *Presenter) Busy() bool { return len(p.queue) > 0 }
func (p *Presenter) Dismiss() {
	if len(p.queue) > 0 && p.queue[0].round {
		f := p.queue[0]
		p.lastCol, p.lastRow, p.hasLast = f.col, f.row, f.done
		p.queue = p.queue[1:]
		p.since = time.Time{}
	}
}

func (p *Presenter) Update(v View, now time.Time, reduced bool) (View, []string) {
	var cues []string
	if p.id != v.MatchID || p.stale || v.Stale || len(v.Entries) < p.seen {
		*p = Presenter{id: v.MatchID, seen: len(v.Entries), stale: v.Stale}
		if len(v.Entries) > 0 {
			e := v.Entries[len(v.Entries)-1]
			if int(e.Board) == v.Board {
				p.lastCol = int(e.Column)
				for row := 0; row < 6; row++ {
					if _, ok := v.Grid.At(p.lastCol, row).Seat(); ok {
						p.lastRow = row
						p.hasLast = true
					}
				}
				v.LastCol, v.LastRow, v.HasLast = p.lastCol, p.lastRow, p.hasLast
			}
		}
		return v, nil
	}
	if len(v.Entries) > p.seen {
		m, _ := match.New(0)
		for i, e := range v.Entries {
			grid := *m.Board()
			row, err := grid.Drop(int(e.Column), e.Seat)
			if err != nil {
				p.Reset()
				return v, nil
			}
			idx := m.Index()
			if err = m.Play(match.Move{Board: e.Board, Seat: e.Seat, Column: e.Column}); err != nil {
				p.Reset()
				return v, nil
			}
			if i < p.seen {
				continue
			}
			winner, won, done := m.Outcome()
			p.queue = append(p.queue, frame{grid: grid, index: idx, score: m.Score(), turn: m.Turn(), results: m.Results(), col: int(e.Column), row: row, seat: e.Seat, round: grid.Over(), done: done, winner: winner, won: won})
		}
		p.seen = len(v.Entries)
	}
	if len(p.queue) == 0 {
		v.HasLast = p.hasLast
		v.LastCol, v.LastRow = p.lastCol, p.lastRow
		return v, nil
	}
	f := p.queue[0]
	if p.since.IsZero() {
		p.since = now
		if f.round {
			cues = append(cues, "result")
		} else if f.turn == v.Seat {
			cues = append(cues, "turn")
		} else {
			cues = append(cues, "drop")
		}
	}
	dropTime := 220 * time.Millisecond
	if reduced {
		dropTime = 0
	}
	duration := dropTime
	if f.round {
		duration += 1500 * time.Millisecond
	}
	elapsed := now.Sub(p.since)
	if elapsed >= duration {
		p.queue = p.queue[1:]
		p.since = time.Time{}
		p.lastCol, p.lastRow, p.hasLast = f.col, f.row, !f.round || f.done
		if len(p.queue) == 0 {
			v.LastCol, v.LastRow, v.HasLast = p.lastCol, p.lastRow, p.hasLast
			return v, cues
		}
		out, next := p.Update(v, now, reduced)
		return out, append(cues, next...)
	}
	v.Grid, v.Board, v.Score, v.Turn, v.Results = f.grid, f.index, f.score, f.turn, f.results
	v.Done, v.Winner, v.Won = f.done, f.winner, f.won
	v.LastCol, v.LastRow, v.HasLast = f.col, f.row, true
	if elapsed < dropTime {
		v.Drop = &Drop{Col: f.col, Row: f.row, Seat: f.seat, Progress: float64(elapsed) / float64(dropTime)}
	}
	if f.round && !f.done {
		winner, won := f.grid.Winner()
		v.RoundMessage = "Round drawn"
		if won {
			name := "Opponent"
			if winner == v.Seat {
				name = "You"
			}
			v.RoundMessage = fmt.Sprintf("%s won round %d", name, f.index+1)
		}
	}
	return v, cues
}
