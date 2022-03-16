// Copyright (C) 2021 Storj Labs, Inc.
// See LICENSE for copying information.

package main

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"

	"github.com/zeebo/clingy"
	"github.com/zeebo/errs"
	"golang.org/x/term"
)

type external struct {
	interactive bool // controls if interactive input is allowed
	quic        bool // if set, use the quic transport

	dirs struct {
		loaded  bool   // true if Setup has been called
		current string // current config directory
		legacy  string // old config directory
	}

	migration struct {
		migrated bool  // true if a migration has been attempted
		err      error // any error from the migration attempt
	}

	config struct {
		loaded bool                // true if the existing config file is successfully loaded
		values map[string][]string // the existing configuration
	}

	access struct {
		loaded      bool              // true if we've successfully loaded access.json
		defaultName string            // default access name to use from accesses
		accesses    map[string]string // map of all of the stored accesses
	}
}

func newExternal() *external {
	return &external{}
}

func (ex *external) Setup(f clingy.Flags) {
	ex.interactive = f.Flag(
		"interactive", "Controls if interactive input is allowed", true,
		clingy.Transform(strconv.ParseBool), clingy.Boolean,
		clingy.Advanced,
	).(bool)

	ex.quic = f.Flag(
		"quic", "If set, uses the quic transport", false,
		clingy.Transform(strconv.ParseBool), clingy.Boolean,
		clingy.Advanced,
	).(bool)

	ex.dirs.current = f.Flag(
		"config-dir", "Directory that stores the configuration",
		appDir(false, "storj", "uplink"),
	).(string)

	ex.dirs.legacy = f.Flag(
		"legacy-config-dir", "Directory that stores legacy configuration. Only used during migration",
		appDir(true, "storj", "uplink"),
		clingy.Advanced,
	).(string)

	ex.dirs.loaded = true
}

func (ex *external) AccessInfoFile() string   { return filepath.Join(ex.dirs.current, "access.json") }
func (ex *external) ConfigFile() string       { return filepath.Join(ex.dirs.current, "config.ini") }
func (ex *external) legacyConfigFile() string { return filepath.Join(ex.dirs.legacy, "config.yaml") }

// Dynamic is called by clingy to look up values for global flags not specified on the command
// line. This call lets us fill in values from config files or environment variables.
func (ex *external) Dynamic(name string) (vals []string, err error) {
	key := "UPLINK_" + strings.ToUpper(strings.ReplaceAll(name, "-", "_"))
	if val, ok := os.LookupEnv(key); ok {
		return []string{val}, nil
	}

	// if we have not yet loaded the directories, we should not try to migrate
	// and load the current config.
	if !ex.dirs.loaded {
		return nil, nil
	}

	// allow errors from migration and configuration loading so that calls to
	// `uplink setup` can happen and write out a new configuration.
	if err := ex.migrate(); err != nil {
		return nil, nil //nolint
	}
	if err := ex.loadConfig(); err != nil {
		return nil, nil //nolint
	}

	return ex.config.values[name], nil
}

// Wrap is called by clingy with the command to be executed.
func (ex *external) Wrap(ctx clingy.Context, cmd clingy.Command) error {
	if err := ex.migrate(); err != nil {
		return err
	}
	if err := ex.loadConfig(); err != nil {
		return err
	}
	if !ex.config.loaded {
		if err := saveInitialConfig(ctx, ex); err != nil {
			return err
		}
	}
	return cmd.Execute(ctx)
}

// PromptInput gets a line of input text from the user and returns an error if
// interactive mode is disabled.
func (ex *external) PromptInput(ctx clingy.Context, prompt string) (input string, err error) {
	if !ex.interactive {
		return "", errs.New("required user input in non-interactive setting")
	}
	fmt.Fprint(ctx.Stdout(), prompt, " ")
	var buf []byte
	var tmp [1]byte
	for {
		_, err := ctx.Stdin().Read(tmp[:])
		if errors.Is(err, io.EOF) {
			break
		} else if err != nil {
			return "", errs.Wrap(err)
		} else if tmp[0] == '\n' {
			break
		}
		buf = append(buf, tmp[0])
	}
	return string(bytes.TrimSpace(buf)), nil
}

// PromptInput gets a line of secret input from the user twice to ensure that
// it is the same value, and returns an error if interactive mode is disabled
// or if the prompt cannot be put into a mode where the typing is not echoed.
func (ex *external) PromptSecret(ctx clingy.Context, prompt string) (secret string, err error) {
	if !ex.interactive {
		return "", errs.New("required secret input in non-interactive setting")
	}

	fh, ok := ctx.Stdin().(interface{ Fd() uintptr })
	if !ok {
		return "", errs.New("unable to request secret from stdin")
	}
	fd := int(fh.Fd())

	for {
		fmt.Fprint(ctx.Stdout(), prompt, " ")

		first, err := term.ReadPassword(fd)
		if err != nil {
			return "", errs.New("unable to request secret from stdin: %w", err)
		}
		fmt.Fprintln(ctx.Stdout())

		fmt.Fprint(ctx.Stdout(), "Again: ")

		second, err := term.ReadPassword(fd)
		if err != nil {
			return "", errs.New("unable to request secret from stdin: %w", err)
		}
		fmt.Fprintln(ctx.Stdout())

		if string(first) != string(second) {
			fmt.Fprintln(ctx.Stdout(), "Values did not match. Try again.")
			fmt.Fprintln(ctx.Stdout())
			continue
		}

		return string(first), nil
	}
}

// appDir returns best base directory for the currently running operating system. It
// has a legacy bool to have it return the same values that storj.io/common/fpath.ApplicationDir
// would have returned.
func appDir(legacy bool, subdir ...string) string {
	for i := range subdir {
		if runtime.GOOS == "windows" || runtime.GOOS == "darwin" {
			subdir[i] = strings.Title(subdir[i])
		} else {
			subdir[i] = strings.ToLower(subdir[i])
		}
	}
	var appdir string
	home := os.Getenv("HOME")

	switch runtime.GOOS {
	case "windows":
		// Windows standards: https://msdn.microsoft.com/en-us/library/windows/apps/hh465094.aspx?f=255&MSPPError=-2147217396
		for _, env := range []string{"AppData", "AppDataLocal", "UserProfile", "Home"} {
			val := os.Getenv(env)
			if val != "" {
				appdir = val
				break
			}
		}
	case "darwin":
		// Mac standards: https://developer.apple.com/library/archive/documentation/FileManagement/Conceptual/FileSystemProgrammingGuide/MacOSXDirectories/MacOSXDirectories.html
		appdir = filepath.Join(home, "Library", "Application Support")
	case "linux":
		fallthrough
	default:
		// Linux standards: https://specifications.freedesktop.org/basedir-spec/basedir-spec-latest.html
		if legacy {
			appdir = os.Getenv("XDG_DATA_HOME")
			if appdir == "" && home != "" {
				appdir = filepath.Join(home, ".local", "share")
			}
		} else {
			appdir = os.Getenv("XDG_CONFIG_HOME")
			if appdir == "" && home != "" {
				appdir = filepath.Join(home, ".config")
			}
		}
	}
	return filepath.Join(append([]string{appdir}, subdir...)...)
}
