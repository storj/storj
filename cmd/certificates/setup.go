// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package main

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"

	"storj.io/storj/internal/fpath"
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
)

func cmdSetup(cmd *cobra.Command, args []string) error {
	setupDir, err := filepath.Abs(confDir)
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
	if !config.Overwrite && !valid {
		fmt.Printf("certificate signer configuration already exists (%v). rerun with --overwrite\n", setupDir)
		return nil
	}

	if config.Overwrite {
		config.CA.Overwrite = true
		config.Identity.Overwrite = true
		config.Signer.Overwrite = true
	}

	if _, err := config.Signer.NewAuthDB(); err != nil {
		return err
	}

	if config.Identity.Status() != identity.CertKey {
		return errors.New("identity is missing")
	}

	overrides := map[string]interface{}{
		"ca.cert-path":       config.CA.CertPath,
		"ca.key-path":        config.CA.KeyPath,
		"identity.cert-path": config.Identity.CertPath,
		"identity.key-path":  config.Identity.KeyPath,
	}
	return process.SaveConfigWithAllDefaults(cmd.Flags(), filepath.Join(setupDir, "config.yaml"), overrides)
}
