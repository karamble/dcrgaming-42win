// Package clipboard reads the system clipboard.
//
// The three bridge credentials are thousands of characters of PEM. Nobody types
// those, so pasting is the only realistic way in - which makes the clipboard
// part of the setup path rather than a convenience.
package clipboard

import (
	"context"
	"errors"
	"os/exec"
	"runtime"
	"strings"
	"time"
)

// Max bounds what will be accepted, so a clipboard holding a file cannot become
// a field holding a file.
const Max = 64 * 1024

// Paste returns the clipboard's text.
func Paste() (string, error) {
	name, args := command()
	if name == "" {
		return "", errors.New("No clipboard tool found. Install wl-clipboard, xclip or xsel.")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	out, err := exec.CommandContext(ctx, name, args...).Output()
	if err != nil {
		return "", errors.New("Could not read the clipboard.")
	}
	if len(out) > Max {
		return "", errors.New("The clipboard holds more than this field accepts.")
	}
	return strings.TrimRight(string(out), "\r\n"), nil
}

func command() (string, []string) {
	switch runtime.GOOS {
	case "darwin":
		return "pbpaste", nil
	case "windows":
		return "powershell.exe", []string{"-NoProfile", "-NonInteractive", "-Command", "Get-Clipboard -Raw"}
	}
	for _, c := range [][]string{
		{"wl-paste", "--no-newline"},
		{"xclip", "-selection", "clipboard", "-o"},
		{"xsel", "--clipboard", "--output"},
	} {
		if _, err := exec.LookPath(c[0]); err == nil {
			return c[0], c[1:]
		}
	}
	return "", nil
}
