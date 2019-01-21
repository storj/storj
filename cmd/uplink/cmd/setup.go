// Copyright (C) 2018 Storj Labs, Inc.
// See LICENSE for copying information.

package cmd

import (
	"crypto/rand"
	"fmt"
	"github.com/jbenet/go-base58"
	"github.com/spf13/cobra"
	"go.uber.org/zap"
	"os"
	"path/filepath"
	"storj.io/storj/pkg/provider"

	"storj.io/storj/internal/fpath"
	"storj.io/storj/pkg/cfgstruct"
	"storj.io/storj/pkg/miniogw"
	"storj.io/storj/pkg/process"
)

var (
	setupCmd = &cobra.Command{
		Use:         "setup",
		Short:       "Create an uplink config file",
		RunE:        cmdSetup,
		Annotations: map[string]string{"type": "setup"},
	}
	setupCfg struct {
		Identity           provider.IdentitySetupConfig
		APIKey             string `default:"" help:"the api key to use for the satellite" setup:"true"`
		EncKey             string `default:"" help:"your root encryption key" setup:"true"`
		GenerateMinioCerts bool   `default:"false" help:"generate sample TLS certs for Minio GW" setup:"true"`
		SatelliteAddr      string `default:"localhost:7778" help:"the address to use for the satellite" setup:"true"`

		Server miniogw.ServerConfig
		Minio  miniogw.MinioConfig
		Client miniogw.ClientConfig
		RS     miniogw.RSConfig
		Enc    miniogw.EncryptionConfig
	}

	defaultConfDir  = fpath.ApplicationDir("storj", "uplink")
	defaultCredsDir = fpath.ApplicationDir("storj", "identity", "uplink")

	cliConfDir string
	gwConfDir  string
	credsDir   string
)

func init() {
	confDirParam := cfgstruct.FindConfigDirParam()
	if confDirParam != "" {
		defaultConfDir = confDirParam
	}
	credsDirParam := cfgstruct.FindCredsDirParam()
	if credsDirParam != "" {
		defaultCredsDir = credsDirParam
	}

	CLICmd.PersistentFlags().StringVar(&cliConfDir, "config-dir", defaultConfDir, "main directory for setup configuration")
	GWCmd.PersistentFlags().StringVar(&gwConfDir, "config-dir", defaultConfDir, "main directory for setup configuration")
	CLICmd.PersistentFlags().StringVar(&credsDir, "creds-dir", defaultCredsDir, "main directory for uplink identity credentials")
	err := CLICmd.PersistentFlags().SetAnnotation("creds-dir", "setup", []string{"true"})
	if err != nil {
		zap.S().Error("Failed to set 'setup' annotation for 'config-dir'")
	}
	GWCmd.PersistentFlags().StringVar(&credsDir, "creds-dir", defaultCredsDir, "main directory for gateway identity credentials")
	err = GWCmd.PersistentFlags().SetAnnotation("creds-dir", "setup", []string{"true"})
	if err != nil {
		zap.S().Error("Failed to set 'setup' annotation for 'config-dir'")
	}

	CLICmd.AddCommand(setupCmd)
	GWCmd.AddCommand(setupCmd)
	cfgstruct.BindSetup(setupCmd.Flags(), &setupCfg, cfgstruct.ConfDir(defaultConfDir), cfgstruct.CredsDir(defaultCredsDir))
}

func cmdSetup(cmd *cobra.Command, args []string) (err error) {
	setupDir, err := filepath.Abs(cliConfDir)
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
		"client.api-key":         setupCfg.APIKey,
		"client.pointer-db-addr": setupCfg.SatelliteAddr,
		"client.overlay-addr":    setupCfg.SatelliteAddr,
		"minio.access-key":       accessKey,
		"minio.secret-key":       secretKey,
		"enc.key":                setupCfg.EncKey,
	}

	return process.SaveConfigWithAllDefaults(cmd.Flags(), filepath.Join(setupDir, "config.yaml"), o)
}

func generateAWSKey() (key string, err error) {
	var buf [20]byte
	_, err = rand.Read(buf[:])
	if err != nil {
		return "", err
	}
	return base58.Encode(buf[:]), nil
}
