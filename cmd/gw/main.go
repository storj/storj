// Copyright (C) 2018 Storj Labs, Inc.
// See LICENSE for copying information.

package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"

	"storj.io/storj/pkg/cfgstruct"
	"storj.io/storj/pkg/miniogw"
	"storj.io/storj/pkg/peertls"
	"storj.io/storj/pkg/process"
)

var (
	rootCmd = &cobra.Command{
		Use:   "gw",
		Short: "Gateway",
	}

	cfg miniogw.Config

	defaultConfDir = "$HOME/.storj/gw"
)

func init() {
	rootCmd.AddCommand(&cobra.Command{
		Use:   "run",
		Short: "Run the gateway",
		RunE:  cmdRun,
	})
	rootCmd.AddCommand(&cobra.Command{
		Use:   "setup",
		Short: "Create config files",
		RunE:  cmdSetup,
	})
	cfgstruct.Bind(rootCmd.PersistentFlags(), &cfg,
		cfgstruct.ConfDir(defaultConfDir))
}

func cmdRun(cmd *cobra.Command, args []string) (err error) {
	return cfg.Run(process.Ctx(cmd))
}

func cmdSetup(cmd *cobra.Command, args []string) (err error) {
	ctx := process.Ctx(cmd)

	// TODO: clean this up somehow?
	if !strings.HasSuffix(cfg.IdentityConfig.CertPath, ".leaf.cert") {
		return fmt.Errorf("certificate path should end with .leaf.cert")
	}
	certpath := strings.TrimSuffix(cfg.IdentityConfig.CertPath, ".leaf.cert")
	if !strings.HasSuffix(cfg.IdentityConfig.KeyPath, ".leaf.key") {
		return fmt.Errorf("key path should end with .leaf.key")
	}
	keypath := strings.TrimSuffix(cfg.IdentityConfig.KeyPath, ".leaf.key")

	err = os.MkdirAll(filepath.Dir(certpath), 0700)
	if err != nil {
		return err
	}
	err = os.MkdirAll(filepath.Dir(keypath), 0700)
	if err != nil {
		return err
	}

	_, err = peertls.NewTLSFileOptions(certpath, keypath, true, false)
	if err != nil {
		return err
	}

	err = os.MkdirAll(filepath.Dir(process.CfgPath(ctx)), 0700)
	if err != nil {
		return err
	}

	return process.SaveConfigAs(cmd, process.CfgPath(ctx))
}

func main() {
	process.ExecuteWithConfig(rootCmd,
		filepath.Join(defaultConfDir, "config.yaml"))
}
