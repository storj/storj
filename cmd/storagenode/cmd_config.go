// Copyright (C) 2020 Storj Labs, Inc.
// See LICENSE for copying information.

package main

import (
	"os"
	"path/filepath"

	"github.com/spf13/cobra"

	"storj.io/common/cfgstruct"
	"storj.io/common/fpath"
	"storj.io/common/process"
)

func newConfigCmd(f *Factory) *cobra.Command {
	var cfg setupCfg

	cmd := &cobra.Command{
		Use:   "config",
		Short: "Edit config files",
		RunE: func(cmd *cobra.Command, args []string) error {
			setupDir, err := filepath.Abs(f.ConfDir)
			if err != nil {
				return err
			}

			cfg.SetupDir = setupDir
			return cmdConfig(cmd, &cfg)
		},
		Annotations: map[string]string{"type": "setup"},
	}

	process.Bind(cmd, &cfg, f.Defaults, cfgstruct.ConfDir(f.ConfDir), cfgstruct.IdentityDir(f.IdentityDir), cfgstruct.SetupMode())

	return cmd
}

func cmdConfig(cmd *cobra.Command, cfg *setupCfg) (err error) {
	setupDir, err := filepath.Abs(cfg.SetupDir)
	if err != nil {
		return err
	}
	// run setup if we can't access the config file
	conf := filepath.Join(setupDir, "config.yaml")
	if _, err := os.Stat(conf); err != nil {
		return cmdSetup(cmd, cfg)
	}

	return fpath.EditFile(conf)
}
