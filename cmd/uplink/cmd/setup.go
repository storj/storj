// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package cmd

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"
	"go.uber.org/zap"

	"storj.io/storj/internal/fpath"
	"storj.io/storj/pkg/cfgstruct"
	"storj.io/storj/pkg/process"
)

var (
	setupCmd = &cobra.Command{
		Use:         "setup",
		Short:       "Create an uplink config file",
		RunE:        cmdSetup,
		Annotations: map[string]string{"type": "setup"},
	}

	setupCfg UplinkFlags

	defaultConfDir     = fpath.ApplicationDir("storj", "uplink")
	defaultIdentityDir = fpath.ApplicationDir("storj", "identity", "uplink")

	confDir     string
	identityDir string
)

func init() {
	confDirParam := cfgstruct.FindConfigDirParam()
	if confDirParam != "" {
		defaultConfDir = confDirParam
	}
	identityDirParam := cfgstruct.FindIdentityDirParam()
	if identityDirParam != "" {
		defaultIdentityDir = identityDirParam
	}

	RootCmd.PersistentFlags().StringVar(&confDir, "config-dir", defaultConfDir, "main directory for setup configuration")
	err := RootCmd.PersistentFlags().SetAnnotation("config-dir", "setup", []string{"true"})
	if err != nil {
		zap.S().Error("Failed to set 'setup' annotation for 'config-dir'")
	}

	RootCmd.PersistentFlags().StringVar(&identityDir, "identity-dir", defaultIdentityDir, "main directory for uplink identity credentials")
	err = RootCmd.PersistentFlags().SetAnnotation("identity-dir", "setup", []string{"true"})
	if err != nil {
		zap.S().Error("Failed to set 'setup' annotation for 'config-dir'")
	}

	RootCmd.AddCommand(setupCmd)
	cfgstruct.BindSetup(setupCmd.Flags(), &setupCfg, cfgstruct.ConfDir(defaultConfDir), cfgstruct.IdentityDir(defaultIdentityDir))
}

func cmdSetup(cmd *cobra.Command, args []string) (err error) {
	setupDir, err := filepath.Abs(confDir)
	if err != nil {
		return err
	}

	valid, _ := fpath.IsValidSetupDir(setupDir)
	if !valid {
		return fmt.Errorf("uplink configuration already exists (%v)", setupDir)
	}

	err = os.MkdirAll(setupDir, 0700)
	if err != nil {
		return err
	}

	overrides := map[string]interface{}{
		"client.api-key":         setupCfg.APIKey,
		"client.pointer-db-addr": setupCfg.SatelliteAddr,
		"client.overlay-addr":    setupCfg.SatelliteAddr,
	}

	return process.SaveConfigWithAllDefaults(cmd.Flags(), filepath.Join(setupDir, "config.yaml"), overrides)
}
