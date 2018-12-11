// Copyright (C) 2018 Storj Labs, Inc.
// See LICENSE for copying information.

package main

import (
	"fmt"
	"net"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"

	"storj.io/storj/internal/fpath"
	"storj.io/storj/pkg/cfgstruct"
	"storj.io/storj/pkg/process"
	"storj.io/storj/pkg/provider"
)

// Config defines broad Captain Planet configuration
type Config struct {
	SatelliteCA         provider.CASetupConfig
	SatelliteIdentity   provider.IdentitySetupConfig
	UplinkCA            provider.CASetupConfig
	UplinkIdentity      provider.IdentitySetupConfig
	StorageNodeCA       provider.CASetupConfig
	StorageNodeIdentity provider.IdentitySetupConfig
	BasePath            string `help:"base path for captain planet storage" default:"$CONFDIR"`
	ListenHost          string `help:"the host for providers to listen on" default:"127.0.0.1"`
	StartingPort        int    `help:"all providers will listen on ports consecutively starting with this one" default:"7777"`
	APIKey              string `default:"abc123" help:"the api key to use for the satellite"`
	EncKey              string `default:"insecure-default-encryption-key" help:"your root encryption key"`
	Overwrite           bool   `help:"whether to overwrite pre-existing configuration files" default:"false"`
	GenerateMinioCerts  bool   `default:"false" help:"generate sample TLS certs for Minio GW"`
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

	valid, err := fpath.IsValidSetupDir(setupCfg.BasePath)
	if !setupCfg.Overwrite && !valid {
		fmt.Printf("captplanet configuration already exists (%v). rerun with --overwrite\n", setupCfg.BasePath)
		return nil
	} else if setupCfg.Overwrite && err == nil {
		fmt.Println("overwriting existing captplanet config")
		err = os.RemoveAll(setupCfg.BasePath)
		if err != nil {
			return err
		}
	}

	satellitePath := filepath.Join(setupCfg.BasePath, "satellite")
	err = os.MkdirAll(satellitePath, 0700)
	if err != nil {
		return err
	}
	setupCfg.SatelliteCA.CertPath = filepath.Join(satellitePath, "ca.cert")
	setupCfg.SatelliteCA.KeyPath = filepath.Join(satellitePath, "ca.key")
	setupCfg.SatelliteIdentity.CertPath = filepath.Join(satellitePath, "identity.cert")
	setupCfg.SatelliteIdentity.KeyPath = filepath.Join(satellitePath, "identity.key")
	fmt.Printf("creating identity for satellite\n")
	err = provider.SetupIdentity(process.Ctx(cmd), setupCfg.SatelliteCA, setupCfg.SatelliteIdentity)
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
	setupCfg.UplinkCA.CertPath = filepath.Join(uplinkPath, "ca.cert")
	setupCfg.UplinkCA.KeyPath = filepath.Join(uplinkPath, "ca.key")
	setupCfg.UplinkIdentity.CertPath = filepath.Join(uplinkPath, "identity.cert")
	setupCfg.UplinkIdentity.KeyPath = filepath.Join(uplinkPath, "identity.key")
	fmt.Printf("creating identity for uplink\n")
	err = provider.SetupIdentity(process.Ctx(cmd), setupCfg.UplinkCA, setupCfg.UplinkIdentity)
	if err != nil {
		return err
	}

	if setupCfg.GenerateMinioCerts {
		minioCertsPath := filepath.Join(uplinkPath, "minio", "certs")
		if err := os.MkdirAll(minioCertsPath, 0744); err != nil {
			return err
		}
		if err := os.Link(setupCfg.UplinkIdentity.CertPath, filepath.Join(minioCertsPath, "public.crt")); err != nil {
			return err
		}
		if err := os.Link(setupCfg.UplinkIdentity.KeyPath, filepath.Join(minioCertsPath, "private.key")); err != nil {
			return err
		}
	}

	startingPort := setupCfg.StartingPort

	overlayAddr := joinHostPort(setupCfg.ListenHost, startingPort+1)

	overrides := map[string]interface{}{
		"satellite.identity.cert-path": setupCfg.SatelliteIdentity.CertPath,
		"satellite.identity.key-path":  setupCfg.SatelliteIdentity.KeyPath,
		"satellite.identity.server.address": joinHostPort(
			setupCfg.ListenHost, startingPort+1),
		"satellite.kademlia.bootstrap-addr": joinHostPort(
			setupCfg.ListenHost, startingPort+1),
		"satellite.pointer-db.database-url": "bolt://" + filepath.Join(
			setupCfg.BasePath, "satellite", "pointerdb.db"),
		"satellite.overlay.database-url": "bolt://" + filepath.Join(
			setupCfg.BasePath, "satellite", "overlay.db"),
		"satellite.kademlia.alpha":         3,
		"satellite.repairer.queue-address": "redis://127.0.0.1:6378?db=1&password=abc123",
		"satellite.repairer.overlay-addr":  overlayAddr,
		"satellite.repairer.pointer-db-addr": joinHostPort(
			setupCfg.ListenHost, startingPort+1),
		"satellite.repairer.api-key": setupCfg.APIKey,
		"uplink.identity.cert-path":  setupCfg.UplinkIdentity.CertPath,
		"uplink.identity.key-path":   setupCfg.UplinkIdentity.KeyPath,
		"uplink.identity.server.address": joinHostPort(
			setupCfg.ListenHost, startingPort),
		"uplink.client.overlay-addr": joinHostPort(
			setupCfg.ListenHost, startingPort+1),
		"uplink.client.pointer-db-addr": joinHostPort(
			setupCfg.ListenHost, startingPort+1),
		"uplink.minio.dir": filepath.Join(
			setupCfg.BasePath, "uplink", "minio"),
		"uplink.enc.key":                  setupCfg.EncKey,
		"uplink.client.api-key":           setupCfg.APIKey,
		"uplink.rs.min-threshold":         1 * len(runCfg.StorageNodes) / 5,
		"uplink.rs.repair-threshold":      2 * len(runCfg.StorageNodes) / 5,
		"uplink.rs.success-threshold":     3 * len(runCfg.StorageNodes) / 5,
		"uplink.rs.max-threshold":         4 * len(runCfg.StorageNodes) / 5,
		"kademlia.bucket-size":            4,
		"kademlia.replacement-cache-size": 1,

		// TODO: this will eventually go away
		"pointer-db.auth.api-key": setupCfg.APIKey,

		// TODO: this is a source of bugs. this value should be pulled from
		// kademlia instead
		"piecestore.agreementsender.overlay_addr": overlayAddr,

		"log.development": true,
		"log.level":       "debug",
	}

	for i := 0; i < len(runCfg.StorageNodes); i++ {
		storagenodePath := filepath.Join(setupCfg.BasePath, fmt.Sprintf("f%d", i))
		storagenode := fmt.Sprintf("storage-nodes.%02d.", i)
		overrides[storagenode+"identity.cert-path"] = filepath.Join(
			storagenodePath, "identity.cert")
		overrides[storagenode+"identity.key-path"] = filepath.Join(
			storagenodePath, "identity.key")
		overrides[storagenode+"identity.server.address"] = joinHostPort(
			setupCfg.ListenHost, startingPort+i*2+3)
		overrides[storagenode+"kademlia.bootstrap-addr"] = joinHostPort(
			setupCfg.ListenHost, startingPort+1)
		overrides[storagenode+"storage.path"] = filepath.Join(storagenodePath, "data")
		overrides[storagenode+"kademlia.alpha"] = 3
	}

	return process.SaveConfig(runCmd.Flags(),
		filepath.Join(setupCfg.BasePath, "config.yaml"), overrides)
}

func joinHostPort(host string, port int) string {
	return net.JoinHostPort(host, fmt.Sprint(port))
}
