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

	"storj.io/storj/internal/fpath"
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
		CA                 provider.CASetupConfig
		Identity           provider.IdentitySetupConfig
		BasePath           string `default:"$CONFDIR" help:"base path for setup"`
		Overwrite          bool   `default:"false" help:"whether to overwrite pre-existing configuration files"`
		SatelliteAddr      string `default:"localhost:7778" help:"the address to use for the satellite"`
		APIKey             string `default:"" help:"the api key to use for the satellite"`
		EncKey             string `default:"" help:"your root encryption key"`
		GenerateMinioCerts bool   `default:"false" help:"generate sample TLS certs for Minio GW"`
	}
)

func init() {
	defaultConfDir := fpath.ApplicationDir("storj", "uplink")
	CLICmd.AddCommand(setupCmd)
	GWCmd.AddCommand(setupCmd)
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

	valid, _ := fpath.IsValidSetupDir(setupCfg.BasePath)
	if !setupCfg.Overwrite && !valid {
		return fmt.Errorf("uplink configuration already exists (%v). Rerun with --overwrite", setupCfg.BasePath)
	}

	err = os.MkdirAll(setupCfg.BasePath, 0700)
	if err != nil {
		return err
	}

	defaultConfDir := fpath.ApplicationDir("storj", "uplink")
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

	if setupCfg.GenerateMinioCerts {
		minioCerts := filepath.Join(setupCfg.BasePath, "minio", "certs")
		if err := os.MkdirAll(minioCerts, 0744); err != nil {
			return err
		}
		if err := os.Link(setupCfg.Identity.CertPath, filepath.Join(minioCerts, "public.crt")); err != nil {
			return err
		}
		if err := os.Link(setupCfg.Identity.KeyPath, filepath.Join(minioCerts, "private.key")); err != nil {
			return err
		}
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
		"identity.cert-path":     setupCfg.Identity.CertPath,
		"identity.key-path":      setupCfg.Identity.KeyPath,
		"client.api-key":         setupCfg.APIKey,
		"client.pointer-db-addr": setupCfg.SatelliteAddr,
		"client.overlay-addr":    setupCfg.SatelliteAddr,
		"minio.access-key":       accessKey,
		"minio.secret-key":       secretKey,
		"enc.key":                setupCfg.EncKey,
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
