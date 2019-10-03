// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package main

import (
	"context"
	"crypto/rand"
	"fmt"
	"net"
	"os"
	"path/filepath"

	base58 "github.com/jbenet/go-base58"
	"github.com/spf13/cobra"
	"github.com/zeebo/errs"
	"go.uber.org/zap"
	"github.com/wthorp/ftpserver-zap/server"

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

	Server ServerConfig
	uplink.Config
	Version version.Config
}

var (
	// Error is the default gateway setup errs class
	Error = errs.Class("gateway setup error")
	// rootCmd represents the base gateway command when called without any subcommands
	rootCmd = &cobra.Command{
		Use:   "gateway",
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

	setupCfg FTPFlags
	runCfg   FTPFlags

	confDir     string
	identityDir string
)

func init() {
	defaultConfDir := fpath.ApplicationDir("storj", "ftp")
	defaultIdentityDir := fpath.ApplicationDir("storj", "identity", "gateway")
	cfgstruct.SetupFlag(zap.L(), rootCmd, &confDir, "config-dir", defaultConfDir, "main directory for FTP configuration")
	cfgstruct.SetupFlag(zap.L(), rootCmd, &identityDir, "identity-dir", defaultIdentityDir, "main directory for gateway identity credentials")
	defaults := cfgstruct.DefaultsFlag(rootCmd)

	rootCmd.AddCommand(runCmd)
	rootCmd.AddCommand(setupCmd)
	process.Bind(runCmd, &runCfg, defaults, cfgstruct.ConfDir(confDir), cfgstruct.IdentityDir(identityDir))
	process.Bind(setupCmd, &setupCfg, defaults, cfgstruct.ConfDir(confDir), cfgstruct.IdentityDir(identityDir), cfgstruct.SetupMode())
}

func cmdSetup(cmd *cobra.Command, args []string) (err error) {
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

	if setupCfg.NonInteractive {
		return setupCfg.nonInteractive(cmd, setupDir, overrides)
	}
	return setupCfg.interactive(cmd, setupDir, overrides)
}

func cmdRun(cmd *cobra.Command, args []string) (err error) {
	address := runCfg.Address
	host, port, err := net.SplitHostPort(address)
	if err != nil {
		return err
	}
	if host == "" {
		address = net.JoinHostPort("127.0.0.1", port)
	}

	ctx, _ := process.Ctx(cmd)

	if err := process.InitMetrics(ctx, zap.L(), nil, ""); err != nil {
		zap.S().Warn("Failed to initialize telemetry batcher: ", err)
	}

	err = version.CheckProcessVersion(ctx, zap.L(), runCfg.Version, version.Build, "Gateway")
	if err != nil {
		return err
	}

	zap.S().Infof("Starting Storj FTP gateway!\n\n")
	zap.S().Infof("Endpoint: %s\n", address)

	err = checkCfg(ctx)
	if err != nil {
		zap.S().Warn("Failed to contact Satellite. Perhaps your configuration is invalid?")
		return err
	}

	return runCfg.Run(ctx)
}

func checkCfg(ctx context.Context) (err error) {
	proj, err := runCfg.openProject(ctx)
	if err != nil {
		return err
	}
	defer func() { err = errs.Combine(err, proj.Close()) }()

	_, err = proj.ListBuckets(ctx, &storj.BucketListOptions{Direction: storj.Forward})
	return err
}

// Run starts a FTP gateway given proper config
func (flags FTPFlags) Run(ctx context.Context) (err error) {
	
}

func (flags FTPFlags) action(ctx context.Context, cliCtx *cli.Context) (err error) {
	gw, err := flags.NewGateway(ctx)
	if err != nil {
		return err
	}

	minio.StartGateway(cliCtx, miniogw.Logging(gw, zap.L()))
	return errs.New("unexpected minio exit")
}

// NewGateway creates a new FTP Gateway
func (flags FTPFlags) NewGateway(ctx context.Context) (gw minio.Gateway, err error) {
	scope, err := flags.GetScope()
	if err != nil {
		return nil, err
	}

	project, err := flags.openProject(ctx)
	if err != nil {
		return nil, err
	}

	return ftp.NewStorjGateway(
		project,
		scope.EncryptionAccess,
		storj.CipherSuite(flags.Enc.PathType),
		flags.GetEncryptionParameters(),
		flags.GetRedundancyScheme(),
		flags.Client.SegmentSize,
	), nil
}

func (flags *FTPFlags) newUplink(ctx context.Context) (*libuplink.Uplink, error) {
	// Transform the gateway config flags to the libuplink config object
	libuplinkCfg := &libuplink.Config{}
	libuplinkCfg.Volatile.Log = zap.L()
	libuplinkCfg.Volatile.MaxInlineSize = flags.Client.MaxInlineSize
	libuplinkCfg.Volatile.MaxMemory = flags.RS.MaxBufferMem
	libuplinkCfg.Volatile.PeerIDVersion = flags.TLS.PeerIDVersions
	libuplinkCfg.Volatile.TLS.SkipPeerCAWhitelist = !flags.TLS.UsePeerCAWhitelist
	libuplinkCfg.Volatile.TLS.PeerCAWhitelistPath = flags.TLS.PeerCAWhitelistPath
	libuplinkCfg.Volatile.DialTimeout = flags.Client.DialTimeout
	libuplinkCfg.Volatile.RequestTimeout = flags.Client.RequestTimeout

	return libuplink.NewUplink(ctx, libuplinkCfg)
}

func (flags FTPFlags) openProject(ctx context.Context) (*libuplink.Project, error) {
	scope, err := flags.GetScope()
	if err != nil {
		return nil, Error.Wrap(err)
	}
	// TODO(jeff): this leaks the uplink and project :(
	uplink, err := flags.newUplink(ctx)
	if err != nil {
		return nil, Error.Wrap(err)
	}
	project, err := uplink.OpenProject(ctx, scope.SatelliteAddr, scope.APIKey)
	if err != nil {
		return nil, Error.Wrap(err)
	}
	return project, nil
}

// interactive creates the configuration of the gateway interactively.
func (flags FTPFlags) interactive(cmd *cobra.Command, setupDir string, overrides map[string]interface{}) error {
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

	uplink, err := flags.newUplink(ctx)
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
func (flags FTPFlags) nonInteractive(cmd *cobra.Command, setupDir string, overrides map[string]interface{}) error {
	// ensure we're using the scope for the setup
	scope, err := setupCfg.GetScope()
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

func main() {


	// Setting up the logger
	logger, _ := zap.NewProduction()
	defer logger.Sync()

	// Loading the driver
	driver, err := ftp.&FTPDriver{
		Logger:       *zap.NewNop(),
		SettingsFile: settingsFile,
		BaseDir:      dir,
	}

	if err != nil {
		logger.Error("Could not load the driver", zap.Error(err))
		return
	}

	// Overriding the driver default silent logger by a sub-logger (component: driver)
	driver.Logger = *logger.With(zap.String("component", "driver"))

	// Instantiating the server by passing our driver implementation
	ftpServer = server.NewFtpServer(driver)

	// Overriding the server default silent logger by a sub-logger (component: server)
	ftpServer.Logger = *logger.With(zap.String("component", "server"))

	// Preparing the SIGTERM handling
	go signalHandler()

	if err := ftpServer.ListenAndServe(); err != nil {
		logger.Error("Problem listening", zap.Error(err))
	}
}

func signalHandler() {
	ch := make(chan os.Signal)
	signal.Notify(ch, syscall.SIGTERM)
	for {
		switch <-ch {
		case syscall.SIGTERM:
			ftpServer.Stop()
			break
		}
	}
}
