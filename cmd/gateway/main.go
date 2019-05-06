// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package main

import (
	"bytes"
	"context"
	"crypto/rand"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"strings"

	base58 "github.com/jbenet/go-base58"
	"github.com/minio/cli"
	minio "github.com/minio/minio/cmd"
	"github.com/spf13/cobra"
	"github.com/zeebo/errs"
	"go.uber.org/zap"
	"golang.org/x/crypto/ssh/terminal"

	"storj.io/storj/internal/fpath"
	libuplink "storj.io/storj/lib/uplink"
	"storj.io/storj/pkg/cfgstruct"
	"storj.io/storj/pkg/identity"
	"storj.io/storj/pkg/miniogw"
	"storj.io/storj/pkg/process"
	"storj.io/storj/pkg/storj"
	"storj.io/storj/uplink"
)

// GatewayFlags configuration flags
type GatewayFlags struct {
	Identity          identity.Config
	GenerateTestCerts bool `default:"false" help:"generate sample TLS certs for Minio GW" setup:"true"`
	Interactive       bool `help:"enable|disable interactive mode" default:"true" setup:"true"`

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
	// HERE BindSetup takes the flags and writes to the config ?
	cfgstruct.BindSetup(setupCmd.Flags(), &setupCfg, defaults, cfgstruct.ConfDir(confDir), cfgstruct.IdentityDir(identityDir))
}

/*
~/go/bin/gateway setup --api-key abc123 --satellite-addr mars.tardigrade.io:7777 \
  --enc.key highlydistributedridiculouslyresilient
*/
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

	if setupCfg.GenerateTestCerts {
		minioCerts := filepath.Join(setupDir, "minio", "certs")
		if err := os.MkdirAll(minioCerts, 0744); err != nil {
			return err
		}
		if err := os.Link(setupCfg.Identity.CertPath, filepath.Join(minioCerts, "public.crt")); err != nil {
			return err
		}
		if err := os.Link(setupCfg.Identity.KeyPath, filepath.Join(minioCerts, "private.key")); err != nil {
			return err
		}
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

	if setupCfg.Interactive {
		return setupCfg.interactive(cmd, setupDir, overrides)
	}

	return process.SaveConfigWithAllDefaults(cmd.Flags(), filepath.Join(setupDir, "config.yaml"), overrides)
}

func cmdRun(cmd *cobra.Command, args []string) (err error) {
	identity, err := runCfg.Identity.Load()
	if err != nil {
		zap.S().Fatal(err)
	}

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
	metainfo, _, err := runCfg.GetMetainfo(ctx, identity)
	if err != nil {
		return err
	}

	if err := process.InitMetricsWithCertPath(ctx, nil, runCfg.Identity.CertPath); err != nil {
		zap.S().Error("Failed to initialize telemetry batcher: ", err)
	}

	_, err = metainfo.ListBuckets(ctx, storj.BucketListOptions{Direction: storj.After})
	if err != nil {
		return fmt.Errorf("Failed to contact Satellite.\n"+
			"Perhaps your configuration is invalid?\n%s", err)
	}

	return runCfg.Run(ctx, identity)
}

func generateKey() (key string, err error) {
	var buf [20]byte
	_, err = rand.Read(buf[:])
	if err != nil {
		return "", err
	}
	return base58.Encode(buf[:]), nil
}

