// Copyright (C) 2018 Storj Labs, Inc.
// See LICENSE for copying information.

package main

import (
	"context"
	"crypto/rand"
	"fmt"
	"net"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"

	"github.com/mr-tron/base58/base58"

	"storj.io/storj/internal/fpath"
	"storj.io/storj/pkg/cfgstruct"
	"storj.io/storj/pkg/miniogw"
	"storj.io/storj/pkg/process"
	"storj.io/storj/pkg/provider"
	"storj.io/storj/pkg/storage/streams"
	"storj.io/storj/pkg/storj"
)

// Config is miniogw.Config configuration
type Config struct {
	miniogw.Config
}

type Gateway struct {
	CA                 provider.CASetupConfig       `setup:"true"`
	Identity           provider.IdentitySetupConfig `setup:"true"`
	Overwrite          bool                         `default:"false" help:"whether to overwrite pre-existing configuration files" setup:"true"`
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

var cfg Config

var (
	// GWCmd represents the base gateway command when called without any subcommands
	GWCmd = &cobra.Command{
		Use:   "gateway",
		Short: "The Storj client-side S3 gateway",
	}
	setupCmd = &cobra.Command{
		Use:         "setup",
		Short:       "Create an gateway config file",
		RunE:        cmdSetup,
		Annotations: map[string]string{"type": "setup"},
	}
	runCmd = &cobra.Command{
		Use:   "run",
		Short: "Run the S3 gateway",
		RunE:  cmdRun,
	}

	setupCfg Gateway
	runCfg   Gateway

	gwConfDir *string
)

func init() {
	defaultConfDir := fpath.ApplicationDir("storj", "gateway")

	dirParam := cfgstruct.FindConfigDirParam()
	if dirParam != "" {
		defaultConfDir = dirParam
	}

	gwConfDir = GWCmd.PersistentFlags().String("config-dir", defaultConfDir, "main directory for setup configuration")

	GWCmd.AddCommand(runCmd)
	GWCmd.AddCommand(setupCmd)
	cfgstruct.Bind(runCmd.Flags(), &runCfg, cfgstruct.ConfDir(defaultConfDir))
	cfgstruct.BindSetup(setupCmd.Flags(), &setupCfg, cfgstruct.ConfDir(defaultConfDir))
}

func cmdSetup(cmd *cobra.Command, args []string) (err error) {
	setupDir, err := filepath.Abs(*gwConfDir)
	if err != nil {
		return err
	}

	for _, flagname := range args {
		return fmt.Errorf("%s - Invalid flag. Pleas see --help", flagname)
	}

	valid, _ := fpath.IsValidSetupDir(setupDir)
	if !setupCfg.Overwrite && !valid {
		return fmt.Errorf("%s configuration already exists (%v). Rerun with --overwrite", "gateway", setupDir)
	}

	err = os.MkdirAll(setupDir, 0700)
	if err != nil {
		return err
	}

	defaultConfDir := fpath.ApplicationDir("storj", "gateway")
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

	return process.SaveConfig(cmd.Flags(), filepath.Join(setupDir, "config.yaml"), o)
}

func cmdRun(cmd *cobra.Command, args []string) (err error) {
	for _, flagname := range args {
		return fmt.Errorf("Invalid argument %#v. Try 'uplink run'", flagname)
	}

	address := runCfg.Server.Address
	host, port, err := net.SplitHostPort(address)
	if err != nil {
		return err
	}
	if host == "" {
		address = net.JoinHostPort("localhost", port)
	}

	fmt.Printf("Starting Storj S3-compatible gateway!\n\n")
	fmt.Printf("Endpoint: %s\n", address)
	fmt.Printf("Access key: %s\n", cfg.Minio.AccessKey)
	fmt.Printf("Secret key: %s\n", cfg.Minio.SecretKey)

	ctx := process.Ctx(cmd)
	metainfo, _, err := cfg.Metainfo(ctx)
	if err != nil {
		return err
	}

	_, err = metainfo.ListBuckets(ctx, storj.BucketListOptions{Direction: storj.After})
	if err != nil {
		return fmt.Errorf("Failed to contact Satellite.\n"+
			"Perhaps your configuration is invalid?\n%s", err)
	}

	return runCfg.Server.Run(process.Ctx(cmd))
}

func generateAWSKey() (key string, err error) {
	var buf [20]byte
	_, err = rand.Read(buf[:])
	if err != nil {
		return "", err
	}
	return base58.Encode(buf[:]), nil
}

// Metainfo loads the storj.Metainfo
//
// Temporarily it also returns an instance of streams.Store until we improve
// the metainfo and streas implementations.
func (c *Config) Metainfo(ctx context.Context) (storj.Metainfo, streams.Store, error) {
	identity, err := c.Identity.Load()
	if err != nil {
		return nil, nil, err
	}

	return c.GetMetainfo(ctx, identity)
}

func main() {
	process.Exec(GWCmd)
}
