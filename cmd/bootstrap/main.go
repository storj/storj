// Copyright (C) 2018 Storj Labs, Inc.
// See LICENSE for copying information.

package main

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"
	"go.uber.org/zap"

	"storj.io/storj/internal/fpath"
	"storj.io/storj/pkg/cfgstruct"
	"storj.io/storj/pkg/identity"
	"storj.io/storj/pkg/kademlia"
	"storj.io/storj/pkg/process"
	"storj.io/storj/pkg/server"
)

// Bootstrap defines a bootstrap node configuration
type Bootstrap struct {
	CA       identity.CASetupConfig `setup:"true"`
	Identity identity.SetupConfig   `setup:"true"`

	Server   server.Config
	Kademlia kademlia.BootstrapConfig
}

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

	cfg Bootstrap

	defaultConfDir string
	confDir        *string
)

const (
	defaultServerAddr = ":28967"
)

func init() {
	defaultConfDir = fpath.ApplicationDir("storj", "bootstrap")

	dirParam := cfgstruct.FindConfigDirParam()
	if dirParam != "" {
		defaultConfDir = dirParam
	}

	confDir = rootCmd.PersistentFlags().String("config-dir", defaultConfDir, "main directory for bootstrap configuration")

	rootCmd.AddCommand(runCmd)
	rootCmd.AddCommand(setupCmd)
	cfgstruct.Bind(runCmd.Flags(), &cfg, cfgstruct.ConfDir(defaultConfDir))
	cfgstruct.BindSetup(setupCmd.Flags(), &cfg, cfgstruct.ConfDir(defaultConfDir))
}

func cmdRun(cmd *cobra.Command, args []string) (err error) {
	ctx := process.Ctx(cmd)
	if err := process.InitMetricsWithCertPath(ctx, nil, cfg.Identity.CertPath); err != nil {
		zap.S().Errorf("Failed to initialize telemetry batcher: %+v", err)
	}
	return cfg.Server.Run(ctx, nil, cfg.Kademlia)
}

func cmdSetup(cmd *cobra.Command, args []string) (err error) {
	setupDir, err := filepath.Abs(*confDir)
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

	if setupDir != defaultConfDir {
		cfg.CA.CertPath = filepath.Join(setupDir, "ca.cert")
		cfg.CA.KeyPath = filepath.Join(setupDir, "ca.key")
		cfg.Identity.CertPath = filepath.Join(setupDir, "identity.cert")
		cfg.Identity.KeyPath = filepath.Join(setupDir, "identity.key")
	}
	err = identity.SetupIdentity(process.Ctx(cmd), cfg.CA, cfg.Identity)
	if err != nil {
		return err
	}

	overrides := map[string]interface{}{
		"identity.cert-path":      cfg.Identity.CertPath,
		"identity.key-path":       cfg.Identity.KeyPath,
		"identity.server.address": defaultServerAddr,
		"kademlia.bootstrap-addr": "localhost" + defaultServerAddr,
	}

	return process.SaveConfigWithAllDefaults(cmd.Flags(), filepath.Join(setupDir, "config.yaml"), overrides)
}

func main() {
	process.Exec(rootCmd)
}
