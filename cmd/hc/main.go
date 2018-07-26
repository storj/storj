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
	"storj.io/storj/pkg/kademlia"
	"storj.io/storj/pkg/overlay"
	"storj.io/storj/pkg/peertls"
	"storj.io/storj/pkg/pointerdb"
	"storj.io/storj/pkg/process"
	"storj.io/storj/pkg/provider"
)

const (
	defaultConfFolder = "$HOME/.storj/hc"
)

var (
	rootCmd = &cobra.Command{
		Use:   "hc",
		Short: "Heavy client",
	}

	cfg struct {
		Identity  provider.IdentityConfig
		Kademlia  kademlia.Config
		PointerDB pointerdb.Config
		Overlay   overlay.Config
	}
)

func init() {
	rootCmd.AddCommand(&cobra.Command{
		Use:   "run",
		Short: "Run the heavy client",
		RunE:  cmdRun,
	})
	rootCmd.AddCommand(&cobra.Command{
		Use:   "setup",
		Short: "Create config files",
		RunE:  cmdSetup,
	})
	cfgstruct.Bind(rootCmd.PersistentFlags(), &cfg,
		cfgstruct.ConfDir(defaultConfFolder))
}

func cmdRun(cmd *cobra.Command, args []string) (err error) {
	return cfg.Identity.Run(process.Ctx(cmd),
		cfg.Kademlia, cfg.PointerDB, cfg.Overlay)
}

func cmdSetup(cmd *cobra.Command, args []string) (err error) {
	ctx := process.Ctx(cmd)

	// TODO: clean this up somehow?
	if !strings.HasSuffix(cfg.Identity.CertPath, ".leaf.cert") {
		return fmt.Errorf("certificate path should end with .leaf.cert")
	}
	certpath := strings.TrimSuffix(cfg.Identity.CertPath, ".leaf.cert")
	if !strings.HasSuffix(cfg.Identity.KeyPath, ".leaf.key") {
		return fmt.Errorf("key path should end with .leaf.key")
	}
	keypath := strings.TrimSuffix(cfg.Identity.KeyPath, ".leaf.key")

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
		filepath.Join(defaultConfFolder, "config.yaml"))
}
