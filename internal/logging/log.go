// Package logging supplies Decred slog subsystems with bounded log rotation.
//
// The same arrangement dcrd and dcrwallet use: named subsystems, a level per
// subsystem, output to both stdout and a rotating file under the profile's own
// log directory. Two seats running on one machine keep separate logs because
// they keep separate profiles.
package logging

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"

	"github.com/decred/slog"
	"github.com/jrick/logrotate/rotator"
)

// IDs are the subsystems. FOUR is the client, SESS the game's own rules and
// move log, SDK the gaming runtime, BRDG the bridge connection.
var IDs = []string{"FOUR", "SESS", "SDK", "BRDG"}

// Levels reads a debug level specification: one level for everything, or a
// comma-separated list of SUBSYSTEM=level.
func Levels(spec string) (map[string]slog.Level, error) {
	levels := make(map[string]slog.Level, len(IDs))
	for _, id := range IDs {
		levels[id] = slog.LevelInfo
	}
	if !strings.ContainsAny(spec, ",=") {
		l, ok := slog.LevelFromString(spec)
		if !ok {
			return nil, fmt.Errorf("invalid log level %q", spec)
		}
		for id := range levels {
			levels[id] = l
		}
		return levels, nil
	}
	for _, part := range strings.Split(spec, ",") {
		pair := strings.Split(part, "=")
		if len(pair) != 2 {
			return nil, fmt.Errorf("invalid subsystem level %q", part)
		}
		id := strings.TrimSpace(pair[0])
		if _, ok := levels[id]; !ok {
			return nil, fmt.Errorf("unknown log subsystem %q", id)
		}
		level, ok := slog.LevelFromString(strings.TrimSpace(pair[1]))
		if !ok {
			return nil, fmt.Errorf("invalid log level %q", pair[1])
		}
		levels[id] = level
	}
	return levels, nil
}

// Subsystems lists what may be named in a level specification.
func Subsystems() string {
	ids := append([]string(nil), IDs...)
	sort.Strings(ids)
	return strings.Join(ids, " ")
}

// Backend writes to stdout and a rotating file at once.
type Backend struct {
	mu       sync.Mutex
	rotation *rotator.Rotator
	stdout   io.Writer
	closed   bool
	loggers  map[string]slog.Logger
	Path     string
}

// Open prepares the log directory and the subsystem loggers.
func Open(dir, spec string, sizeKB int64, maxFiles int, stdout io.Writer) (*Backend, error) {
	levels, err := Levels(spec)
	if err != nil {
		return nil, err
	}
	if sizeKB < 1 || maxFiles < 1 {
		return nil, fmt.Errorf("invalid log rotation limits")
	}
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return nil, err
	}
	path := filepath.Join(dir, "dcr4inarow.log")
	f, err := os.OpenFile(path, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o600)
	if err != nil {
		return nil, err
	}
	if err := f.Close(); err != nil {
		return nil, err
	}
	r, err := rotator.New(path, sizeKB, false, maxFiles)
	if err != nil {
		return nil, err
	}
	// Synchronous uncompressed rotation keeps the retention bound even under
	// rapid writes; asynchronous compression can leave old archives behind.
	r.SetCompressor(nil, "")

	b := &Backend{rotation: r, stdout: stdout, Path: path, loggers: map[string]slog.Logger{}}
	backend := slog.NewBackend(b)
	for id, level := range levels {
		l := backend.Logger(id)
		l.SetLevel(level)
		b.loggers[id] = l
	}
	return b, nil
}

func (b *Backend) Write(p []byte) (int, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	if b.closed {
		return len(p), nil
	}
	if b.stdout != nil {
		_, _ = b.stdout.Write(p)
	}
	return b.rotation.Write(p)
}

// Logger is a subsystem's logger, or a disabled one for a name that has none.
func (b *Backend) Logger(id string) slog.Logger {
	if l := b.loggers[id]; l != nil {
		return l
	}
	return slog.Disabled
}

func (b *Backend) Close() error {
	b.mu.Lock()
	defer b.mu.Unlock()
	if b.closed {
		return nil
	}
	b.closed = true
	return b.rotation.Close()
}
