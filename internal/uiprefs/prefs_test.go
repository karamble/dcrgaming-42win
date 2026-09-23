package uiprefs

import (
	"os"
	"path/filepath"
	"testing"
)

func TestPreferencesArePrivateAndSurviveRestart(t *testing.T) {
	dir := t.TempDir()
	p, err := Load(dir)
	if err != nil || p.Muted || p.ReducedMotion {
		t.Fatal("bad defaults")
	}
	want := Preferences{Muted: true, ReducedMotion: true}
	if err = want.Save(dir); err != nil {
		t.Fatal(err)
	}
	p, err = Load(dir)
	if err != nil || p != want {
		t.Fatal("preferences did not persist")
	}
	info, _ := os.Stat(filepath.Join(dir, "ui.json"))
	if info.Mode().Perm() != 0o600 {
		t.Fatal("wrong permissions")
	}
	if err = os.WriteFile(filepath.Join(dir, "ui.json"), []byte("invalid"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err = Load(dir); err == nil {
		t.Fatal("corruption hidden")
	}
}
