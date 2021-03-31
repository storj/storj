// Copyright (C) 2021 Storj Labs, Inc.
// See LICENSE for copying information.

package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/zeebo/clingy"
	"github.com/zeebo/errs"
	"github.com/zeebo/ini"
)

type globalFlags struct {
	interactive  bool
	configDir    string
	oldConfigDir string

	setup    bool
	migrated bool

	configLoaded bool
	config       map[string][]string

	accessesLoaded bool
	accessDefault  string
	accesses       map[string]string
}

func newGlobalFlags() *globalFlags {
	return &globalFlags{}
}

func (g *globalFlags) Setup(f clingy.Flags) {
	g.interactive = f.New(
		"interactive", "Controls if interactive input is allowed", true,
		clingy.Transform(strconv.ParseBool),
		clingy.Advanced,
	).(bool)

	g.configDir = f.New(
		"config-dir", "Directory that stores the configuration", appDir(false, "storj", "uplink"),
	).(string)

	g.oldConfigDir = f.New(
		"old-config-dir", "Directory that stores legacy configuration. Only used during migration", appDir(true, "storj", "uplink"),
		clingy.Advanced,
	).(string)

	g.setup = true
}

func (g *globalFlags) accessFile() string    { return filepath.Join(g.configDir, "access.json") }
func (g *globalFlags) configFile() string    { return filepath.Join(g.configDir, "config.ini") }
func (g *globalFlags) oldConfigFile() string { return filepath.Join(g.oldConfigDir, "config.yaml") }

func (g *globalFlags) Dynamic(name string) (vals []string, err error) {
	if !g.setup {
		return nil, nil
	}
	if err := g.migrate(); err != nil {
		return nil, err
	}
	if err := g.loadConfig(); err != nil {
		return nil, err
	}
	key := "UPLINK_" + strings.ToUpper(strings.ReplaceAll(name, "-", "_"))
	if val, ok := os.LookupEnv(key); ok {
		return []string{val}, nil
	}
	return g.config[name], nil
}

// loadConfig loads the configuration file from disk if it is not already loaded.
// This makes calls to loadConfig idempotent.
func (g *globalFlags) loadConfig() error {
	if g.config != nil {
		return nil
	}
	g.config = make(map[string][]string)

	fh, err := os.Open(g.configFile())
	if os.IsNotExist(err) {
		return nil
	} else if err != nil {
		return errs.Wrap(err)
	}
	defer func() { _ = fh.Close() }()

	err = ini.Read(fh, func(ent ini.Entry) error {
		if ent.Section != "" {
			ent.Key = ent.Section + "." + ent.Key
		}
		g.config[ent.Key] = append(g.config[ent.Key], ent.Value)
		return nil
	})
	if err != nil {
		return err
	}

	g.configLoaded = true
	return nil
}

func (g *globalFlags) loadAccesses() error {
	if g.accesses != nil {
		return nil
	}

	fh, err := os.Open(g.accessFile())
	if os.IsNotExist(err) {
		return nil
	} else if err != nil {
		return errs.Wrap(err)
	}
	defer func() { _ = fh.Close() }()

	var jsonInput struct {
		Default  string
		Accesses map[string]string
	}

	if err := json.NewDecoder(fh).Decode(&jsonInput); err != nil {
		return errs.Wrap(err)
	}

	g.accessDefault = jsonInput.Default
	g.accesses = jsonInput.Accesses
	g.accessesLoaded = true
	return nil
}

func (g *globalFlags) GetAccessInfo() (string, map[string]string, error) {
	if !g.accessesLoaded {
		if err := g.loadAccesses(); err != nil {
			return "", nil, err
		}
		if !g.accessesLoaded {
			return "", nil, errs.New("must configure accesses")
		}
	}

	// return a copy to avoid mutations messing things up
	accesses := make(map[string]string)
	for name, accessData := range g.accesses {
		accesses[name] = accessData
	}

	return g.accessDefault, accesses, nil
}

// SaveAccessInfo writes out the access file using the provided values.
func (g *globalFlags) SaveAccessInfo(accessDefault string, accesses map[string]string) error {
	// TODO(jeff): write it atomically

	accessFh, err := os.OpenFile(g.accessFile(), os.O_RDWR|os.O_CREATE|os.O_TRUNC, 0600)
	if err != nil {
		return errs.Wrap(err)
	}
	defer func() { _ = accessFh.Close() }()

	var jsonOutput = struct {
		Default  string
		Accesses map[string]string
	}{
		Default:  accessDefault,
		Accesses: accesses,
	}

	data, err := json.MarshalIndent(jsonOutput, "", "\t")
	if err != nil {
		return errs.Wrap(err)
	}

	if _, err := accessFh.Write(data); err != nil {
		return errs.Wrap(err)
	}

	if err := accessFh.Sync(); err != nil {
		return errs.Wrap(err)
	}

	if err := accessFh.Close(); err != nil {
		return errs.Wrap(err)
	}

	return nil
}

func (g *globalFlags) Wrap(ctx clingy.Context, cmd clingy.Cmd) error {
	if err := g.migrate(); err != nil {
		return err
	}
	if !g.configLoaded {
		// TODO(jeff): prompt for initial config setup
		_ = false
	}
	return cmd.Execute(ctx)
}
