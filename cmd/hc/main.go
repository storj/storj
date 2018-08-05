// Copyright (C) 2018 Storj Labs, Inc.
// See LICENSE for copying information.

package main

import (
	"context"
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
		BasePath    string `default:"$CONFDIR" help:"base path for setup"`
		Concurrency uint   `default:"4" help:"number of concurrent workers for certificate authority generation"`
		CA          provider.CAConfig
		Identity    provider.IdentityConfig
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
	err = os.MkdirAll(setupCfg.BasePath, 0700)
	if err != nil {
		return err
	}

	// GenerateCA CA
	ca := provider.GenerateCA(context.Background(), setupCfg.CA.Difficulty, 4)
	fi, err := ca.GenerateIdentity()
	if err != nil {
		return err
	}

	err = setupCfg.CA.Save(ca)
	if err != nil {
		return err
	}

	err = setupCfg.Identity.Save(fi)
	if err != nil {
		return err
	}

	return process.SaveConfig(runCmd.Flags(),
		filepath.Join(setupCfg.BasePath, "config.yaml"), nil)
}

func main() {
	runCmd.Flags().String("config",
		filepath.Join(defaultConfDir, "config.yaml"), "path to configuration")
	process.Exec(rootCmd)
}
