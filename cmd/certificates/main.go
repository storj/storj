// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package main

import (
	"github.com/spf13/cobra"

	"storj.io/storj/internal/fpath"
	"storj.io/storj/pkg/certificates"
	"storj.io/storj/pkg/cfgstruct"
	"storj.io/storj/pkg/process"
	"storj.io/storj/pkg/server"
)

type batchCfg struct {
	EmailsPath string `help:"optional path to a list of emails, delimited by <delimiter>, for batch processing"`
	Delimiter  string `help:"delimiter to split emails loaded from <emails-path> on (e.g. comma, new-line)" default:"\n"`
}

var (
	rootCmd = &cobra.Command{
		Use:   "certificates",
		Short: "Certificate request signing",
	}

	runCmd = &cobra.Command{
		Use:   "run",
		Short: "Run a certificate signing server",
		RunE:  cmdRun,
	}

	runCfg struct {
		CertSigner certificates.CertServerConfig
		Server     server.Config
	}

	defaultConfDir = fpath.ApplicationDir("storj", "cert-signing")
	confDir        *string
)

func init() {
	dirParam := cfgstruct.FindConfigDirParam()
	if dirParam != "" {
		defaultConfDir = dirParam
	}
	confDir = rootCmd.PersistentFlags().String("config-dir", defaultConfDir, "main directory for captplanet configuration")

	rootCmd.AddCommand(runCmd)
	cfgstruct.Bind(runCmd.Flags(), &runCfg, cfgstruct.ConfDir(defaultConfDir))
}

func cmdRun(cmd *cobra.Command, args []string) error {
	ctx := process.Ctx(cmd)

	return runCfg.Server.Run(ctx, nil, runCfg.CertSigner)
}

func main() {
	process.Exec(rootCmd)
}
