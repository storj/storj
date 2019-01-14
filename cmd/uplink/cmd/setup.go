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
	"storj.io/storj/pkg/miniogw"
	"storj.io/storj/pkg/process"
	"storj.io/storj/pkg/provider"
)

var (
	setupCmd = &cobra.Command{
		Use:         "setup",
		Short:       "Create an uplink config file",
		RunE:        cmdSetup,
		Annotations: map[string]string{"type": "setup"},
	}
	setupCfg struct {
		CA                 provider.CASetupConfig       `setup:"true"`
		Identity           provider.IdentitySetupConfig `setup:"true"`
		APIKey             string                       `default:"" help:"the api key to use for the satellite" setup:"true"`
		EncKey             string                       `default:"" help:"your root encryption key" setup:"true"`
		GenerateMinioCerts bool                         `default:"false" help:"generate sample TLS certs for Minio GW" setup:"true"`
		SatelliteAddr      string                       `default:"localhost:7778" help:"the address to use for the satellite" setup:"true"`

		Server miniogw.ServerConfig
		Minio  miniogw.MinioConfig
		Client miniogw.ClientConfig
		RS     miniogw.RSConfig
		Enc    miniogw.EncryptionConfig
	}

	cliConfDir *string
	gwConfDir  *string
)

func init() {
	defaultConfDir := fpath.ApplicationDir("storj", "uplink")

	dirParam := cfgstruct.FindConfigDirParam()
	if dirParam != "" {
		defaultConfDir = dirParam
	}

	cliConfDir = CLICmd.PersistentFlags().String("config-dir", defaultConfDir, "main directory for setup configuration")
	gwConfDir = GWCmd.PersistentFlags().String("config-dir", defaultConfDir, "main directory for setup configuration")

	CLICmd.AddCommand(setupCmd)
	GWCmd.AddCommand(setupCmd)
	cfgstruct.BindSetup(setupCmd.Flags(), &setupCfg, cfgstruct.ConfDir(defaultConfDir))
}

func cmdSetup(cmd *cobra.Command, args []string) (err error) {
	setupDir, err := filepath.Abs(*cliConfDir)
	if err != nil {
		return err
	}

	for _, flagname := range args {
		return fmt.Errorf("%s - Invalid flag. Pleas see --help", flagname)
	}

	valid, _ := fpath.IsValidSetupDir(setupDir)
	if !valid {
		return fmt.Errorf("uplink configuration already exists (%v)", setupDir)
	}

	err = os.MkdirAll(setupDir, 0700)
	if err != nil {
		return err
	}

	defaultConfDir := fpath.ApplicationDir("storj", "uplink")
	// TODO: handle setting base path *and* identity file paths via args
	// NB: if base path is set this overrides identity and CA path options
	if setupDir != defaultConfDir {
		setupCfg.CA.CertPath = filepath.Join(setupDir, "ca.cert")
		setupCfg.CA.KeyPath = filepath.Join(setupDir, "ca.key")
		setupCfg.Identity.CertPath = filepath.Join(setupDir, "identity.cert")
		setupCfg.Identity.KeyPath = filepath.Join(setupDir, "identity.key")
	}
	err = provider.SetupIdentity(process.Ctx(cmd), setupCfg.CA, setupCfg.Identity)
	if err != nil {
		return err
	}

	if setupCfg.GenerateMinioCerts {
		minioCerts := filepath.Join(setupDir, "minio", "certs")
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

	return process.SaveConfig(cmd.Flags(), filepath.Join(setupDir, "config.yaml"), o, true)
}

func generateAWSKey() (key string, err error) {
	var buf [20]byte
	_, err = rand.Read(buf[:])
	if err != nil {
		return "", err
	}
	return base58.Encode(buf[:]), nil
}
