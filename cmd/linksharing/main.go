// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package main

import (
	"crypto/tls"
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"
	"github.com/zeebo/errs"
	"go.uber.org/zap"

	"storj.io/common/fpath"
	"storj.io/storj/lib/uplink"
	"storj.io/storj/linksharing"
	"storj.io/storj/linksharing/httpserver"
	"storj.io/storj/pkg/cfgstruct"
	"storj.io/storj/pkg/process"
)

// LinkSharing defines link sharing configuration
type LinkSharing struct {
	Address   string `user:"true" help:"public address to listen on" devDefault:"localhost:8080" releaseDefault:":8443"`
	CertFile  string `user:"true" help:"server certificate file" devDefault:"" releaseDefault:"server.crt.pem"`
	KeyFile   string `user:"true" help:"server key file" devDefault:"" releaseDefault:"server.key.pem"`
	PublicURL string `user:"true" help:"public url for the server" devDefault:"http://localhost:8080" releaseDefault:""`
}

var (
	rootCmd = &cobra.Command{
		Use:   "link sharing service",
		Short: "Link Sharing Service",
	}
	runCmd = &cobra.Command{
		Use:   "run",
		Short: "Run the link sharing service",
		RunE:  cmdRun,
	}
	setupCmd = &cobra.Command{
		Use:         "setup",
		Short:       "Create config files",
		RunE:        cmdSetup,
		Annotations: map[string]string{"type": "setup"},
	}

	runCfg   LinkSharing
	setupCfg LinkSharing

	confDir string
)

func init() {
	defaultConfDir := fpath.ApplicationDir("storj", "linksharing")
	cfgstruct.SetupFlag(zap.L(), rootCmd, &confDir, "config-dir", defaultConfDir, "main directory for link sharing configuration")
	defaults := cfgstruct.DefaultsFlag(rootCmd)
	rootCmd.AddCommand(runCmd)
	rootCmd.AddCommand(setupCmd)
	process.Bind(runCmd, &runCfg, defaults, cfgstruct.ConfDir(confDir))
	process.Bind(setupCmd, &setupCfg, defaults, cfgstruct.ConfDir(confDir), cfgstruct.SetupMode())
}

func cmdRun(cmd *cobra.Command, args []string) (err error) {
	ctx, _ := process.Ctx(cmd)
	log := zap.L()

	uplink, err := uplink.NewUplink(ctx, nil)
	if err != nil {
		return err
	}

	tlsConfig, err := configureTLS(runCfg.CertFile, runCfg.KeyFile)
	if err != nil {
		return err
	}

	handler, err := linksharing.NewHandler(log, linksharing.HandlerConfig{
		Uplink:  uplink,
		URLBase: runCfg.PublicURL,
	})
	if err != nil {
		return err
	}

	server, err := httpserver.New(log, httpserver.Config{
		Name:            "Link Sharing",
		Address:         runCfg.Address,
		Handler:         handler,
		TLSConfig:       tlsConfig,
		ShutdownTimeout: -1,
	})
	if err != nil {
		return err
	}

	return server.Run(ctx)
}

func cmdSetup(cmd *cobra.Command, args []string) (err error) {
	setupDir, err := filepath.Abs(confDir)
	if err != nil {
		return err
	}

	valid, _ := fpath.IsValidSetupDir(setupDir)
	if !valid {
		return fmt.Errorf("link sharing configuration already exists (%v)", setupDir)
	}

	err = os.MkdirAll(setupDir, 0700)
	if err != nil {
		return err
	}

	return process.SaveConfig(cmd, filepath.Join(setupDir, "config.yaml"))
}

func configureTLS(certFile, keyFile string) (*tls.Config, error) {
	switch {
	case certFile != "" && keyFile != "":
	case certFile == "" && keyFile == "":
		return nil, nil
	case certFile != "" && keyFile == "":
		return nil, errs.New("key file must be provided with cert file")
	case certFile == "" && keyFile != "":
		return nil, errs.New("cert file must be provided with key file")
	}

	cert, err := tls.LoadX509KeyPair(certFile, keyFile)
	if err != nil {
		return nil, errs.New("unable to load server keypair: %v", err)
	}

	return &tls.Config{
		Certificates: []tls.Certificate{cert},
	}, nil
}

func main() {
	process.Exec(rootCmd)
}
