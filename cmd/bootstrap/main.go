// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package main

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"
	"github.com/zeebo/errs"
	"go.uber.org/zap"

	"storj.io/storj/bootstrap"
	"storj.io/storj/bootstrap/bootstrapdb"
	"storj.io/storj/internal/fpath"
	"storj.io/storj/pkg/cfgstruct"
	"storj.io/storj/pkg/process"
)

var (
	rootCmd = &cobra.Command{
		Use:   "bootstrap",
		Short: "bootstrap",
	}
	runCmd = &cobra.Command{
		Use:   "run",
		Short: "Run the bootstrap server",
		RunE:  cmdRun,
	}
	setupCmd = &cobra.Command{
		Use:         "setup",
		Short:       "Create config files",
		RunE:        cmdSetup,
		Annotations: map[string]string{"type": "setup"},
	}

	runCfg   bootstrap.Config
	setupCfg bootstrap.Config

	defaultConfDir     = fpath.ApplicationDir("storj", "bootstrap")
	defaultIdentityDir = fpath.ApplicationDir("storj", "identity", "bootstrap")
	confDir            string
	identityDir        string
)

const (
	defaultServerAddr = ":28967"
)

func init() {
	confDirParam := cfgstruct.FindConfigDirParam()
	if confDirParam != "" {
		defaultConfDir = confDirParam
	}
	identityDirParam := cfgstruct.FindIdentityDirParam()
	if identityDirParam != "" {
		defaultIdentityDir = identityDirParam
	}

	rootCmd.PersistentFlags().StringVar(&confDir, "config-dir", defaultConfDir, "main directory for bootstrap configuration")
	err := rootCmd.PersistentFlags().SetAnnotation("config-dir", "setup", []string{"true"})
	if err != nil {
		zap.S().Error("Failed to set 'setup' annotation for 'config-dir'")
	}
	rootCmd.PersistentFlags().StringVar(&identityDir, "identity-dir", defaultIdentityDir, "main directory for bootstrap identity credentials")
	err = rootCmd.PersistentFlags().SetAnnotation("identity-dir", "setup", []string{"true"})
	if err != nil {
		zap.S().Error("Failed to set 'setup' annotation for 'config-dir'")
	}

	rootCmd.AddCommand(runCmd)
	rootCmd.AddCommand(setupCmd)
	cfgstruct.Bind(runCmd.Flags(), &runCfg, cfgstruct.ConfDir(defaultConfDir), cfgstruct.IdentityDir(defaultIdentityDir))
	cfgstruct.BindSetup(setupCmd.Flags(), &setupCfg, cfgstruct.ConfDir(defaultConfDir), cfgstruct.IdentityDir(defaultIdentityDir))
}

func cmdRun(cmd *cobra.Command, args []string) (err error) {
	log := zap.L()

	identity, err := runCfg.Identity.Load()
	if err != nil {
		zap.S().Fatal(err)
	}

	if err := runCfg.Verify(log); err != nil {
		log.Sugar().Error("Invalid configuration: ", err)
		return err
	}

	ctx := process.Ctx(cmd)
	if err := process.InitMetricsWithCertPath(ctx, nil, runCfg.Identity.CertPath); err != nil {
		zap.S().Error("Failed to initialize telemetry batcher: ", err)
	}

	db, err := bootstrapdb.New(bootstrapdb.Config{
		Kademlia: runCfg.Kademlia.DBPath,
	})
	if err != nil {
		return errs.New("Error starting master database on bootstrap: %+v", err)
	}

	defer func() {
		err = errs.Combine(err, db.Close())
	}()

	err = db.CreateTables()
	if err != nil {
		return errs.New("Error creating tables for master database on bootstrap: %+v", err)
	}

	peer, err := bootstrap.New(log, identity, db, runCfg)
	if err != nil {
		return err
	}

	runError := peer.Run(ctx)
	closeError := peer.Close()

	return errs.Combine(runError, closeError)
}

func cmdSetup(cmd *cobra.Command, args []string) (err error) {
	setupDir, err := filepath.Abs(confDir)
	if err != nil {
		return err
	}

	valid, _ := fpath.IsValidSetupDir(setupDir)
	if !valid {
		return fmt.Errorf("bootstrap configuration already exists (%v)", setupDir)
	}

	err = os.MkdirAll(setupDir, 0700)
	if err != nil {
		return err
	}

	overrides := map[string]interface{}{}

	serverAddress := cmd.Flag("server.address")
	if !serverAddress.Changed {
		overrides[serverAddress.Name] = defaultServerAddr
	}

	kademliaBootstrapAddr := cmd.Flag("kademlia.bootstrap-addr")
	if !kademliaBootstrapAddr.Changed {
		overrides[kademliaBootstrapAddr.Name] = "localhost" + defaultServerAddr
	}

	return process.SaveConfigWithAllDefaults(cmd.Flags(), filepath.Join(setupDir, "config.yaml"), overrides)
}

func main() {
	process.Exec(rootCmd)
}
