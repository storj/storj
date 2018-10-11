// Copyright (C) 2018 Storj Labs, Inc.
// See LICENSE for copying information.

package cmd

import (
	"crypto/rand"
	"fmt"
	"os"
	"path/filepath"

	base58 "github.com/jbenet/go-base58"
	"github.com/spf13/cobra"

	"storj.io/storj/pkg/cfgstruct"
	"storj.io/storj/pkg/process"
	"storj.io/storj/pkg/provider"
)

var (
	setupCmd = &cobra.Command{
		Use:   "setup",
		Short: "Create an uplink config file",
		RunE:  cmdSetup,
	}
	setupCfg struct {
		CA            provider.CASetupConfig
		Identity      provider.IdentitySetupConfig
		BasePath      string `default:"$CONFDIR" help:"base path for setup"`
		Overwrite     bool   `default:"false" help:"whether to overwrite pre-existing configuration files"`
		SatelliteAddr string `default:"localhost:7778" help:"the address to use for the satellite"`
		APIKey        string `default:"" help:"the api key to use for the satellite"`
		EncKey        string `default:"" help:"your root encryption key"`
	}
)

func init() {
	RootCmd.AddCommand(setupCmd)
	cfgstruct.Bind(setupCmd.Flags(), &setupCfg, cfgstruct.ConfDir(defaultConfDir))
}

func cmdSetup(cmd *cobra.Command, args []string) (err error) {
	setupCfg.BasePath, err = filepath.Abs(setupCfg.BasePath)
	if err != nil {
		return err
	}

	for _, flagname := range args {
		return fmt.Errorf("%s - Invalid flag. Pleas see --help", flagname)
	}

	_, err = os.Stat(setupCfg.BasePath)
	if !setupCfg.Overwrite && err == nil {
		return fmt.Errorf("An uplink configuration already exists. Rerun with --overwrite")
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

	accessKey, err := generateAWSKey()
	if err != nil {
		return err
	}

	secretKey, err := generateAWSKey()
	if err != nil {
		return err
	}

	o := map[string]interface{}{
		"cert-path":       setupCfg.Identity.CertPath,
		"key-path":        setupCfg.Identity.KeyPath,
		"api-key":         setupCfg.APIKey,
		"pointer-db-addr": setupCfg.SatelliteAddr,
		"overlay-addr":    setupCfg.SatelliteAddr,
		"access-key":      accessKey,
		"secret-key":      secretKey,
		"enc-key":         setupCfg.EncKey,
	}

	return process.SaveConfig(runCmd.Flags(),
		filepath.Join(setupCfg.BasePath, "config.yaml"), o)
}

func generateAWSKey() (key string, err error) {
	var buf [20]byte
	_, err = rand.Read(buf[:])
	if err != nil {
		return "", err
	}
	return base58.Encode(buf[:]), nil
}
