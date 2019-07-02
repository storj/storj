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
	"github.com/minio/cli"
	minio "github.com/minio/minio/cmd"
	"github.com/spf13/cobra"
	"github.com/zeebo/errs"
	"go.uber.org/zap"

	"storj.io/storj/cmd/internal/wizard"
	"storj.io/storj/internal/fpath"
	libuplink "storj.io/storj/lib/uplink"
	"storj.io/storj/pkg/cfgstruct"
	"storj.io/storj/pkg/miniogw"
	"storj.io/storj/pkg/process"
	"storj.io/storj/pkg/storj"
	"storj.io/storj/uplink"
	"storj.io/storj/uplink/setup"
)

// GatewayFlags configuration flags
type GatewayFlags struct {
	NonInteractive bool `help:"disable interactive mode" default:"false" setup:"true"`

	Server miniogw.ServerConfig
	Minio  miniogw.MinioConfig

	uplink.Config
}

var (
	// Error is the default gateway setup errs class
	Error = errs.Class("gateway setup error")
	// rootCmd represents the base gateway command when called without any subcommands
	rootCmd = &cobra.Command{
		Use:   "gateway",
		Short: "The Storj client-side S3 gateway",
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
		Short: "Run the S3 gateway",
		RunE:  cmdRun,
	}

	setupCfg GatewayFlags
	runCfg   GatewayFlags

	confDir     string
	identityDir string
)

func init() {
	defaultConfDir := fpath.ApplicationDir("storj", "gateway")
	defaultIdentityDir := fpath.ApplicationDir("storj", "identity", "gateway")
	cfgstruct.SetupFlag(zap.L(), rootCmd, &confDir, "config-dir", defaultConfDir, "main directory for gateway configuration")
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
		return Error.New("gateway configuration already exists (%v)", setupDir)
	}

	err = os.MkdirAll(setupDir, 0700)
	if err != nil {
		return Error.Wrap(err)
	}

	overrides := map[string]interface{}{}

	accessKeyFlag := cmd.Flag("minio.access-key")
	if !accessKeyFlag.Changed {
		accessKey, err := generateKey()
		if err != nil {
			return err
		}

		overrides[accessKeyFlag.Name] = accessKey
	}

	secretKeyFlag := cmd.Flag("minio.secret-key")
	if !secretKeyFlag.Changed {
		secretKey, err := generateKey()
		if err != nil {
			return err
		}

		overrides[secretKeyFlag.Name] = secretKey
	}

	// override is required because the default value of Enc.KeyFilepath is ""
	// and setting the value directly in setupCfg.Enc.KeyFiletpath will set the
	// value in the config file but commented out.
	encryptionKeyFilepath := setupCfg.Enc.KeyFilepath
	if encryptionKeyFilepath == "" {
		encryptionKeyFilepath = filepath.Join(setupDir, ".encryption.key")
		overrides["enc.key-filepath"] = encryptionKeyFilepath
	}

	if setupCfg.NonInteractive {
		return setupCfg.nonInteractive(cmd, setupDir, encryptionKeyFilepath, overrides)
	}

	return setupCfg.interactive(cmd, setupDir, encryptionKeyFilepath, overrides)
}

func cmdRun(cmd *cobra.Command, args []string) (err error) {
	address := runCfg.Server.Address
	host, port, err := net.SplitHostPort(address)
	if err != nil {
		return err
	}
	if host == "" {
		address = net.JoinHostPort("127.0.0.1", port)
	}

	fmt.Printf("Starting Storj S3-compatible gateway!\n\n")
	fmt.Printf("Endpoint: %s\n", address)
	fmt.Printf("Access key: %s\n", runCfg.Minio.AccessKey)
	fmt.Printf("Secret key: %s\n", runCfg.Minio.SecretKey)

	ctx := process.Ctx(cmd)

	if err := process.InitMetrics(ctx, nil, ""); err != nil {
		zap.S().Error("Failed to initialize telemetry batcher: ", err)
	}

	err = checkCfg(ctx)
	if err != nil {
		return fmt.Errorf("Failed to contact Satellite.\n"+
			"Perhaps your configuration is invalid?\n%s", err)
	}

	return runCfg.Run(ctx)
}

func generateKey() (key string, err error) {
	var buf [20]byte
	_, err = rand.Read(buf[:])
	if err != nil {
		return "", Error.Wrap(err)
	}
	return base58.Encode(buf[:]), nil
}

func checkCfg(ctx context.Context) (err error) {
	proj, err := runCfg.openProject(ctx)
	if err != nil {
		return err
	}
	defer func() { err = errs.Combine(err, proj.Close()) }()

	_, err = proj.ListBuckets(ctx, &storj.BucketListOptions{Direction: storj.After})
	return err
}

