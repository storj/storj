// Copyright (C) 2018 Storj Labs, Inc.
// See LICENSE for copying information.

package main

import (
	"os"
	"path/filepath"

	"github.com/spf13/cobra"

	"storj.io/storj/pkg/cfgstruct"
	"storj.io/storj/pkg/kademlia"
	"storj.io/storj/pkg/peertls"
	"storj.io/storj/pkg/piecestore/psservice"
	"storj.io/storj/pkg/process"
	"storj.io/storj/pkg/provider"
)

var (
	rootCmd = &cobra.Command{
		Use:   "farmer",
		Short: "Farmer",
	}
	runCmd = &cobra.Command{
		Use:   "run",
		Short: "Run the farmer",
		RunE:  cmdRun,
	}
	setupCmd = &cobra.Command{
		Use:   "setup",
		Short: "Create config files",
		RunE:  cmdSetup,
	}

	runCfg struct {
		Identity provider.IdentityConfig
		Kademlia kademlia.Config
		Storage  psservice.Config
	}
	setupCfg struct {
		BasePath string `default:"$CONFDIR" help:"base path for setup"`
	}

	defaultConfDir = "$HOME/.storj/farmer"
)

func init() {
	rootCmd.AddCommand(runCmd)
	rootCmd.AddCommand(setupCmd)
	cfgstruct.Bind(runCmd.Flags(), &runCfg, cfgstruct.ConfDir(defaultConfDir))
	cfgstruct.Bind(setupCmd.Flags(), &setupCfg, cfgstruct.ConfDir(defaultConfDir))
}

func cmdRun(cmd *cobra.Command, args []string) (err error) {
	return runCfg.Identity.Run(process.Ctx(cmd), runCfg.Kademlia, runCfg.Storage)
}

func cmdSetup(cmd *cobra.Command, args []string) (err error) {
	setupCfg.BasePath, err = filepath.Abs(setupCfg.BasePath)
	if err != nil {
		return err
	}

	err = os.MkdirAll(setupCfg.BasePath, 0700)
	if err != nil {
		return err
	}

	identityPath := filepath.Join(setupCfg.BasePath, "identity")
	_, err = peertls.NewTLSFileOptions(identityPath, identityPath, true, false)
	if err != nil {
		return err
	}

	overrides := map[string]interface{}{
		"identity.cert-path": filepath.Join(
			setupCfg.BasePath, "identity.leaf.cert"),
		"identity.key-path": filepath.Join(
			setupCfg.BasePath, "identity.leaf.key"),
		"storage.path": filepath.Join(
			setupCfg.BasePath, "storage"),
	}

	return process.SaveConfig(runCmd.Flags(),
		filepath.Join(setupCfg.BasePath, "config.yaml"), overrides)
}

func main() {
	runCmd.Flags().String("config",
		filepath.Join(defaultConfDir, "config.yaml"), "path to configuration")
	process.Exec(rootCmd)
}
