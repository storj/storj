// Copyright (C) 2020 Storj Labs, Inc.
// See LICENSE for copying information.

package main

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"
	"github.com/zeebo/errs"
	"go.uber.org/zap"

	"storj.io/common/fpath"
	"storj.io/private/cfgstruct"
	"storj.io/private/process"
	"storj.io/storj/multinode"
	"storj.io/storj/multinode/multinodedb"
)

// Config defines multinode configuration.
type Config struct {
	Database string `help:"multinode database connection string" releaseDefault:"postgres://" devDefault:"postgres://"`

	multinode.Config
}

var (
	rootCmd = &cobra.Command{
		Use:   "multinode",
		Short: "Multinode Dashboard",
	}
	runCmd = &cobra.Command{
		Use:   "run",
		Short: "Run the multinode dashboard",
		RunE:  cmdRun,
	}
	createSchemaCmd = &cobra.Command{
		Use:   "create-schema",
		Short: "Create schemas for multinode dashboard databases",
		RunE:  cmdCreateSchema,
	}
	setupCmd = &cobra.Command{
		Use:         "setup",
		Short:       "Create config files",
		RunE:        cmdSetup,
		Annotations: map[string]string{"type": "setup"},
	}

	runCfg      Config
	setupCfg    Config
	confDir     string
	identityDir string
)

func main() {
	process.ExecCustomDebug(rootCmd)
}

func init() {
	defaultConfDir := fpath.ApplicationDir("storj", "multinode")
	defaultIdentityDir := fpath.ApplicationDir("storj", "identity", "multinode")
	cfgstruct.SetupFlag(zap.L(), rootCmd, &confDir, "config-dir", defaultConfDir, "main directory for multinode configuration")
	cfgstruct.SetupFlag(zap.L(), rootCmd, &identityDir, "identity-dir", defaultIdentityDir, "main directory for multinode identity credentials")
	defaults := cfgstruct.DefaultsFlag(rootCmd)

	rootCmd.AddCommand(setupCmd)
	rootCmd.AddCommand(runCmd)
	rootCmd.AddCommand(createSchemaCmd)

	process.Bind(runCmd, &runCfg, defaults, cfgstruct.ConfDir(confDir), cfgstruct.IdentityDir(identityDir))
	process.Bind(createSchemaCmd, &runCfg, defaults, cfgstruct.ConfDir(confDir), cfgstruct.IdentityDir(identityDir))
	process.Bind(setupCmd, &setupCfg, defaults, cfgstruct.ConfDir(confDir), cfgstruct.IdentityDir(identityDir), cfgstruct.SetupMode())
}

func cmdSetup(cmd *cobra.Command, args []string) (err error) {
	setupDir, err := filepath.Abs(confDir)
	if err != nil {
		return err
	}

	valid, _ := fpath.IsValidSetupDir(setupDir)
	if !valid {
		return fmt.Errorf("multinode configuration already exists (%v)", setupDir)
	}

	err = os.MkdirAll(setupDir, 0700)
	if err != nil {
		return err
	}

	return process.SaveConfig(cmd, filepath.Join(setupDir, "config.yaml"))
}

func cmdRun(cmd *cobra.Command, args []string) (err error) {
	ctx, _ := process.Ctx(cmd)
	log := zap.L()

	runCfg.Debug.Address = *process.DebugAddrFlag

	identity, err := runCfg.Identity.Load()
	if err != nil {
		log.Error("failed to load identity", zap.Error(err))
		return errs.New("failed to load identity: %+v", err)
	}

	db, err := multinodedb.Open(ctx, log.Named("db"), runCfg.Database)
	if err != nil {
		return errs.New("error connecting to master database on multinode: %+v", err)
	}
	defer func() {
		err = errs.Combine(err, db.Close())
	}()

	peer, err := multinode.New(log, identity, runCfg.Config, db)
	if err != nil {
		return err
	}

	runError := peer.Run(ctx)
	closeError := peer.Close()
	return errs.Combine(runError, closeError)
}

func cmdCreateSchema(cmd *cobra.Command, args []string) (err error) {
	ctx, _ := process.Ctx(cmd)
	log := zap.L()

	db, err := multinodedb.Open(ctx, log.Named("db"), runCfg.Database)
	if err != nil {
		return errs.New("error connecting to master database on multinode: %+v", err)
	}
	defer func() {
		err = errs.Combine(err, db.Close())
	}()

	err = db.CreateSchema(ctx)
	if err != nil {
		return errs.New("error creating database schemas for multinode dashboard db: %+v", err)
	}

	return nil
}
