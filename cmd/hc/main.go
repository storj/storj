// Copyright (C) 2018 Storj Labs, Inc.
// See LICENSE for copying information.

package main

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"
	"storj.io/storj/pkg/cfgstruct"
	"storj.io/storj/pkg/kademlia"
	"storj.io/storj/pkg/overlay"
	"storj.io/storj/pkg/pointerdb"
	"storj.io/storj/pkg/process"
	"storj.io/storj/pkg/provider"
)

var (
	rootCmd = &cobra.Command{
		Use:   "hc",
		Short: "Heavy client",
	}
	runCmd = &cobra.Command{
		Use:   "run",
		Short: "Run the heavy client",
		RunE:  cmdRun,
	}
	setupCmd = &cobra.Command{
		Use:   "setup",
		Short: "Create config files",
		RunE:  cmdSetup,
	}

	runCfg struct {
		Identity  provider.IdentityConfig
		Kademlia  kademlia.Config
		PointerDB pointerdb.Config
		Overlay   overlay.Config
	}
	setupCfg struct {
		BasePath  string `default:"$CONFDIR" help:"base path for setup"`
		CA        provider.CASetupConfig
		Identity  provider.IdentitySetupConfig
		Overwrite bool `default:"false" help:"whether to overwrite pre-existing configuration files"`
	}

	defaultConfDir = "$HOME/.storj/hc"
)

func init() {
	rootCmd.AddCommand(runCmd)
	rootCmd.AddCommand(setupCmd)
	cfgstruct.Bind(runCmd.Flags(), &runCfg, cfgstruct.ConfDir(defaultConfDir))
	cfgstruct.Bind(setupCmd.Flags(), &setupCfg, cfgstruct.ConfDir(defaultConfDir))
}

func cmdRun(cmd *cobra.Command, args []string) (err error) {
	return runCfg.Identity.Run(process.Ctx(cmd),
		runCfg.Kademlia, runCfg.PointerDB, runCfg.Overlay)
}

func cmdSetup(cmd *cobra.Command, args []string) (err error) {
	_, err = os.Stat(setupCfg.BasePath)
	if !setupCfg.Overwrite && err == nil {
		fmt.Println("An hc configuration already exists. Rerun with --overwrite")
		return nil
	}

	err = os.MkdirAll(setupCfg.BasePath, 0700)
	if err != nil {
		return err
	}

	// TODO: handle setting base path *and* identity file paths via args
	// NB: if base path is set this overrides identity and CA path options
	if setupCfg.BasePath != defaultConfDir {
		setupCfg.CA.CertPath = filepath.Join(setupCfg.BasePath, "ca.cert")
		setupCfg.CA.KeyPath = filepath.Join(setupCfg.BasePath, "ca.key")
		setupCfg.Identity.CertPath = filepath.Join(setupCfg.BasePath, "identity.cert")
		setupCfg.Identity.KeyPath = filepath.Join(setupCfg.BasePath, "identity.key")
	}
	err = provider.SetupIdentity(process.Ctx(cmd), setupCfg.CA, setupCfg.Identity)
	if err != nil {
		return err
	}

	o := map[string]interface{}{
		"identity.cert-path": setupCfg.CA.CertPath,
		"identity.key-path":  setupCfg.CA.CertPath,
	}

	return process.SaveConfig(runCmd.Flags(),
		filepath.Join(setupCfg.BasePath, "config.yaml"), o)
}

func main() {
	runCmd.Flags().String("config",
		filepath.Join(defaultConfDir, "config.yaml"), "path to configuration")
	process.Exec(rootCmd)
}
