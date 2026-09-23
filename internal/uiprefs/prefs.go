// Package uiprefs stores presentation preferences separately from credentials.
package uiprefs

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
)

type Preferences struct {
	Muted         bool `json:"muted"`
	ReducedMotion bool `json:"reduced_motion"`
}

func Load(dir string) (Preferences, error) {
	var p Preferences
	b, err := os.ReadFile(filepath.Join(dir, "ui.json"))
	if errors.Is(err, os.ErrNotExist) {
		return p, nil
	}
	if err != nil {
		return p, err
	}
	err = json.Unmarshal(b, &p)
	return p, err
}

func (p Preferences) Save(dir string) error {
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return err
	}
	f, err := os.CreateTemp(dir, ".ui-")
	if err != nil {
		return err
	}
	defer os.Remove(f.Name())
	if err = json.NewEncoder(f).Encode(p); err != nil {
		f.Close()
		return err
	}
	if err = f.Sync(); err != nil {
		f.Close()
		return err
	}
	if err = f.Close(); err != nil {
		return err
	}
	return os.Rename(f.Name(), filepath.Join(dir, "ui.json"))
}