// Run starts a Minio Gateway given proper config
func (flags GatewayFlags) Run(ctx context.Context) (err error) {
	err = minio.RegisterGatewayCommand(cli.Command{
		Name:  "storj",
		Usage: "Storj",
		Action: func(cliCtx *cli.Context) error {
			return flags.action(ctx, cliCtx)
		},
		HideHelpCommand: true,
	})
	if err != nil {
		return err
	}

	// TODO(jt): Surely there is a better way. This is so upsetting
	err = os.Setenv("MINIO_ACCESS_KEY", flags.Minio.AccessKey)
	if err != nil {
		return err
	}
	err = os.Setenv("MINIO_SECRET_KEY", flags.Minio.SecretKey)
	if err != nil {
		return err
	}

	minio.Main([]string{"storj", "gateway", "storj",
		"--address", flags.Server.Address, "--config-dir", flags.Minio.Dir, "--quiet"})
	return errs.New("unexpected minio exit")
}

func (flags GatewayFlags) action(ctx context.Context, cliCtx *cli.Context) (err error) {
	gw, err := flags.NewGateway(ctx)
	if err != nil {
		return err
	}

	minio.StartGateway(cliCtx, miniogw.Logging(gw, zap.L()))
	return errs.New("unexpected minio exit")
}

// NewGateway creates a new minio Gateway
func (flags GatewayFlags) NewGateway(ctx context.Context) (gw minio.Gateway, err error) {
	access, err := setup.LoadEncryptionAccess(ctx, flags.Enc)
	if err != nil {
		return nil, err
	}

	project, err := flags.openProject(ctx)
	if err != nil {
		return nil, err
	}

	return miniogw.NewStorjGateway(
		project,
		access,
		storj.Cipher(flags.Enc.PathType).ToCipherSuite(),
		flags.GetEncryptionScheme().ToEncryptionParameters(),
		flags.GetRedundancyScheme(),
		flags.Client.SegmentSize,
	), nil
}

func (flags GatewayFlags) openProject(ctx context.Context) (*libuplink.Project, error) {
	cfg := libuplink.Config{}
	cfg.Volatile.TLS = struct {
		SkipPeerCAWhitelist bool
		PeerCAWhitelistPath string
	}{
		SkipPeerCAWhitelist: !flags.TLS.UsePeerCAWhitelist,
		PeerCAWhitelistPath: flags.TLS.PeerCAWhitelistPath,
	}
	cfg.Volatile.MaxInlineSize = flags.Client.MaxInlineSize
	cfg.Volatile.MaxMemory = flags.RS.MaxBufferMem

	apiKey, err := libuplink.ParseAPIKey(flags.Client.APIKey)
	if err != nil {
		return nil, err
	}

	uplk, err := libuplink.NewUplink(ctx, &cfg)
	if err != nil {
		return nil, err
	}

	return uplk.OpenProject(ctx, flags.Client.SatelliteAddr, apiKey)
}

// interactive creates the configuration of the gateway interactively.
//
// encryptionKeyFilepath should be set to the filepath indicated by the user or
// or to a default path whose directory tree exists.
func (flags GatewayFlags) interactive(
	cmd *cobra.Command, setupDir string, encryptionKeyFilepath string, overrides map[string]interface{},
) error {
	satelliteAddress, err := wizard.PromptForSatellite(cmd)
	if err != nil {
		return Error.Wrap(err)
	}

	apiKey, err := wizard.PromptForAPIKey()
	if err != nil {
		return Error.Wrap(err)
	}

	humanReadableKey, err := wizard.PromptForEncryptionKey()
	if err != nil {
		return Error.Wrap(err)
	}

	err = setup.SaveEncryptionKey(humanReadableKey, encryptionKeyFilepath)
	if err != nil {
		return Error.Wrap(err)
	}

	overrides["satellite-addr"] = satelliteAddress
	overrides["api-key"] = apiKey
	overrides["enc.key-filepath"] = encryptionKeyFilepath

	err = process.SaveConfigWithAllDefaults(cmd.Flags(), filepath.Join(setupDir, "config.yaml"), overrides)
	if err != nil {
		return nil
	}

	_, err = fmt.Printf(`
Your encryption key is saved to: %s

Your S3 Gateway is configured and ready to use!

Some things to try next:

* Run 'gateway --help' to see the operations that can be performed

* See https://github.com/storj/docs/blob/master/S3-Gateway.md#using-the-aws-s3-commandline-interface for some example commands
	`, encryptionKeyFilepath)
	if err != nil {
		return nil
	}

	return nil

}

// nonInteractive creates the configuration of the gateway non-interactively.
//
// encryptionKeyFilepath should be set to the filepath indicated by the user or
// or to a default path whose directory tree exists.
func (flags GatewayFlags) nonInteractive(
	cmd *cobra.Command, setupDir string, encryptionKeyFilepath string, overrides map[string]interface{},
) error {
	if setupCfg.Enc.EncryptionKey != "" {
		err := setup.SaveEncryptionKey(setupCfg.Enc.EncryptionKey, encryptionKeyFilepath)
		if err != nil {
			return Error.Wrap(err)
		}
	}

	err := process.SaveConfigWithAllDefaults(cmd.Flags(), filepath.Join(setupDir, "config.yaml"), overrides)
	if err != nil {
		return Error.Wrap(err)
	}

	if setupCfg.Enc.EncryptionKey != "" {
		_, _ = fmt.Printf("Your encryption key is saved to: %s\n", encryptionKeyFilepath)
	}

	return nil
}

func main() {
	process.Exec(rootCmd)
}
