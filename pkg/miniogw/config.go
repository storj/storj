// Copyright (C) 2018 Storj Labs, Inc.
// See LICENSE for copying information.

package miniogw

import (
	"context"
	"os"

	"github.com/minio/cli"
	minio "github.com/minio/minio/cmd"
	"github.com/vivint/infectious"

	"storj.io/storj/pkg/eestream"
	"storj.io/storj/pkg/overlay"
	"storj.io/storj/pkg/pointerdb"
	"storj.io/storj/pkg/provider"
	ecclient "storj.io/storj/pkg/storage/ec"
	"storj.io/storj/pkg/storage/objects"
	segment "storj.io/storj/pkg/storage/segments"
	"storj.io/storj/pkg/storage/streams"
	"storj.io/storj/pkg/transport"
)

// RSConfig is a configuration struct that keeps details about default
// redundancy strategy information
type RSConfig struct {
	MaxBufferMem     int `help:"maximum buffer memory (in bytes) to be allocated for read buffers" default:"0x400000"`
	StripeSize       int `help:"the size of each new stripe in bytes" default:"1024"`
	MinThreshold     int `help:"the minimum pieces required to recover a segment. k." default:"20"`
	RepairThreshold  int `help:"the minimum safe pieces before a repair is triggered. m." default:"30"`
	SuccessThreshold int `help:"the desired total pieces for a segment. o." default:"40"`
	MaxThreshold     int `help:"the largest amount of pieces to encode to. n." default:"50"`
}

// MinioConfig is a configuration struct that keeps details about starting
// Minio
type MinioConfig struct {
	AccessKey string `help:"Minio Access Key to use" default:"insecure-dev-access-key"`
	SecretKey string `help:"Minio Secret Key to use" default:"insecure-dev-secret-key"`
	MinioDir  string `help:"Minio generic server config path" default:"$CONFDIR/miniogw"`
}

// ClientConfig is a configuration struct for the miniogw that controls how
// the miniogw figures out how to talk to the rest of the network.
type ClientConfig struct {
	// TODO(jt): these should probably be the same
	OverlayAddr   string `help:"Address to contact overlay server through"`
	PointerDBAddr string `help:"Address to contact pointerdb server through"`
}

// Config is a general miniogw configuration struct. This should be everything
// one needs to start a minio gateway.
type Config struct {
	provider.IdentityConfig
	MinioConfig
	ClientConfig
	RSConfig
}

// Run starts a Minio Gateway given proper config
func (c Config) Run(ctx context.Context) (err error) {
	defer mon.Task()(&ctx)(&err)

	identity, err := c.LoadIdentity()
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
	err = os.Setenv("MINIO_ACCESS_KEY", c.AccessKey)
	if err != nil {
		return err
	}
	err = os.Setenv("MINIO_SECRET_KEY", c.SecretKey)
	if err != nil {
		return err
	}
	minio.Main([]string{"storj", "gateway", "storj",
		"--address", c.Address, "--config-dir", c.MinioDir})
	return Error.New("unexpected minio exit")
}

func (c Config) action(ctx context.Context, cliCtx *cli.Context,
	identity *provider.FullIdentity) (
	err error) {
	defer mon.Task()(&ctx)(&err)

	// TODO(jt): the transport client should use tls and should use the identity
	// defined in this function.
	t := transport.NewClient()

	// TODO(jt): overlay.NewClient should dial the overlay server with the
	// transport client. probably should use the same connection as the
	// pointerdb client
	oc, err := overlay.NewOverlayClient(c.OverlayAddr)
	if err != nil {
		return err
	}

	// TODO(jt): pointerdb.NewClient should dial the pointerdb server with the
	// transport client. probably should use the same connection as the
	// overlay client
	pdb, err := pointerdb.NewClient(c.PointerDBAddr)
	if err != nil {
		return err
	}

	ec := ecclient.NewClient(t, c.MaxBufferMem)
	fc, err := infectious.NewFEC(c.MinThreshold, c.MaxThreshold)
	if err != nil {
		return err
	}
	rs, err := eestream.NewRedundancyStrategy(
		eestream.NewRSScheme(fc, c.StripeSize/c.MaxThreshold),
		c.RepairThreshold, c.SuccessThreshold)
	if err != nil {
		return err
	}

	segments := segment.NewSegmentStore(oc, ec, pdb, rs)

	// TODO(jt): wrap segments and turn segments into streams actually
	// TODO: passthrough is bad
	stream := streams.NewPassthrough(segments)

	minio.StartGateway(cliCtx, NewStorjGateway(objects.NewStore(stream)))
	return Error.New("unexpected minio exit")
}
