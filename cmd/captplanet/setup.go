// Copyright (C) 2018 Storj Labs, Inc.
// See LICENSE for copying information.

package main

import (
	"fmt"
	"net"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"

	"storj.io/storj/pkg/cfgstruct"
	"storj.io/storj/pkg/process"
	"storj.io/storj/pkg/provider"
)

// Config defines broad Captain Planet configuration
type Config struct {
	HCCA                provider.CASetupConfig
	HCIdentity          provider.IdentitySetupConfig
	ULCA                provider.CASetupConfig
	ULIdentity          provider.IdentitySetupConfig
	StorageNodeCA       provider.CASetupConfig
	StorageNodeIdentity provider.IdentitySetupConfig
	BasePath            string `help:"base path for captain planet storage" default:"$CONFDIR"`
	ListenHost          string `help:"the host for providers to listen on" default:"127.0.0.1"`
	StartingPort        int    `help:"all providers will listen on ports consecutively starting with this one" default:"7777"`
	APIKey              string `default:"abc123" help:"the api key to use for the satellite"`
	EncKey              string `default:"highlydistributedridiculouslyresilient" help:"your root encryption key"`
	Overwrite           bool   `help:"whether to overwrite pre-existing configuration files" default:"false"`
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

	hcPath := filepath.Join(setupCfg.BasePath, "satellite")
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

	for i := 0; i < len(runCfg.StorageNodes); i++ {
		storagenodePath := filepath.Join(setupCfg.BasePath, fmt.Sprintf("f%d", i))
		err = os.MkdirAll(storagenodePath, 0700)
		if err != nil {
			return err
		}
		storagenodeCA := setupCfg.StorageNodeCA
		storagenodeCA.CertPath = filepath.Join(storagenodePath, "ca.cert")
		storagenodeCA.KeyPath = filepath.Join(storagenodePath, "ca.key")
		storagenodeIdentity := setupCfg.StorageNodeIdentity
		storagenodeIdentity.CertPath = filepath.Join(storagenodePath, "identity.cert")
		storagenodeIdentity.KeyPath = filepath.Join(storagenodePath, "identity.key")
		fmt.Printf("creating identity for storage node %d\n", i+1)
		err := provider.SetupIdentity(process.Ctx(cmd), storagenodeCA, storagenodeIdentity)
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

	overrides := map[string]interface{}{
		"satellite.identity.cert-path": setupCfg.HCIdentity.CertPath,
		"satellite.identity.key-path":  setupCfg.HCIdentity.KeyPath,
		"satellite.identity.address": joinHostPort(
			setupCfg.ListenHost, startingPort+1),
		"satellite.kademlia.todo-listen-addr": joinHostPort(
			setupCfg.ListenHost, startingPort+2),
		"satellite.kademlia.bootstrap-addr": joinHostPort(
			setupCfg.ListenHost, startingPort+4),
		"satellite.pointer-db.database-url": "bolt://" + filepath.Join(
			setupCfg.BasePath, "satellite", "pointerdb.db"),
		"satellite.overlay.database-url": "bolt://" + filepath.Join(
			setupCfg.BasePath, "satellite", "overlay.db"),
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
		"uplink.enc-key":          setupCfg.EncKey,
		"uplink.api-key":          setupCfg.APIKey,
		"pointer-db.auth.api-key": setupCfg.APIKey,
	}

	for i := 0; i < len(runCfg.StorageNodes); i++ {
		storagenodePath := filepath.Join(setupCfg.BasePath, fmt.Sprintf("f%d", i))
		storagenode := fmt.Sprintf("storage-nodes.%03d.", i)
		overrides[storagenode+"identity.cert-path"] = filepath.Join(
			storagenodePath, "identity.cert")
		overrides[storagenode+"identity.key-path"] = filepath.Join(
			storagenodePath, "identity.key")
		overrides[storagenode+"identity.address"] = joinHostPort(
			setupCfg.ListenHost, startingPort+i*2+3)
		overrides[storagenode+"kademlia.todo-listen-addr"] = joinHostPort(
			setupCfg.ListenHost, startingPort+i*2+4)
		overrides[storagenode+"kademlia.bootstrap-addr"] = joinHostPort(
			setupCfg.ListenHost, startingPort+1)
		overrides[storagenode+"storage.path"] = filepath.Join(storagenodePath, "data")
	}

	return process.SaveConfig(runCmd.Flags(),
		filepath.Join(setupCfg.BasePath, "config.yaml"), overrides)
}

func joinHostPort(host string, port int) string {
	return net.JoinHostPort(host, fmt.Sprint(port))
}
