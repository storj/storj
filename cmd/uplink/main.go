// Copyright (C) 2018 Storj Labs, Inc.
// See LICENSE for copying information.

package main

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"

	"storj.io/storj/pkg/cfgstruct"
	"storj.io/storj/pkg/miniogw"
	"storj.io/storj/pkg/process"
	"storj.io/storj/pkg/provider"
)

var (
	rootCmd = &cobra.Command{
		Use:   "uplink",
		Short: "Uplink",
	}
	runCmd = &cobra.Command{
		Use:   "run",
		Short: "Run the uplink",
		RunE:  cmdRun,
	}
	setupCmd = &cobra.Command{
		Use:   "setup",
		Short: "Create config files",
		RunE:  cmdSetup,
	}

	runCfg   miniogw.Config
	setupCfg struct {
		CA            provider.CASetupConfig
		Identity      provider.IdentitySetupConfig
		BasePath      string `default:"$CONFDIR" help:"base path for setup"`
		Concurrency   uint   `default:"4" help:"number of concurrent workers for certificate authority generation"`
		Overwrite     bool   `default:"false" help:"whether to overwrite pre-existing configuration files"`
		SatelliteAddr string `default:"localhost:7778" help:"the address to use for the satellite"`
		APIKey        string `default:"" help:"the api key to use for the satellite"`
	}

	defaultConfDir = "$HOME/.storj/uplink"
)

func init() {
	rootCmd.AddCommand(runCmd)
	rootCmd.AddCommand(setupCmd)
	cfgstruct.Bind(runCmd.Flags(), &runCfg, cfgstruct.ConfDir(defaultConfDir))
	cfgstruct.Bind(setupCmd.Flags(), &setupCfg, cfgstruct.ConfDir(defaultConfDir))
}

func cmdRun(cmd *cobra.Command, args []string) (err error) {
	return runCfg.Run(process.Ctx(cmd))
}

func cmdSetup(cmd *cobra.Command, args []string) (err error) {
	setupCfg.BasePath, err = filepath.Abs(setupCfg.BasePath)
	if err != nil {
		return err
	}

	_, err = os.Stat(setupCfg.BasePath)
	if !setupCfg.Overwrite && err == nil {
		fmt.Println("An uplink configuration already exists. Rerun with --overwrite")
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
		"cert-path":       setupCfg.Identity.CertPath,
		"key-path":        setupCfg.Identity.KeyPath,
		"api-key":         setupCfg.APIKey,
		"pointer-db-addr": setupCfg.SatelliteAddr,
		"overlay-addr":    setupCfg.SatelliteAddr,
	}

	return process.SaveConfig(runCmd.Flags(),
		filepath.Join(setupCfg.BasePath, "config.yaml"), o)
}

func main() {
	runCmd.Flags().String("config",
		filepath.Join(defaultConfDir, "config.yaml"), "path to configuration")
	process.Exec(rootCmd)
}
