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

	"storj.io/storj/internal/fpath"
	libuplink "storj.io/storj/lib/uplink"
	"storj.io/storj/pkg/cfgstruct"
	"storj.io/storj/pkg/miniogw"
	"storj.io/storj/pkg/process"
	"storj.io/storj/pkg/storj"
	"storj.io/storj/uplink"
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
	cfgstruct.Bind(runCmd.Flags(), &runCfg, defaults, cfgstruct.ConfDir(confDir), cfgstruct.IdentityDir(identityDir))
	cfgstruct.BindSetup(setupCmd.Flags(), &setupCfg, defaults, cfgstruct.ConfDir(confDir), cfgstruct.IdentityDir(identityDir))
}

func cmdSetup(cmd *cobra.Command, args []string) (err error) {
	setupDir, err := filepath.Abs(confDir)
	if err != nil {
		return err
	}

	valid, _ := fpath.IsValidSetupDir(setupDir)
	if !valid {
		return fmt.Errorf("gateway configuration already exists (%v)", setupDir)
	}

	err = os.MkdirAll(setupDir, 0700)
	if err != nil {
		return err
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

	if !setupCfg.NonInteractive {
		return setupCfg.interactive(cmd, setupDir, overrides)
	}

	return process.SaveConfigWithAllDefaults(cmd.Flags(), filepath.Join(setupDir, "config.yaml"), overrides)
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
		return "", err
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
	if err != nil {
		return err
	}

	return nil
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
	encKey := new(storj.Key)
	copy(encKey[:], flags.Enc.Key)

	project, err := flags.openProject(ctx)
	if err != nil {
		return nil, err
	}

	return miniogw.NewStorjGateway(
		project,
		encKey,
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

	uplink, err := libuplink.NewUplink(ctx, &cfg)
	if err != nil {
		return nil, err
	}

	apiKey, err := libuplink.ParseAPIKey(flags.Client.APIKey)
	if err != nil {
		return nil, err
	}

	encKey := new(storj.Key)
	copy(encKey[:], flags.Enc.Key)

	var opts libuplink.ProjectOptions
	opts.Volatile.EncryptionKey = encKey

	return uplink.OpenProject(ctx, flags.Client.SatelliteAddr, apiKey, &opts)
}

func (flags GatewayFlags) interactive(cmd *cobra.Command, setupDir string, overrides map[string]interface{}) error {
	satelliteAddress, err := cfgstruct.PromptForSatelitte(cmd)
	if err != nil {
		return Error.Wrap(err)
	}

	apiKey, err := cfgstruct.PromptForAPIKey()
	if err != nil {
		return Error.Wrap(err)
	}

	encKey, err := cfgstruct.PromptForEncryptionKey()
	if err != nil {
		return Error.Wrap(err)
	}

	overrides["satellite-addr"] = satelliteAddress
	overrides["api-key"] = apiKey
	overrides["enc.key"] = encKey

	err = process.SaveConfigWithAllDefaults(cmd.Flags(), filepath.Join(setupDir, "config.yaml"), overrides)
	if err != nil {
		return nil
	}

	_, err = fmt.Println(`
Your S3 Gateway is configured and ready to use!

Some things to try next:

* Run 'gateway --help' to see the operations that can be performed

* See https://github.com/storj/docs/blob/master/S3-Gateway.md#using-the-aws-s3-commandline-interface for some example commands
	`)
	if err != nil {
		return nil
	}

	return nil

}

func main() {
	process.Exec(rootCmd)
}
