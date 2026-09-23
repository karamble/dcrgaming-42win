// Package appconfig follows Decred's conventions: an application directory
// from dcrutil, an INI file created on first run, and command-line options that
// override it.
//
// The same shape as dcrd, dcrwallet and dcrstakewars, because somebody running
// this alongside them should not have to learn a second set of habits - and
// because each --appdata directory is a separate identity, which is how two
// seats are run on one machine.
package appconfig

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/decred/dcrd/dcrutil/v4"
	flags "github.com/jessevdk/go-flags"
)

const (
	AppName    = "dcr4inarow"
	ConfigName = AppName + ".conf"
)

// Options is every setting, from the command line or the INI file.
type Options struct {
	SkipCover    bool   `long:"skip-cover" ini-name:"skip-cover" description:"Skip the illustrated startup cover"`
	AppData      string `short:"A" long:"appdata" ini-name:"appdata" description:"Application directory for configuration, identity, saved tables and logs"`
	DataDir      string `long:"datadir" ini-name:"datadir" description:"Alias for --appdata; use a different directory for each identity"`
	ConfigFile   string `short:"C" long:"configfile" ini-name:"configfile" description:"Configuration file (default: APPDATA/dcr4inarow.conf)"`
	LogDir       string `long:"logdir" ini-name:"logdir" description:"Log directory (default: APPDATA/logs)"`
	DebugLevel   string `short:"d" long:"debuglevel" ini-name:"debuglevel" description:"trace, debug, info, warn, error, critical; or FOUR=debug,SESS=info,SDK=warn,BRDG=info; show lists subsystems"`
	MaxLogFiles  int    `long:"maxlogfiles" ini-name:"maxlogfiles" description:"Maximum number of rotated logs retained"`
	LogSize      int64  `long:"logsize" ini-name:"logsize" description:"Log rotation size in kilobytes"`
	BridgeConfig string `long:"bridge-config" ini-name:"bridge-config" description:"Bridge credential file (default: APPDATA/bridge.json)"`
	Connect      bool   `long:"connect" ini-name:"connect" description:"Connect to the saved bridge at startup"`
	Settings     bool   `long:"settings" ini-name:"settings" description:"Open bridge settings at startup"`
	DevBoard     bool   `long:"dev-board" ini-name:"dev-board" description:"Open the local fixture board (dev build only)"`
}

// Config is the resolved settings plus what happened while resolving them.
type Config struct {
	Options
	// Created says the INI file did not exist and was written.
	Created bool
	// CustomDir says the profile directory came from the command line, which
	// is what running two seats on one machine looks like.
	CustomDir bool
}

func defaults() Options {
	return Options{DebugLevel: "info", MaxLogFiles: 10, LogSize: 1024}
}

// DefaultDir is the application directory Decred's own tools would choose.
func DefaultDir() string { return dcrutil.AppDataDir(AppName, false) }

func expand(p string) (string, error) {
	p = os.ExpandEnv(p)
	if p == "~" || strings.HasPrefix(p, "~/") || strings.HasPrefix(p, "~\\") {
		home, err := os.UserHomeDir()
		if err != nil {
			return "", err
		}
		p = filepath.Join(home, strings.TrimLeft(strings.TrimPrefix(p, "~"), "/\\"))
	}
	if p == "" {
		return "", errors.New("empty path")
	}
	return filepath.Abs(filepath.Clean(p))
}

func parser(o *Options) *flags.Parser { return flags.NewParser(o, flags.HelpFlag|flags.PassDoubleDash) }

// arguments accepts Go-style -long flags as well as Decred-style --long, so a
// habit from either side of the fence works.
func arguments(args []string, p *flags.Parser) []string {
	out := append([]string(nil), args...)
	for i, a := range out {
		if a == "--" {
			break
		}
		if strings.HasPrefix(a, "-") && !strings.HasPrefix(a, "--") {
			name := strings.SplitN(a[1:], "=", 2)[0]
			if len(name) > 1 && p.FindOptionByLongName(name) != nil {
				out[i] = "-" + a
			}
		}
	}
	return out
}

