// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package main

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"

	"storj.io/storj/internal/fpath"
	"storj.io/storj/pkg/certificates"
	"storj.io/storj/pkg/cfgstruct"
	"storj.io/storj/pkg/identity"
	"storj.io/storj/pkg/process"
)

var (
	setupCmd = &cobra.Command{
		Use:         "setup",
		Short:       "Setup a certificate signing server",
		RunE:        cmdSetup,
		Annotations: map[string]string{"type": "setup"},
	}

	setupCfg struct {
		// NB: cert and key paths overridden in setup
		CA identity.CASetupConfig
		// NB: cert and key paths overridden in setup
		Identity  identity.SetupConfig
		Signer    certificates.CertServerConfig
		Overwrite bool `default:"false" help:"if true ca, identity, and authorization db will be overwritten/truncated"`
	}
)

func init() {
	rootCmd.AddCommand(setupCmd)
	cfgstruct.BindSetup(setupCmd.Flags(), &setupCfg, cfgstruct.ConfDir(defaultConfDir))
}

func cmdSetup(cmd *cobra.Command, args []string) error {
	setupDir, err := filepath.Abs(*confDir)
	if err != nil {
		return err
	}

	err = os.MkdirAll(setupDir, 0700)
	if err != nil {
		return err
	}

	valid, err := fpath.IsValidSetupDir(setupDir)
	if err != nil {
		return err
	}
	if !setupCfg.Overwrite && !valid {
		fmt.Printf("certificate signer configuration already exists (%v). rerun with --overwrite\n", setupDir)
		return nil
	}

	if setupCfg.Overwrite {
		setupCfg.CA.Overwrite = true
		setupCfg.Identity.Overwrite = true
		setupCfg.Signer.Overwrite = true
	}

	if _, err := setupCfg.Signer.NewAuthDB(); err != nil {
		return err
	}
	setupCfg.CA.CertPath = filepath.Join(setupDir, "ca.cert")
	setupCfg.CA.KeyPath = filepath.Join(setupDir, "ca.key")
	setupCfg.Identity.CertPath = filepath.Join(setupDir, "identity.cert")
	setupCfg.Identity.KeyPath = filepath.Join(setupDir, "identity.key")

	err = identity.SetupIdentity(process.Ctx(cmd), setupCfg.CA, setupCfg.Identity)
	if err != nil {
		return err
	}

	o := map[string]interface{}{
		"ca.cert-path":       setupCfg.CA.CertPath,
		"ca.key-path":        setupCfg.CA.KeyPath,
		"identity.cert-path": setupCfg.Identity.CertPath,
		"identity.key-path":  setupCfg.Identity.KeyPath,
		"log.level":          "info",
	}
	return process.SaveConfigWithAllDefaults(cmd.Flags(), filepath.Join(setupDir, "config.yaml"), o)
}
