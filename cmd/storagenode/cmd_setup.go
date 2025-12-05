// Copyright (C) 2020 Storj Labs, Inc.
// See LICENSE for copying information.

package main

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"
	"go.uber.org/zap"

	"storj.io/common/cfgstruct"
	"storj.io/common/fpath"
	"storj.io/common/process"
	"storj.io/storj/storagenode/storagenodedb"
)

const (
	defaultServerAddr        = ":28967"
	defaultPrivateServerAddr = "127.0.0.1:7778"
)

type setupCfg struct {
	StorageNodeFlags

	SetupDir string `internal:"true" help:"path to setup directory"`
}

func newSetupCmd(f *Factory) *cobra.Command {
	var setupCfg setupCfg

	cmd := &cobra.Command{
		Use:   "setup",
		Short: "Create config files",
		RunE: func(cmd *cobra.Command, args []string) error {
			setupDir, err := filepath.Abs(f.ConfDir)
			if err != nil {
				return err
			}
			setupCfg.SetupDir = setupDir
			return cmdSetup(cmd, &setupCfg)
		},
		Annotations: map[string]string{"type": "setup"},
	}

	process.Bind(cmd, &setupCfg, f.Defaults, cfgstruct.ConfDir(f.ConfDir), cfgstruct.IdentityDir(f.IdentityDir), cfgstruct.SetupMode())

	return cmd
}

func cmdSetup(cmd *cobra.Command, cfg *setupCfg) (err error) {
	ctx, _ := process.Ctx(cmd)

	valid, _ := fpath.IsValidSetupDir(cfg.SetupDir)
	if !valid {
		return fmt.Errorf("storagenode configuration already exists (%v)", cfg.SetupDir)
	}

	identity, err := cfg.Identity.Load()
	if err != nil {
		return err
	}

	err = os.MkdirAll(cfg.SetupDir, 0700)
	if err != nil {
		return err
	}

	overrides := map[string]interface{}{
		"log.level": "info",
	}
	serverAddress := cmd.Flag("server.address")
	if !serverAddress.Changed {
		overrides[serverAddress.Name] = defaultServerAddr
	}

	serverPrivateAddress := cmd.Flag("server.private-address")
	if !serverPrivateAddress.Changed {
		overrides[serverPrivateAddress.Name] = defaultPrivateServerAddr
	}

	configFile := filepath.Join(cfg.SetupDir, "config.yaml")
	err = process.SaveConfig(cmd, configFile, process.SaveConfigWithOverrides(overrides))
	if err != nil {
		return err
	}

	if cfg.EditConf {
		return fpath.EditFile(configFile)
	}

	// create db
	db, err := storagenodedb.OpenNew(ctx, zap.L().Named("db"), cfg.DatabaseConfig())
	if err != nil {
		return err
	}

	if err := db.Pieces().CreateVerificationFile(ctx, identity.ID); err != nil {
		return err
	}

	return db.Close()
}
