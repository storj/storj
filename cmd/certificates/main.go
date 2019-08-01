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
		CA       identity.CASetupConfig
		Identity identity.SetupConfig
		Server   struct { // workaround server.Config change
			Identity identity.Config
			server.Config
		}
		Signer     certificates.CertServerConfig
		All        bool   `help:"print the all authorizations for auth info/export subcommands" default:"false"`
		Out        string `help:"output file path for auth export subcommand; if \"-\", will use STDOUT" default:"-"`
		ShowTokens bool   `help:"if true, token strings will be printed for auth info command" default:"false"`
		Overwrite  bool   `default:"false" help:"if true ca, identity, and authorization db will be overwritten/truncated"`
	}

	confDir     string
	identityDir string
)

func cmdRun(cmd *cobra.Command, args []string) error {
	ctx := process.Ctx(cmd)

	identity, err := config.Server.Identity.Load()
	if err != nil {
		zap.S().Fatal(err)
	}

	return config.Server.Run(ctx, zap.L(), identity, nil, config.Signer)
}

func main() {
	defaultConfDir := fpath.ApplicationDir("storj", "cert-signing")
	defaultIdentityDir := fpath.ApplicationDir("storj", "identity", "certificates")
	cfgstruct.SetupFlag(zap.L(), rootCmd, &confDir, "config-dir", defaultConfDir, "main directory for certificates configuration")
	//cfgstruct.SetupFlag(zap.L(), rootCmd, &identityDir, "identity-dir", fpath.ApplicationDir("storj", "identity", "bootstrap"), "main directory for bootstrap identity credentials")
	rootCmd.PersistentFlags().StringVar(&identityDir, "identity-dir", defaultIdentityDir, "main directory for storagenode identity credentials")
	defaults := cfgstruct.DefaultsFlag(rootCmd)

	rootCmd.AddCommand(authCmd)
	rootCmd.AddCommand(runCmd)
	rootCmd.AddCommand(setupCmd)
	rootCmd.AddCommand(signCmd)
	rootCmd.AddCommand(verifyCmd)
	rootCmd.AddCommand(claimsCmd)
	claimsCmd.AddCommand(claimsExportCmd)
	claimsCmd.AddCommand(claimDeleteCmd)
	authCmd.AddCommand(authCreateCmd)
	authCmd.AddCommand(authInfoCmd)
	authCmd.AddCommand(authExportCmd)

	process.Bind(authCreateCmd, &config, defaults, cfgstruct.ConfDir(confDir))
	process.Bind(authInfoCmd, &config, defaults, cfgstruct.ConfDir(confDir))
	process.Bind(authExportCmd, &config, defaults, cfgstruct.ConfDir(confDir))
	process.Bind(runCmd, &config, defaults, cfgstruct.ConfDir(confDir), cfgstruct.IdentityDir(identityDir))
	process.Bind(setupCmd, &config, defaults, cfgstruct.ConfDir(confDir), cfgstruct.IdentityDir(identityDir), cfgstruct.SetupMode())
	process.Bind(signCmd, &signCfg, defaults, cfgstruct.ConfDir(confDir), cfgstruct.IdentityDir(identityDir))
	process.Bind(verifyCmd, &verifyCfg, defaults, cfgstruct.ConfDir(confDir), cfgstruct.IdentityDir(identityDir))
	process.Bind(claimsExportCmd, &claimsExportCfg, defaults, cfgstruct.ConfDir(confDir))
	process.Bind(claimDeleteCmd, &claimsDeleteCfg, defaults, cfgstruct.ConfDir(confDir))

	process.Exec(rootCmd)
}