// home resolves the profile directory from the two spellings of it.
func home(o Options) (string, error) {
	if o.AppData != "" && o.DataDir != "" {
		a, err := expand(o.AppData)
		if err != nil {
			return "", err
		}
		b, err := expand(o.DataDir)
		if err != nil {
			return "", err
		}
		if a != b {
			return "", errors.New("--appdata and --datadir name different directories")
		}
		return a, nil
	}
	if o.DataDir != "" {
		return expand(o.DataDir)
	}
	if o.AppData != "" {
		return expand(o.AppData)
	}
	return expand(DefaultDir())
}

// Load resolves the configuration: command line, then INI file, then defaults.
func Load(args []string) (*Config, error) {
	pre := defaults()
	p := parser(&pre)
	args = arguments(args, p)
	rest, err := p.ParseArgs(args)
	if err != nil {
		return nil, err
	}
	if len(rest) != 0 {
		return nil, fmt.Errorf("unexpected positional arguments")
	}
	for _, name := range []string{"appdata", "datadir"} {
		o := p.FindOptionByLongName(name)
		if o.IsSet() && ((name == "appdata" && pre.AppData == "") || (name == "datadir" && pre.DataDir == "")) {
			return nil, fmt.Errorf("%s must not be empty", name)
		}
	}
	root, err := home(pre)
	if err != nil {
		return nil, err
	}
	path := pre.ConfigFile
	if path == "" {
		path = filepath.Join(root, ConfigName)
	}
	if path, err = expand(path); err != nil {
		return nil, err
	}
	// Listing subsystems creates no files and touches no profile.
	if pre.DebugLevel == "show" {
		return &Config{Options: pre}, nil
	}
	created, err := create(path)
	if err != nil {
		return nil, err
	}

	o := defaults()
	p = parser(&o)
	if err := flags.NewIniParser(p).ParseFile(path); err != nil {
		return nil, fmt.Errorf("parse configuration: %w", err)
	}
	// Either spelling on the command line overrides a directory named in the
	// file; otherwise a profile could be redirected by the file it lives in.
	if pre.AppData != "" || pre.DataDir != "" {
		o.AppData, o.DataDir = "", ""
	}
	if _, err := p.ParseArgs(args); err != nil {
		return nil, err
	}
	if root, err = home(o); err != nil {
		return nil, err
	}
	o.AppData, o.DataDir, o.ConfigFile = root, root, path

	for _, v := range []struct {
		p    *string
		name string
	}{{&o.LogDir, "logs"}, {&o.BridgeConfig, "bridge.json"}} {
		if *v.p == "" {
			*v.p = filepath.Join(root, v.name)
			continue
		}
		if *v.p, err = expand(*v.p); err != nil {
			return nil, err
		}
	}
	if o.MaxLogFiles < 1 || o.MaxLogFiles > 1000 {
		return nil, errors.New("maxlogfiles must be between 1 and 1000")
	}
	if o.LogSize < 1 || o.LogSize > 1048576 {
		return nil, errors.New("logsize must be between 1 and 1048576 KB")
	}
	if err := os.MkdirAll(root, 0o700); err != nil {
		return nil, err
	}
	return &Config{
		Options:   o,
		Created:   created,
		CustomDir: pre.AppData != "" || pre.DataDir != "",
	}, nil
}

// create writes the INI file if it is not there, and leaves an existing one
// alone. Exclusive creation, so two profiles starting at once cannot both
// believe they wrote it.
func create(path string) (bool, error) {
	if _, err := os.Stat(path); err == nil {
		return false, nil
	} else if !errors.Is(err, os.ErrNotExist) {
		return false, err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return false, err
	}
	o := defaults()
	var b bytes.Buffer
	b.WriteString("; dcr4inarow configuration. Command-line options override this file.\n" +
		"; Bridge PEM credentials are managed by the settings screen, in bridge.json.\n" +
		"; Each --appdata / --datadir directory is a separate game identity.\n\n")
	flags.NewIniParser(parser(&o)).Write(&b, flags.IniIncludeComments|flags.IniIncludeDefaults|flags.IniCommentDefaults)

	f, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
	if errors.Is(err, os.ErrExist) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	defer f.Close()
	if _, err := f.Write(b.Bytes()); err != nil {
		return false, err
	}
	if err := f.Sync(); err != nil {
		return false, err
	}
	return true, f.Close()
}
