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

	"github.com/btcsuite/btcutil/base58"
	"github.com/minio/cli"
	minio "github.com/minio/minio/cmd"
	"github.com/spf13/cobra"
	"github.com/zeebo/errs"
	"go.uber.org/zap"

	"storj.io/common/fpath"
	"storj.io/storj/cmd/internal/wizard"
	"storj.io/storj/cmd/uplink/cmd"
	"storj.io/storj/pkg/cfgstruct"
	"storj.io/storj/pkg/miniogw"
	"storj.io/storj/pkg/process"
	"storj.io/storj/private/version"
	"storj.io/storj/private/version/checker"
	"storj.io/uplink"
)

// GatewayFlags configuration flags
type GatewayFlags struct {
	NonInteractive bool `help:"disable interactive mode" default:"false" setup:"true"`

	Server miniogw.ServerConfig
	Minio  miniogw.MinioConfig

	cmd.Config

	Version checker.Config

	PBKDFConcurrency int `help:"Unfortunately, up until v0.26.2, keys generated from passphrases depended on the number of cores the local CPU had. If you entered a passphrase with v0.26.2 earlier, you'll want to set this number to the number of CPU cores your computer had at the time. This flag may go away in the future. For new installations the default value is highly recommended." default:"0"`
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

	if setupCfg.NonInteractive {
		return setupCfg.nonInteractive(cmd, setupDir, overrides)
	}
	return setupCfg.interactive(cmd, setupDir, overrides)
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

	ctx, _ := process.Ctx(cmd)

	if err := process.InitMetrics(ctx, zap.L(), nil, ""); err != nil {
		zap.S().Warn("Failed to initialize telemetry batcher: ", err)
	}

	err = checker.CheckProcessVersion(ctx, zap.L(), runCfg.Version, version.Build, "Gateway")
	if err != nil {
		return err
	}

	zap.S().Infof("Starting Storj S3-compatible gateway!\n\n")
	zap.S().Infof("Endpoint: %s\n", address)
	zap.S().Infof("Access key: %s\n", runCfg.Minio.AccessKey)
	zap.S().Infof("Secret key: %s\n", runCfg.Minio.SecretKey)

	err = checkCfg(ctx)
	if err != nil {
		zap.S().Warn("Failed to contact Satellite. Perhaps your configuration is invalid?")
		return err
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
	project, err := runCfg.openProject(ctx)
	if err != nil {
		return err
	}
	defer func() { err = errs.Combine(err, project.Close()) }()

	buckets := project.ListBuckets(ctx, nil)
	_ = buckets.Next()
	return buckets.Err()
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
	project, err := flags.openProject(ctx)
	if err != nil {
		return nil, err
	}

	return miniogw.NewStorjGateway(
		project,
	), nil
}

func (flags *GatewayFlags) newUplinkConfig(ctx context.Context) uplink.Config {
	// Transform the gateway config flags to the uplink config object
	uplinkCfg := uplink.Config{}
	uplinkCfg.DialTimeout = flags.Client.DialTimeout
	return uplinkCfg
}

func (flags GatewayFlags) openProject(ctx context.Context) (*uplink.Project, error) {
	oldAccess, err := flags.GetAccess()
	if err != nil {
		return nil, Error.Wrap(err)
	}

	serializedAccess, err := oldAccess.Serialize()
	if err != nil {
		return nil, Error.Wrap(err)
	}

	access, err := uplink.ParseAccess(serializedAccess)
	if err != nil {
		return nil, Error.Wrap(err)
	}

	config := flags.newUplinkConfig(ctx)
	project, err := config.OpenProject(ctx, access)
	if err != nil {
		return nil, Error.Wrap(err)
	}
	return project, nil
}

// interactive creates the configuration of the gateway interactively.
func (flags GatewayFlags) interactive(cmd *cobra.Command, setupDir string, overrides map[string]interface{}) error {
	ctx, _ := process.Ctx(cmd)

	satelliteAddress, err := wizard.PromptForSatellite(cmd)
	if err != nil {
		return Error.Wrap(err)
	}

	apiKey, err := wizard.PromptForAPIKey()
	if err != nil {
		return Error.Wrap(err)
	}

	passphrase, err := wizard.PromptForEncryptionPassphrase()
	if err != nil {
		return Error.Wrap(err)
	}

	access, err := uplink.RequestAccessWithPassphrase(ctx, satelliteAddress, apiKey, passphrase)
	if err != nil {
		return Error.Wrap(err)
	}

	accessData, err := access.Serialize()
	if err != nil {
		return Error.Wrap(err)
	}
	overrides["access"] = accessData

	err = process.SaveConfig(cmd, filepath.Join(setupDir, "config.yaml"),
		process.SaveConfigWithOverrides(overrides),
		process.SaveConfigRemovingDeprecated())
	if err != nil {
		return Error.Wrap(err)
	}

	fmt.Println(`
Your S3 Gateway is configured and ready to use!

Some things to try next:

* See https://documentation.tardigrade.io/api-reference/s3-gateway for some example commands`)

	return nil
}

// nonInteractive creates the configuration of the gateway non-interactively.
func (flags GatewayFlags) nonInteractive(cmd *cobra.Command, setupDir string, overrides map[string]interface{}) error {
	// ensure we're using the access for the setup
	access, err := setupCfg.GetAccess()
	if err != nil {
		return err
	}
	accessData, err := access.Serialize()
	if err != nil {
		return err
	}
	overrides["access"] = accessData

	return Error.Wrap(process.SaveConfig(cmd, filepath.Join(setupDir, "config.yaml"),
		process.SaveConfigWithOverrides(overrides),
		process.SaveConfigRemovingDeprecated()))
}

func main() {
	process.Exec(rootCmd)
}
