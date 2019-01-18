// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package main

import (
	"github.com/spf13/cobra"
	"go.uber.org/zap"

	"storj.io/storj/internal/fpath"
	"storj.io/storj/pkg/certificates"
	"storj.io/storj/pkg/cfgstruct"
	"storj.io/storj/pkg/identity"
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

	config struct {
		batchCfg
		CA         identity.CASetupConfig
		Identity   identity.SetupConfig
		Server     server.Config
		Signer     certificates.CertServerConfig
		All        bool   `help:"print the all authorizations for auth info/export subcommands" default:"false"`
		Out        string `help:"output file path for auth export subcommand; if \"-\", will use STDOUT" default:"-"`
		ShowTokens bool   `help:"if true, token strings will be printed for auth info command" default:"false"`
		Overwrite  bool   `default:"false" help:"if true ca, identity, and authorization db will be overwritten/truncated"`
	}

	defaultConfDir  = fpath.ApplicationDir("storj", "cert-signing")
	defaultCredsDir = fpath.ApplicationDir("storj", "identity", "certificates")
	confDir         string
	credsDir        string
)

func init() {
	confDirParam := cfgstruct.FindConfigDirParam()
	if confDirParam != "" {
		defaultCredsDir = confDirParam
	}
	credsDirParam := cfgstruct.FindCredsDirParam()
	if credsDirParam != "" {
		defaultCredsDir = credsDirParam
	}

	rootCmd.PersistentFlags().StringVar(&confDir, "config-dir", defaultConfDir, "main directory for certificates configuration")
	err := rootCmd.PersistentFlags().SetAnnotation("config-dir", "setup", []string{"true"})
	if err != nil {
		zap.S().Error("Failed to set 'setup' annotation for 'config-dir'")
	}
	rootCmd.PersistentFlags().StringVar(&credsDir, "creds-dir", defaultCredsDir, "main directory for storagenode identity credentials")

	rootCmd.AddCommand(runCmd)
	cfgstruct.Bind(runCmd.Flags(), &config, cfgstruct.ConfDir(defaultConfDir), cfgstruct.CredsDir(defaultCredsDir))
}

func cmdRun(cmd *cobra.Command, args []string) error {
	ctx := process.Ctx(cmd)

	return config.Server.Run(ctx, nil, config.Signer)
}

func main() {
	process.Exec(rootCmd)
}
