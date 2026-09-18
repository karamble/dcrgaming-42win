//go:build desktop && dev

package main

import (
	"github.com/karamble/dcr4inarow/internal/match"
	"github.com/karamble/dcr4inarow/pkg/render"
)

// local is a fixture board driven from one keyboard.
//
// An engineering tool, not a way to play: there is no log, no signature, no
// bridge and no money in it. It exists so the window, the drawing and the input
// can be exercised without a table.
type local struct{ m *match.Match }

func (l *local) View() render.View {
	winner, won, done := l.m.Outcome()
	v := render.View{
		MatchID: "local fixture",
		Grid:    *l.m.Board(),
		Seat:    l.m.Turn(), // whoever is to move is "you" at one keyboard
		Board:   l.m.Index(),
		Turn:    l.m.Turn(),
		Score:   l.m.Score(),
		Done:    done,
		Won:     won,
		Winner:  winner,
		Notice:  "local fixture: no log, no stake, no bridge",
	}
	for _, r := range l.m.Results() {
		v.Moves += r.Moves
	}
	v.Moves += l.m.Board().Moves()
	return v
}

func (l *local) Play(column int) error {
	return l.m.Play(match.Move{Board: uint8(l.m.Index()), Seat: l.m.Turn(), Column: uint8(column)})
}

func openFixture() (source, error) {
	m, err := match.New(0)
	if err != nil {
		return nil, err
	}
	return &local{m: m}, nil
}
