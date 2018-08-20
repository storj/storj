// Copyright (C) 2018 Storj Labs, Inc.
// See LICENSE for copying information.

package cmd

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"
	"storj.io/storj/pkg/cfgstruct"
	"storj.io/storj/pkg/process"
	"storj.io/storj/pkg/provider"
)

var (
	setupCmd = &cobra.Command{
		Use:   "setup",
		Short: "A brief description of your command",
		RunE:  cmdSetup,
	}
	setupCfg struct {
		CA        provider.CASetupConfig
		Identity  provider.IdentitySetupConfig
		BasePath  string `default:"$CONFDIR" help:"base path for setup"`
		Overwrite bool   `default:"false" help:"whether to overwrite pre-existing configuration files"`
	}
)

func init() {
	RootCmd.AddCommand(setupCmd)
	cfgstruct.Bind(setupCmd.Flags(), &setupCfg, cfgstruct.ConfDir(defaultConfDir))
}

func cmdSetup(cmd *cobra.Command, args []string) (err error) {
	_, err = os.Stat(setupCfg.BasePath)
	if !setupCfg.Overwrite && err == nil {
		fmt.Println("A cli configuration already exists. Rerun with --overwrite")
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
		"cert-path": setupCfg.Identity.CertPath,
		"key-path":  setupCfg.Identity.KeyPath,
	}

	return process.SaveConfig(cpCmd.Flags(),
		filepath.Join(setupCfg.BasePath, "config.yaml"), o)
}
