// Copyright (C) 2018 Storj Labs, Inc.
// See LICENSE for copying information.

package main

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"

	"storj.io/storj/pkg/cfgstruct"
	"storj.io/storj/pkg/miniogw"
	"storj.io/storj/pkg/peertls"
	"storj.io/storj/pkg/process"
)

// Config defines broad Captain Planet configuration
type Config struct {
	FarmerCount  int    `help:"number of farmers to run" default:"20"`
	BasePath     string `help:"base path for captain planet storage" default:"$CONFDIR"`
	ListenHost   string `help:"the host for providers to listen on" default:"127.0.0.1"`
	StartingPort int    `help:"all providers will listen on ports consecutively starting with this one" default:"7777"`
	miniogw.RSConfig
	miniogw.MinioConfig
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
	hcPath := filepath.Join(setupCfg.BasePath, "hc")
	err = os.MkdirAll(hcPath, 0700)
	if err != nil {
		return err
	}
	identPath := filepath.Join(hcPath, "ident")
	_, err = peertls.NewTLSFileOptions(identPath, identPath, true, false)
	if err != nil {
		return err
	}

	for i := 0; i < setupCfg.FarmerCount; i++ {
		farmerPath := filepath.Join(setupCfg.BasePath, fmt.Sprintf("f%d", i))
		err = os.MkdirAll(farmerPath, 0700)
		if err != nil {
			return err
		}
		identPath = filepath.Join(farmerPath, "ident")
		_, err = peertls.NewTLSFileOptions(identPath, identPath, true, false)
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
	_, err = peertls.NewTLSFileOptions(identPath, identPath, true, false)
	if err != nil {
		return err
	}

	return process.SaveConfig(cmd)
}
