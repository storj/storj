// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package main

import (
	"github.com/spf13/cobra"
	"github.com/zeebo/errs"
	"go.uber.org/zap"

	"storj.io/common/fpath"
	"storj.io/storj/certificate"
	"storj.io/storj/certificate/authorization"
	"storj.io/storj/pkg/cfgstruct"
	"storj.io/storj/pkg/process"
	"storj.io/storj/pkg/revocation"
)

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

	runCfg certificate.Config

	setupCfg struct {
		Overwrite bool `help:"if true ca, identity, and authorization db will be overwritten/truncated" default:"false"`
		certificate.Config
	}

	authCfg struct {
		All        bool   `help:"print the all authorizations for auth info/export subcommands" default:"false"`
		Out        string `help:"output file path for auth export subcommand; if \"-\", will use STDOUT" default:"-"`
		ShowTokens bool   `help:"if true, token strings will be printed for auth info command" default:"false"`
		EmailsPath string `help:"optional path to a list of emails, delimited by <delimiter>, for batch processing"`
		Delimiter  string `help:"delimiter to split emails loaded from <emails-path> on (e.g. comma, new-line)" default:"\n"`

		certificate.Config
	}

	claimsExportCfg struct {
		Raw bool `default:"false" help:"if true, the raw data structures will be printed"`
		certificate.Config
	}

	claimsDeleteCfg certificate.Config

	confDir     string
	identityDir string
)

func cmdRun(cmd *cobra.Command, args []string) error {
	ctx, _ := process.Ctx(cmd)

	identity, err := runCfg.Identity.Load()
	if err != nil {
		return err
	}

	signer, err := runCfg.Signer.Load()
	if err != nil {
		return err
	}

	authorizationDB, err := authorization.NewDBFromCfg(runCfg.AuthorizationDB)
	if err != nil {
		return errs.New("error opening authorizations database: %+v", err)
	}

	revocationDB, err := revocation.NewDBFromCfg(runCfg.Server.Config)
	if err != nil {
		return errs.New("error creating revocation database: %+v", err)
	}
	defer func() {
		err = errs.Combine(err, revocationDB.Close())
	}()

	peer, err := certificate.New(zap.L(), identity, signer, authorizationDB, revocationDB, &runCfg)
	if err != nil {
		return err
	}
	return peer.Run(ctx)
}

func main() {
	defaultConfDir := fpath.ApplicationDir("storj", "cert-signing")
	defaultIdentityDir := fpath.ApplicationDir("storj", "identity", "certificates")
	cfgstruct.SetupFlag(zap.L(), rootCmd, &confDir, "config-dir", defaultConfDir, "main directory for certificates configuration")
	cfgstruct.SetupFlag(zap.L(), rootCmd, &identityDir, "identity-dir", defaultIdentityDir, "main directory for identity credentials")
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

	process.Bind(authCreateCmd, &authCfg, defaults, cfgstruct.ConfDir(confDir))
	process.Bind(authInfoCmd, &authCfg, defaults, cfgstruct.ConfDir(confDir))
	process.Bind(authExportCmd, &authCfg, defaults, cfgstruct.ConfDir(confDir))
	process.Bind(runCmd, &runCfg, defaults, cfgstruct.ConfDir(confDir), cfgstruct.IdentityDir(identityDir))
	process.Bind(setupCmd, &setupCfg, defaults, cfgstruct.ConfDir(confDir), cfgstruct.IdentityDir(identityDir), cfgstruct.SetupMode())
	process.Bind(signCmd, &signCfg, defaults, cfgstruct.ConfDir(confDir), cfgstruct.IdentityDir(identityDir))
	process.Bind(verifyCmd, &verifyCfg, defaults, cfgstruct.ConfDir(confDir), cfgstruct.IdentityDir(identityDir))
	process.Bind(claimsExportCmd, &claimsExportCfg, defaults, cfgstruct.ConfDir(confDir))
	process.Bind(claimDeleteCmd, &claimsDeleteCfg, defaults, cfgstruct.ConfDir(confDir))

	process.Exec(rootCmd)
}
