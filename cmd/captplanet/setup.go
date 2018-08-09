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
	"storj.io/storj/pkg/peertls"
	"storj.io/storj/pkg/process"
)

// Config defines broad Captain Planet configuration
type Config struct {
	BasePath     string `help:"base path for captain planet storage" default:"$CONFDIR"`
	ListenHost   string `help:"the host for providers to listen on" default:"127.0.0.1"`
	StartingPort int    `help:"all providers will listen on ports consecutively starting with this one" default:"7777"`
	Overwrite    bool   `help:"whether to overwrite pre-existing configuration files" default:"false"`
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
	identPath := filepath.Join(hcPath, "ident")
	_, err = peertls.NewTLSFileOptions(identPath, identPath, true, true)
	if err != nil {
		return err
	}

	for i := 0; i < len(runCfg.Farmers); i++ {
		farmerPath := filepath.Join(setupCfg.BasePath, fmt.Sprintf("f%d", i))
		err = os.MkdirAll(farmerPath, 0700)
		if err != nil {
			return err
		}
		identPath = filepath.Join(farmerPath, "ident")
		_, err = peertls.NewTLSFileOptions(identPath, identPath, true, true)
		if err != nil {
			return err
		}
	}

	gwPath := filepath.Join(setupCfg.BasePath, "gw")
	err = os.MkdirAll(gwPath, 0700)
	if err != nil {
		return err
	}
	identPath = filepath.Join(gwPath, "ident")
	_, err = peertls.NewTLSFileOptions(identPath, identPath, true, true)
	if err != nil {
		return err
	}

	startingPort := setupCfg.StartingPort

	apiKey, err := newAPIKey()
	if err != nil {
		return err
	}

	overrides := map[string]interface{}{
		"heavy-client.identity.cert-path": filepath.Join(
			setupCfg.BasePath, "hc", "ident.leaf.cert"),
		"heavy-client.identity.key-path": filepath.Join(
			setupCfg.BasePath, "hc", "ident.leaf.key"),
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
		"gateway.cert-path": filepath.Join(
			setupCfg.BasePath, "gw", "ident.leaf.cert"),
		"gateway.key-path": filepath.Join(
			setupCfg.BasePath, "gw", "ident.leaf.key"),
		"gateway.address": joinHostPort(
			setupCfg.ListenHost, startingPort),
		"gateway.overlay-addr": joinHostPort(
			setupCfg.ListenHost, startingPort+1),
		"gateway.pointer-db-addr": joinHostPort(
			setupCfg.ListenHost, startingPort+1),
		"gateway.minio-dir": filepath.Join(
			setupCfg.BasePath, "gw", "minio"),
		"gateway.api-key":         apiKey,
		"pointer-db.auth.api-key": apiKey,
	}

	for i := 0; i < len(runCfg.Farmers); i++ {
		basepath := filepath.Join(setupCfg.BasePath, fmt.Sprintf("f%d", i))
		farmer := fmt.Sprintf("farmers.%02d.", i)
		overrides[farmer+"identity.cert-path"] = filepath.Join(
			basepath, "ident.leaf.cert")
		overrides[farmer+"identity.key-path"] = filepath.Join(
			basepath, "ident.leaf.key")
		overrides[farmer+"identity.address"] = joinHostPort(
			setupCfg.ListenHost, startingPort+i*2+3)
		overrides[farmer+"kademlia.todo-listen-addr"] = joinHostPort(
			setupCfg.ListenHost, startingPort+i*2+4)
		overrides[farmer+"kademlia.bootstrap-addr"] = joinHostPort(
			setupCfg.ListenHost, startingPort+1)
		overrides[farmer+"storage.path"] = filepath.Join(basepath, "data")
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
