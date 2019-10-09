// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package main

import (
	"context"
	"fmt"
	"net"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"
	"github.com/wthorp/ftpserver-zap/server"
	"github.com/zeebo/errs"
	"go.uber.org/zap"

	"storj.io/storj/cmd/internal/wizard"
	"storj.io/storj/internal/fpath"
	"storj.io/storj/internal/version"
	libuplink "storj.io/storj/lib/uplink"
	"storj.io/storj/pkg/cfgstruct"
	"storj.io/storj/pkg/ftp"
	"storj.io/storj/pkg/process"
	"storj.io/storj/pkg/storj"
	"storj.io/storj/uplink"
)

// FTPFlags configuration flags
type FTPFlags struct {
	NonInteractive bool `help:"disable interactive mode" default:"false" setup:"true"`

	Server ftp.ServerConfig
	uplink.Config
	Version version.Config
}

var (
	// Error is the default gateway setup errs class
	Error = errs.Class("FTP setup error")
	// rootCmd represents the base gateway command when called without any subcommands
	rootCmd = &cobra.Command{
		Use:   "FTP",
		Short: "The Storj client-side FTP gateway",
		Args:  cobra.OnlyValidArgs,
	}
	setupCmd = &cobra.Command{
		Use:         "setup",
		Short:       "Create a gateway config file",
		RunE:        cmdSetup,
		Annotations: map[string]string{"type": "setup"},
	}
	runCmd = &cobra.Command{
		Use:   "run",
		Short: "Run the FTP gateway",
		RunE:  cmdRun,
	}

	confDir     string
	identityDir string
	config      FTPFlags
	ftpServer   *server.FtpServer
	logger      *zap.Logger
)

func main() {
	logger, _ := zap.NewProduction()
	defaultConfDir := fpath.ApplicationDir("storj", "ftp")
	defaultIdentityDir := fpath.ApplicationDir("storj", "identity", "gateway")
	cfgstruct.SetupFlag(logger, rootCmd, &confDir, "config-dir", defaultConfDir, "main directory for FTP configuration")
	cfgstruct.SetupFlag(logger, rootCmd, &identityDir, "identity-dir", defaultIdentityDir, "main directory for gateway identity credentials")
	defaults := cfgstruct.DefaultsFlag(rootCmd)
	rootCmd.AddCommand(runCmd)
	rootCmd.AddCommand(setupCmd)
	process.Bind(runCmd, &config, defaults, cfgstruct.ConfDir(confDir), cfgstruct.IdentityDir(identityDir))
	process.Bind(setupCmd, &config, defaults, cfgstruct.ConfDir(confDir), cfgstruct.IdentityDir(identityDir), cfgstruct.SetupMode())
	process.Exec(rootCmd)
}

///////////////////////////////////////////////////////////////////////////////
//                             RUN RELATED STUFF                             //
///////////////////////////////////////////////////////////////////////////////

func cmdRun(cmd *cobra.Command, args []string) (err error) {
	address := config.Server.Address
	host, port, err := net.SplitHostPort(address)
	if err != nil {
		return Error.Wrap(err)
	}
	if host == "" {
		address = net.JoinHostPort("127.0.0.1", port)
	}
	//basic housekeeping
	defer logger.Sync()
	ctx, _ := process.Ctx(cmd)
	if err := process.InitMetrics(ctx, logger, nil, ""); err != nil {
		logger.Warn("Failed to initialize telemetry batcher", zap.Error(err))
	}
	err = version.CheckProcessVersion(ctx, logger, config.Version, version.Build, "Gateway")
	if err != nil {
		return Error.Wrap(err)
	}
	//try to get an uplink
	logger.Info("Starting Storj FTP gateway!", zap.String("Endpoint", address))
	project, scope, err := openProject(ctx)
	if err != nil {
		return Error.Wrap(err)
	}
	defer func() { err = errs.Combine(err, project.Close()) }()
	// the uplink looks good, start the FTP Server
	ftpServer = ftp.NewFtpServer(
		project,
		scope.EncryptionAccess,
		storj.CipherSuite(config.Enc.PathType),
		config.GetEncryptionParameters(),
		config.GetRedundancyScheme(),
		config.Client.SegmentSize,
		logger.With(zap.String("component", "driver")),
	)
	ftpServer.Logger = *logger.With(zap.String("component", "server"))
	if err := ftpServer.ListenAndServe(); err != nil {
		return Error.Wrap(err)
	}
	defer ftpServer.Stop()
	<-ctx.Done()
	return nil
}

