package appconfig

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestAFreshProfileWritesItsConfiguration(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "profile")
	cfg, err := Load([]string{"--appdata", dir})
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if !cfg.Created {
		t.Fatal("a profile with no configuration did not report writing one")
	}
	data, err := os.ReadFile(filepath.Join(dir, ConfigName))
	if err != nil {
		t.Fatalf("read the configuration: %v", err)
	}
	if !strings.Contains(string(data), "debuglevel") {
		t.Fatalf("the written configuration does not mention its own options:\n%s", data)
	}
	info, err := os.Stat(filepath.Join(dir, ConfigName))
	if err != nil {
		t.Fatalf("stat: %v", err)
	}
	if mode := info.Mode().Perm(); mode != 0o600 {
		t.Fatalf("the configuration is mode %o, want 0600", mode)
	}
	// Loading again must not rewrite it.
	again, err := Load([]string{"--appdata", dir})
	if err != nil {
		t.Fatalf("second load: %v", err)
	}
	if again.Created {
		t.Fatal("an existing configuration was reported as newly written")
	}
}

func TestDerivedPathsLiveInTheProfile(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "profile")
	cfg, err := Load([]string{"--appdata", dir})
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if cfg.LogDir != filepath.Join(dir, "logs") {
		t.Fatalf("logs go to %q", cfg.LogDir)
	}
	if cfg.BridgeConfig != filepath.Join(dir, "bridge.json") {
		t.Fatalf("bridge credentials go to %q", cfg.BridgeConfig)
	}
	if !cfg.CustomDir {
		t.Fatal("a profile named on the command line was not reported as custom")
	}
}

// Two seats on one machine is two profiles, and the alias exists because
// dcrstakewars and dcrwallet spell it differently.
func TestAppdataAndDatadirAreTheSameDirectory(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "profile")
	byAlias, err := Load([]string{"--datadir", dir})
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if byAlias.AppData != byAlias.DataDir {
		t.Fatalf("the two spellings resolved differently: %q and %q", byAlias.AppData, byAlias.DataDir)
	}
	if _, err := Load([]string{"--appdata", dir, "--datadir", dir + "-other"}); err == nil {
		t.Fatal("two different profile directories were accepted at once")
	}
}

// The file supplies defaults; the command line wins.
func TestTheCommandLineOverridesTheFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, ConfigName)
	if err := os.WriteFile(path, []byte("debuglevel=warn\nlogsize=64\n"), 0o600); err != nil {
		t.Fatalf("write config: %v", err)
	}
	fromFile, err := Load([]string{"--appdata", dir})
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if fromFile.DebugLevel != "warn" || fromFile.LogSize != 64 {
		t.Fatalf("the file was not read: %+v", fromFile.Options)
	}
	overridden, err := Load([]string{"--appdata", dir, "--debuglevel", "debug"})
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if overridden.DebugLevel != "debug" {
		t.Fatalf("the command line did not override the file: %q", overridden.DebugLevel)
	}
	if overridden.LogSize != 64 {
		t.Fatalf("overriding one option discarded another: %d", overridden.LogSize)
	}
}

// Go-style single-dash long flags work too, because that is what the earlier
// builds of this client accepted and what dcrstakewars documents.
func TestSingleDashLongFlagsStillWork(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "profile")
	cfg, err := Load([]string{"-datadir", dir, "-debuglevel", "warn"})
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if cfg.AppData != cfg.DataDir || cfg.DebugLevel != "warn" {
		t.Fatalf("single-dash flags were not honoured: %+v", cfg.Options)
	}
}

func TestOutOfRangeRotationLimitsAreRefused(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "profile")
	for _, args := range [][]string{
		{"--appdata", dir, "--maxlogfiles", "0"},
		{"--appdata", dir, "--logsize", "0"},
	} {
		if _, err := Load(args); err == nil {
			t.Fatalf("%v was accepted", args)
		}
	}
}

// Listing subsystems must not create a profile as a side effect.
func TestShowingSubsystemsTouchesNothing(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "profile")
	if _, err := Load([]string{"--appdata", dir, "--debuglevel", "show"}); err != nil {
		t.Fatalf("load: %v", err)
	}
	if _, err := os.Stat(dir); !os.IsNotExist(err) {
		t.Fatal("listing subsystems created a profile directory")
	}
}
