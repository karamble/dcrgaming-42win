package logging

import (
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/decred/slog"
)

func TestOneLevelAppliesToEverySubsystem(t *testing.T) {
	levels, err := Levels("warn")
	if err != nil {
		t.Fatalf("levels: %v", err)
	}
	for _, id := range IDs {
		if levels[id] != slog.LevelWarn {
			t.Fatalf("%s is at %v", id, levels[id])
		}
	}
}

func TestSubsystemsTakeTheirOwnLevels(t *testing.T) {
	levels, err := Levels("SESS=debug,SDK=error")
	if err != nil {
		t.Fatalf("levels: %v", err)
	}
	if levels["SESS"] != slog.LevelDebug || levels["SDK"] != slog.LevelError {
		t.Fatalf("named subsystems were not set: %v", levels)
	}
	if levels["BRDG"] != slog.LevelInfo {
		t.Fatalf("an unnamed subsystem did not keep the default: %v", levels["BRDG"])
	}
}

func TestAnUnknownSubsystemOrLevelIsRefused(t *testing.T) {
	for _, spec := range []string{"NOPE=debug", "SESS=loud", "SESS", "=debug"} {
		if _, err := Levels(spec); err == nil {
			t.Fatalf("%q was accepted", spec)
		}
	}
}

func TestSubsystemsAreListedForTheOperator(t *testing.T) {
	list := Subsystems()
	for _, id := range IDs {
		if !strings.Contains(list, id) {
			t.Fatalf("%s is missing from %q", id, list)
		}
	}
}

func TestTheLogIsWrittenIntoTheProfile(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "logs")
	b, err := Open(dir, "info", 1024, 10, io.Discard)
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	b.Logger("FOUR").Infof("a line worth keeping")
	if err := b.Close(); err != nil {
		t.Fatalf("close: %v", err)
	}
	data, err := os.ReadFile(filepath.Join(dir, "dcr4inarow.log"))
	if err != nil {
		t.Fatalf("read the log: %v", err)
	}
	if !strings.Contains(string(data), "a line worth keeping") {
		t.Fatalf("the log does not hold what was written to it:\n%s", data)
	}
}

func TestAnUnknownSubsystemLoggerIsSilentRatherThanNil(t *testing.T) {
	b, err := Open(filepath.Join(t.TempDir(), "logs"), "info", 1024, 10, io.Discard)
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	defer b.Close()
	b.Logger("NOPE").Infof("this must not panic")
}