// Run starts a Minio Gateway given proper config
func (flags GatewayFlags) Run(ctx context.Context, identity *identity.FullIdentity) (err error) {
	err = minio.RegisterGatewayCommand(cli.Command{
		Name:  "storj",
		Usage: "Storj",
		Action: func(cliCtx *cli.Context) error {
			return flags.action(ctx, cliCtx, identity)
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

func (flags GatewayFlags) action(ctx context.Context, cliCtx *cli.Context, identity *identity.FullIdentity) (err error) {
	gw, err := flags.NewGateway(ctx, identity)
	if err != nil {
		return err
	}

	minio.StartGateway(cliCtx, miniogw.Logging(gw, zap.L()))
	return errs.New("unexpected minio exit")
}

// NewGateway creates a new minio Gateway
func (flags GatewayFlags) NewGateway(ctx context.Context, ident *identity.FullIdentity) (gw minio.Gateway, err error) {
	cfg := libuplink.Config{}
	cfg.Volatile.TLS = struct {
		SkipPeerCAWhitelist bool
		PeerCAWhitelistPath string
	}{
		SkipPeerCAWhitelist: !flags.TLS.UsePeerCAWhitelist,
		PeerCAWhitelistPath: flags.TLS.PeerCAWhitelistPath,
	}
	cfg.Volatile.UseIdentity = ident
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

	project, err := uplink.OpenProject(ctx, flags.Client.SatelliteAddr, apiKey, &opts)
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

func (flags GatewayFlags) interactive(cmd *cobra.Command, setupDir string, overrides map[string]interface{}) error {
	_, err := fmt.Print(`
Pick satellite to use:
[1] mars.tardigrade.io
[2] jupiter.tardigrade.io
[3] saturn.tardigrade.io
Please enter numeric choice or enter satellite address manually [1]: `)
	if err != nil {
		return err
	}
	satellites := []string{"mars.tardigrade.io", "jupiter.tardigrade.io", "saturn.tardigrade.io"}
	var satelliteAddress string
	n, err := fmt.Scanln(&satelliteAddress)
	if err != nil {
		if n == 0 {
			// fmt.Scanln cannot handle empty input
			satelliteAddress = satellites[0]
		} else {
			return err
		}
	}

	// TODO add better validation
	if satelliteAddress == "" {
		return errs.New("satellite address cannot be empty")
	} else if len(satelliteAddress) == 1 {
		switch satelliteAddress {
		case "1":
			satelliteAddress = satellites[0]
		case "2":
			satelliteAddress = satellites[1]
		case "3":
			satelliteAddress = satellites[2]
		default:
			return errs.New("Satellite address cannot be one character")
		}
	}

	satelliteAddress, err = ApplyDefaultHostAndPortToAddr(satelliteAddress, cmd.Flags().Lookup("satellite-addr").Value.String())
	if err != nil {
		return err
	}

	_, err = fmt.Print("Enter your API key: ")
	if err != nil {
		return err
	}
	var apiKey string
	n, err = fmt.Scanln(&apiKey)
	if err != nil && n != 0 {
		return err
	}

	if apiKey == "" {
		return errs.New("API key cannot be empty")
	}

	_, err = fmt.Print("Enter your encryption passphrase: ")
	if err != nil {
		return err
	}
	encKey, err := terminal.ReadPassword(int(os.Stdin.Fd()))
	if err != nil {
		return err
	}
	_, err = fmt.Println()
	if err != nil {
		return err
	}

	_, err = fmt.Print("Enter your encryption passphrase again: ")
	if err != nil {
		return err
	}
	repeatedEncKey, err := terminal.ReadPassword(int(os.Stdin.Fd()))
	if err != nil {
		return err
	}
	_, err = fmt.Println()
	if err != nil {
		return err
	}

	if !bytes.Equal(encKey, repeatedEncKey) {
		return errs.New("encryption passphrases doesn't match")
	}

	if len(encKey) == 0 {
		_, err = fmt.Println("Warning: Encryption passphrase is empty!")
		if err != nil {
			return err
		}
	}

	overrides["satellite-addr"] = satelliteAddress
	overrides["api-key"] = apiKey
	overrides["enc.key"] = string(encKey)

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

// ApplyDefaultHostAndPortToAddr applies the default host and/or port if either is missing in the specified address.
func ApplyDefaultHostAndPortToAddr(address, defaultAddress string) (string, error) {
	defaultHost, defaultPort, err := net.SplitHostPort(defaultAddress)
	if err != nil {
		return "", Error.Wrap(err)
	}

	addressParts := strings.Split(address, ":")
	numberOfParts := len(addressParts)

	if numberOfParts > 1 && len(addressParts[0]) > 0 && len(addressParts[1]) > 0 {
		// address is host:port so skip applying any defaults.
		return address, nil
	}

	// We are missing a host:port part. Figure out which part we are missing.
	indexOfPortSeparator := strings.Index(address, ":")
	lengthOfFirstPart := len(addressParts[0])

	if indexOfPortSeparator < 0 {
		if lengthOfFirstPart == 0 {
			// address is blank.
			return defaultAddress, nil
		}
		// address is host
		return net.JoinHostPort(addressParts[0], defaultPort), nil
	}

	if indexOfPortSeparator == 0 {
		// address is :1234
		return net.JoinHostPort(defaultHost, addressParts[1]), nil
	}

	// address is host:
	return net.JoinHostPort(addressParts[0], defaultPort), nil
}

func main() {
	process.Exec(rootCmd)
}
