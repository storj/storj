// Copyright (C) 2018 Storj Labs, Inc.
// See LICENSE for copying information.

package main

import (
	"crypto/rand"
	"fmt"
	"net"
	"os"
	"path/filepath"

	base58 "github.com/jbenet/go-base58"
	"github.com/spf13/cobra"

	"storj.io/storj/pkg/cfgstruct"
	"storj.io/storj/pkg/process"
	"storj.io/storj/pkg/provider"
)

// Config defines broad Captain Planet configuration
type Config struct {
	HCCA           provider.CASetupConfig
	HCIdentity     provider.IdentitySetupConfig
	ULCA           provider.CASetupConfig
	ULIdentity     provider.IdentitySetupConfig
	FarmerCA       provider.CASetupConfig
	FarmerIdentity provider.IdentitySetupConfig
	BasePath       string `help:"base path for captain planet storage" default:"$CONFDIR"`
	ListenHost     string `help:"the host for providers to listen on" default:"127.0.0.1"`
	StartingPort   int    `help:"all providers will listen on ports consecutively starting with this one" default:"7777"`
	Overwrite      bool   `help:"whether to overwrite pre-existing configuration files" default:"false"`
}

var (
	setupCmd = &cobra.Command{
		Use:   "setup",
		Short: "Set up configurations",
		RunE:  cmdSetup,
	}
	setupCfg Config
)

func init() {
	rootCmd.AddCommand(setupCmd)
	cfgstruct.Bind(setupCmd.Flags(), &setupCfg,
		cfgstruct.ConfDir(defaultConfDir),
	)
}

func cmdSetup(cmd *cobra.Command, args []string) (err error) {
	setupCfg.BasePath, err = filepath.Abs(setupCfg.BasePath)
	if err != nil {
		return err
	}

	_, err = os.Stat(setupCfg.BasePath)
	if !setupCfg.Overwrite && err == nil {
		fmt.Println("A captplanet configuration already exists. Rerun with --overwrite")
		return nil
	}

	hcPath := filepath.Join(setupCfg.BasePath, "hc")
	err = os.MkdirAll(hcPath, 0700)
	if err != nil {
		return err
	}
	setupCfg.HCCA.CertPath = filepath.Join(hcPath, "ca.cert")
	setupCfg.HCCA.KeyPath = filepath.Join(hcPath, "ca.key")
	setupCfg.HCIdentity.CertPath = filepath.Join(hcPath, "identity.cert")
	setupCfg.HCIdentity.KeyPath = filepath.Join(hcPath, "identity.key")
	fmt.Printf("creating identity for satellite\n")
	err = provider.SetupIdentity(process.Ctx(cmd), setupCfg.HCCA, setupCfg.HCIdentity)
	if err != nil {
		return err
	}

	for i := 0; i < len(runCfg.Farmers); i++ {
		farmerPath := filepath.Join(setupCfg.BasePath, fmt.Sprintf("f%d", i))
		err = os.MkdirAll(farmerPath, 0700)
		if err != nil {
			return err
		}
		farmerCA := setupCfg.FarmerCA
		farmerCA.CertPath = filepath.Join(farmerPath, "ca.cert")
		farmerCA.KeyPath = filepath.Join(farmerPath, "ca.key")
		farmerIdentity := setupCfg.FarmerIdentity
		farmerIdentity.CertPath = filepath.Join(farmerPath, "identity.cert")
		farmerIdentity.KeyPath = filepath.Join(farmerPath, "identity.key")
		fmt.Printf("creating identity for storage node %d\n", i+1)
		err := provider.SetupIdentity(process.Ctx(cmd), farmerCA, farmerIdentity)
		if err != nil {
			return err
		}
	}

	uplinkPath := filepath.Join(setupCfg.BasePath, "uplink")
	err = os.MkdirAll(uplinkPath, 0700)
	if err != nil {
		return err
	}
	setupCfg.ULCA.CertPath = filepath.Join(uplinkPath, "ca.cert")
	setupCfg.ULCA.KeyPath = filepath.Join(uplinkPath, "ca.key")
	setupCfg.ULIdentity.CertPath = filepath.Join(uplinkPath, "identity.cert")
	setupCfg.ULIdentity.KeyPath = filepath.Join(uplinkPath, "identity.key")
	fmt.Printf("creating identity for uplink\n")
	err = provider.SetupIdentity(process.Ctx(cmd), setupCfg.ULCA, setupCfg.ULIdentity)
	if err != nil {
		return err
	}

	startingPort := setupCfg.StartingPort

	apiKey, err := newAPIKey()
	if err != nil {
		return err
	}

	overrides := map[string]interface{}{
		"heavy-client.identity.cert-path": setupCfg.HCIdentity.CertPath,
		"heavy-client.identity.key-path":  setupCfg.HCIdentity.KeyPath,
		"heavy-client.identity.address": joinHostPort(
			setupCfg.ListenHost, startingPort+1),
		"heavy-client.kademlia.todo-listen-addr": joinHostPort(
			setupCfg.ListenHost, startingPort+2),
		"heavy-client.kademlia.bootstrap-addr": joinHostPort(
			setupCfg.ListenHost, startingPort+4),
		"heavy-client.pointer-db.database-url": "bolt://" + filepath.Join(
			setupCfg.BasePath, "hc", "pointerdb.db"),
		"heavy-client.overlay.database-url": "bolt://" + filepath.Join(
			setupCfg.BasePath, "hc", "overlay.db"),
		"uplink.cert-path": setupCfg.ULIdentity.CertPath,
		"uplink.key-path":  setupCfg.ULIdentity.KeyPath,
		"uplink.address": joinHostPort(
			setupCfg.ListenHost, startingPort),
		"uplink.overlay-addr": joinHostPort(
			setupCfg.ListenHost, startingPort+1),
		"uplink.pointer-db-addr": joinHostPort(
			setupCfg.ListenHost, startingPort+1),
		"uplink.minio-dir": filepath.Join(
			setupCfg.BasePath, "uplink", "minio"),
		"uplink.api-key":          apiKey,
		"pointer-db.auth.api-key": apiKey,
	}

	for i := 0; i < len(runCfg.Farmers); i++ {
		farmerPath := filepath.Join(setupCfg.BasePath, fmt.Sprintf("f%d", i))
		farmer := fmt.Sprintf("farmers.%02d.", i)
		overrides[farmer+"identity.cert-path"] = filepath.Join(
			farmerPath, "identity.cert")
		overrides[farmer+"identity.key-path"] = filepath.Join(
			farmerPath, "identity.key")
		overrides[farmer+"identity.address"] = joinHostPort(
			setupCfg.ListenHost, startingPort+i*2+3)
		overrides[farmer+"kademlia.todo-listen-addr"] = joinHostPort(
			setupCfg.ListenHost, startingPort+i*2+4)
		overrides[farmer+"kademlia.bootstrap-addr"] = joinHostPort(
			setupCfg.ListenHost, startingPort+1)
		overrides[farmer+"storage.path"] = filepath.Join(farmerPath, "data")
	}

	return process.SaveConfig(runCmd.Flags(),
		filepath.Join(setupCfg.BasePath, "config.yaml"), overrides)
}

func joinHostPort(host string, port int) string {
	return net.JoinHostPort(host, fmt.Sprint(port))
}

func newAPIKey() (string, error) {
	var buf [20]byte
	_, err := rand.Read(buf[:])
	if err != nil {
		return "", err
	}
	return base58.Encode(buf[:]), nil
}
