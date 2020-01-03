// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package main

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"

	"storj.io/common/fpath"
	"storj.io/storj/certificate/authorization"
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
	if !setupCfg.Overwrite && !valid {
		fmt.Printf("certificate signer configuration already exists (%v). rerun with --overwrite\n", setupDir)
		return nil
	}

	authorizationDB, err := authorization.NewDBFromCfg(setupCfg.Config.AuthorizationDB)
	if err != nil {
		return err
	}
	if err := authorizationDB.Close(); err != nil {
		return err
	}

	return process.SaveConfig(cmd, filepath.Join(setupDir, "config.yaml"),
		process.SaveConfigWithOverrides(map[string]interface{}{
			"signer.cert-path":   setupCfg.Signer.CertPath,
			"signer.key-path":    setupCfg.Signer.KeyPath,
			"identity.cert-path": setupCfg.Identity.CertPath,
			"identity.key-path":  setupCfg.Identity.KeyPath,
		}))
}
