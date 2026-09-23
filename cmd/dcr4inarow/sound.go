//go:build desktop

package main

import (
	"bytes"
	"encoding/binary"
	"github.com/ebitengine/oto/v3"
	"math"
)

type soundBank struct {
	ctx               *oto.Context
	players           []*oto.Player
	ready             chan audioInit
	started, disabled bool
}

type audioInit struct {
	ctx *oto.Context
	err error
}

// Use Ebitengine's Oto backend directly: audio.Context registers an engine
// hook that terminates RunGame on a device error. Here errors disable only sound.
func (s *soundBank) prepare() {
	if s.started || s.disabled {
		return
	}
	s.started = true
	s.ready = make(chan audioInit, 1)
	go func() {
		ctx, ready, err := oto.NewContext(&oto.NewContextOptions{SampleRate: 44100, ChannelCount: 2, Format: oto.FormatSignedInt16LE, ApplicationName: "dcr4inarow"})
		if err == nil {
			<-ready
			err = ctx.Err()
		}
		s.ready <- audioInit{ctx: ctx, err: err}
	}()
}

func (s *soundBank) poll() {
	if s.ready != nil {
		select {
		case r := <-s.ready:
			s.ready = nil
			s.ctx = r.ctx
			s.disabled = r.err != nil
		default:
		}
	}
	if !s.disabled && s.ctx != nil && s.ctx.Err() != nil {
		s.disabled = true
	}
}

func (s *soundBank) play(cue string) {
	s.prepare()
	s.poll()
	if s.disabled || s.ctx == nil {
		return
	}
	var active []*oto.Player
	for _, p := range s.players {
		if p.IsPlaying() {
			active = append(active, p)
		} else {
			p.Close()
		}
	}
	s.players = active
	freq, duration := 180.0, .10
	if cue == "turn" {
		freq, duration = 660, .14
	}
	if cue == "result" {
		freq, duration = 880, .32
	}
	n := int(44100 * duration)
	data := make([]byte, n*4)
	for i := 0; i < n; i++ {
		t := float64(i) / 44100
		env := math.Exp(-t*16) * math.Min(t*400, 1)
		value := int16(math.Sin(2*math.Pi*freq*t) * env * 1800)
		binary.LittleEndian.PutUint16(data[i*4:], uint16(value))
		binary.LittleEndian.PutUint16(data[i*4+2:], uint16(value))
	}
	p := s.ctx.NewPlayer(bytes.NewReader(data))
	p.Play()
	s.players = append(s.players, p)
}