func newUplink(ctx context.Context) (*libuplink.Uplink, error) {
	// Transform the gateway config flags to the libuplink config object
	libuplinkCfg := &libuplink.Config{}
	libuplinkCfg.Volatile.Log = logger
	libuplinkCfg.Volatile.MaxInlineSize = config.Client.MaxInlineSize
	libuplinkCfg.Volatile.MaxMemory = config.RS.MaxBufferMem
	libuplinkCfg.Volatile.PeerIDVersion = config.TLS.PeerIDVersions
	libuplinkCfg.Volatile.TLS.SkipPeerCAWhitelist = !config.TLS.UsePeerCAWhitelist
	libuplinkCfg.Volatile.TLS.PeerCAWhitelistPath = config.TLS.PeerCAWhitelistPath
	libuplinkCfg.Volatile.DialTimeout = config.Client.DialTimeout
	libuplinkCfg.Volatile.RequestTimeout = config.Client.RequestTimeout
	return libuplink.NewUplink(ctx, libuplinkCfg)
}

func openProject(ctx context.Context) (*libuplink.Project, *libuplink.Scope, error) {
	uplink, err := newUplink(ctx)
	if err != nil {
		return nil, nil, err
	}
	scope, err := config.GetScope()
	if err != nil {
		return nil, nil, err
	}
	project, err := uplink.OpenProject(ctx, scope.SatelliteAddr, scope.APIKey)
	if err != nil {
		return nil, nil, err
	}
	_, err = project.ListBuckets(ctx, &storj.BucketListOptions{Direction: storj.Forward})
	if err != nil {
		return nil, nil, err
	}
	return project, scope, nil
}

///////////////////////////////////////////////////////////////////////////////
//                            SETUP RELATED STUFF                            //
///////////////////////////////////////////////////////////////////////////////

func cmdSetup(cmd *cobra.Command, args []string) (err error) {
	//defer logger.Sync()
	setupDir, err := filepath.Abs(confDir)
	if err != nil {
		return Error.Wrap(err)
	}

	valid, _ := fpath.IsValidSetupDir(setupDir)
	if !valid {
		return Error.New("FTP configuration already exists (%v)", setupDir)
	}

	err = os.MkdirAll(setupDir, 0700)
	if err != nil {
		return Error.Wrap(err)
	}

	overrides := map[string]interface{}{}

	if config.NonInteractive {
		return nonInteractive(cmd, setupDir, overrides)
	}
	return interactive(cmd, setupDir, overrides)
}

// interactive creates the configuration of the gateway interactively.
func interactive(cmd *cobra.Command, setupDir string, overrides map[string]interface{}) error {
	ctx, _ := process.Ctx(cmd)

	satelliteAddress, err := wizard.PromptForSatellite(cmd)
	if err != nil {
		return Error.Wrap(err)
	}

	apiKeyString, err := wizard.PromptForAPIKey()
	if err != nil {
		return Error.Wrap(err)
	}

	apiKey, err := libuplink.ParseAPIKey(apiKeyString)
	if err != nil {
		return Error.Wrap(err)
	}

	passphrase, err := wizard.PromptForEncryptionPassphrase()
	if err != nil {
		return Error.Wrap(err)
	}

	uplink, err := newUplink(ctx)
	if err != nil {
		return Error.Wrap(err)
	}
	defer func() { err = errs.Combine(err, uplink.Close()) }()

	project, err := uplink.OpenProject(ctx, satelliteAddress, apiKey)
	if err != nil {
		return Error.Wrap(err)
	}
	defer func() { err = errs.Combine(err, project.Close()) }()

	key, err := project.SaltedKeyFromPassphrase(ctx, passphrase)
	if err != nil {
		return Error.Wrap(err)
	}

	scopeData, err := (&libuplink.Scope{
		SatelliteAddr:    satelliteAddress,
		APIKey:           apiKey,
		EncryptionAccess: libuplink.NewEncryptionAccessWithDefaultKey(*key),
	}).Serialize()
	if err != nil {
		return Error.Wrap(err)
	}
	overrides["scope"] = scopeData

	err = process.SaveConfig(cmd, filepath.Join(setupDir, "config.yaml"),
		process.SaveConfigWithOverrides(overrides),
		process.SaveConfigRemovingDeprecated())
	if err != nil {
		return Error.Wrap(err)
	}

	fmt.Println(`
Your FTP Gateway is configured and ready to use!

Some things to try next:

* Run 'gateway --help' to see the operations that can be performed

* See https://github.com/storj/docs/blob/master/FTP-Gateway.md#using-the-aws-FTP-commandline-interface for some example commands`)

	return nil
}

// nonInteractive creates the configuration of the gateway non-interactively.
func nonInteractive(cmd *cobra.Command, setupDir string, overrides map[string]interface{}) error {
	// ensure we're using the scope for the setup
	scope, err := config.GetScope()
	if err != nil {
		return err
	}
	scopeData, err := scope.Serialize()
	if err != nil {
		return err
	}
	overrides["scope"] = scopeData

	return Error.Wrap(process.SaveConfig(cmd, filepath.Join(setupDir, "config.yaml"),
		process.SaveConfigWithOverrides(overrides),
		process.SaveConfigRemovingDeprecated()))
}
