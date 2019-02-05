// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package miniogw

import (
	"context"
	"os"

	"github.com/minio/cli"
	minio "github.com/minio/minio/cmd"
	"go.uber.org/zap"

	"storj.io/storj/pkg/identity"
	"storj.io/storj/pkg/metainfo"
	"storj.io/storj/pkg/storj"
)

// MinioConfig is a configuration struct that keeps details about starting
// Minio
type MinioConfig struct {
	AccessKey string `help:"Minio Access Key to use" default:"insecure-dev-access-key"`
	SecretKey string `help:"Minio Secret Key to use" default:"insecure-dev-secret-key"`
	Dir       string `help:"Minio generic server config path" default:"$CONFDIR/minio"`
}

// ServerConfig determines how minio listens for requests
type ServerConfig struct {
	Address string `help:"address to serve S3 api over" default:"localhost:7777"`
}

// Config is a general miniogw configuration struct. This should be everything
// one needs to start a minio gateway.
type Config struct {
	Identity identity.Config
	Server   ServerConfig
	Minio    MinioConfig

	metainfo.Config
}

// Run starts a Minio Gateway given proper config
func (c Config) Run(ctx context.Context) (err error) {
	defer mon.Task()(&ctx)(&err)

	identity, err := c.Identity.Load()
	if err != nil {
		return err
	}

	err = minio.RegisterGatewayCommand(cli.Command{
		Name:  "storj",
		Usage: "Storj",
		Action: func(cliCtx *cli.Context) error {
			return c.action(ctx, cliCtx, identity)
		},
		HideHelpCommand: true,
	})
	if err != nil {
		return err
	}

	// TODO(jt): Surely there is a better way. This is so upsetting
	err = os.Setenv("MINIO_ACCESS_KEY", c.Minio.AccessKey)
	if err != nil {
		return err
	}
	err = os.Setenv("MINIO_SECRET_KEY", c.Minio.SecretKey)
	if err != nil {
		return err
	}

	minio.Main([]string{"storj", "gateway", "storj",
		"--address", c.Server.Address, "--config-dir", c.Minio.Dir, "--quiet"})
	return Error.New("unexpected minio exit")
}

func (c Config) action(ctx context.Context, cliCtx *cli.Context, identity *identity.FullIdentity) (err error) {
	defer mon.Task()(&ctx)(&err)

	gw, err := c.NewGateway(ctx, identity)
	if err != nil {
		return err
	}

	minio.StartGateway(cliCtx, Logging(gw, zap.L()))
	return Error.New("unexpected minio exit")
}

// NewGateway creates a new minio Gateway
func (c Config) NewGateway(ctx context.Context, identity *identity.FullIdentity) (gw minio.Gateway, err error) {
	defer mon.Task()(&ctx)(&err)

	metainfo, streams, err := c.GetMetainfo(ctx, identity)
	if err != nil {
		return nil, err
	}

	return NewStorjGateway(metainfo, streams, storj.Cipher(c.Enc.PathType), c.GetEncryptionScheme(), c.GetRedundancyScheme()), nil
}
