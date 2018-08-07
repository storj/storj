// Copyright (C) 2018 Storj Labs, Inc.
// See LICENSE for copying information.

package cmd

import (
	"os"
	"path/filepath"

	"github.com/spf13/cobra"
	"storj.io/storj/pkg/cfgstruct"
	"storj.io/storj/pkg/peertls"
	"storj.io/storj/pkg/process"
)

var (
	setupCmd = &cobra.Command{
		Use:   "setup",
		Short: "A brief description of your command",
		RunE:  cmdSetup,
	}

	setupCfg struct {
		BasePath string `default:"$CONFDIR" help:"base path for setup"`
	}
)

func init() {
	defaultConfDir := "$HOME/.storj/clt"

	RootCmd.AddCommand(setupCmd)
	cfgstruct.Bind(setupCmd.Flags(), &setupCfg, cfgstruct.ConfDir(defaultConfDir))
}

func cmdSetup(cmd *cobra.Command, args []string) error {
	err := os.MkdirAll(setupCfg.BasePath, 0700)
	if err != nil {
		return err
	}

	identityPath := filepath.Join(setupCfg.BasePath, "identity")
	_, err = peertls.NewTLSFileOptions(identityPath, identityPath, true, false)
	if err != nil {
		return err
	}

	return process.SaveConfig(cpCmd.Flags(),
		filepath.Join(setupCfg.BasePath, "config.yaml"), nil)
}
