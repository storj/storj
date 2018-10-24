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
	"storj.io/storj/pkg/miniogw/logging"
	"storj.io/storj/pkg/overlay"
	"storj.io/storj/pkg/pointerdb/pdbclient"
	"storj.io/storj/pkg/provider"
	"storj.io/storj/pkg/storage/buckets"
	ecclient "storj.io/storj/pkg/storage/ec"
	"storj.io/storj/pkg/storage/objects"
	segment "storj.io/storj/pkg/storage/segments"
	streams "storj.io/storj/pkg/storage/streams"
	"storj.io/storj/pkg/storj"
	"storj.io/storj/pkg/transport"
)

// RSConfig is a configuration struct that keeps details about default
// redundancy strategy information
type RSConfig struct {
	MaxBufferMem     int `help:"maximum buffer memory (in bytes) to be allocated for read buffers" default:"0x400000"`
	ErasureShareSize int `help:"the size of each new erasure sure in bytes" default:"1024"`
	MinThreshold     int `help:"the minimum pieces required to recover a segment. k." default:"29"`
	RepairThreshold  int `help:"the minimum safe pieces before a repair is triggered. m." default:"35"`
	SuccessThreshold int `help:"the desired total pieces for a segment. o." default:"80"`
	MaxThreshold     int `help:"the largest amount of pieces to encode to. n." default:"95"`
}

// EncryptionConfig is a configuration struct that keeps details about
// encrypting segments
type EncryptionConfig struct {
	EncKey       string `help:"root key for encrypting the data"`
	EncBlockSize int    `help:"size (in bytes) of encrypted blocks" default:"1024"`
	EncType      int    `help:"Type of encryption to use (1=AES-GCM, 2=SecretBox)" default:"1"`
}

// MinioConfig is a configuration struct that keeps details about starting
// Minio
type MinioConfig struct {
	AccessKey string `help:"Minio Access Key to use" default:"insecure-dev-access-key"`
	SecretKey string `help:"Minio Secret Key to use" default:"insecure-dev-secret-key"`
	MinioDir  string `help:"Minio generic server config path" default:"$CONFDIR/minio"`
}

// ClientConfig is a configuration struct for the miniogw that controls how
// the miniogw figures out how to talk to the rest of the network.
type ClientConfig struct {
	// TODO(jt): these should probably be the same
	OverlayAddr   string `help:"Address to contact overlay server through"`
	PointerDBAddr string `help:"Address to contact pointerdb server through"`

	APIKey        string `help:"API Key (TODO: this needs to change to macaroons somehow)"`
	MaxInlineSize int    `help:"max inline segment size in bytes" default:"4096"`
	SegmentSize   int64  `help:"the size of a segment in bytes" default:"64000000"`
}

// Config is a general miniogw configuration struct. This should be everything
// one needs to start a minio gateway.
type Config struct {
	provider.IdentityConfig
	MinioConfig
	ClientConfig
	RSConfig
	EncryptionConfig
}

// Run starts a Minio Gateway given proper config
func (c Config) Run(ctx context.Context) (err error) {
	defer mon.Task()(&ctx)(&err)

	identity, err := c.Load()
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
		"--address", c.Address, "--config-dir", c.MinioDir, "--quiet"})
	return Error.New("unexpected minio exit")
}

func (c Config) action(ctx context.Context, cliCtx *cli.Context, identity *provider.FullIdentity) (err error) {
	defer mon.Task()(&ctx)(&err)

	gw, err := c.NewGateway(ctx, identity)
	if err != nil {
		return err
	}

	minio.StartGateway(cliCtx, logging.Gateway(gw))
	return Error.New("unexpected minio exit")
}

// GetBucketStore returns an implementation of buckets.Store
func (c Config) GetBucketStore(ctx context.Context, identity *provider.FullIdentity) (bs buckets.Store, err error) {
	defer mon.Task()(&ctx)(&err)

	t := transport.NewClient(identity)

	var oc overlay.Client
	oc, err = overlay.NewOverlayClient(identity, c.OverlayAddr)
	if err != nil {
		return nil, err
	}

	pdb, err := pdbclient.NewClient(identity, c.PointerDBAddr, c.APIKey)
	if err != nil {
		return nil, err
	}

	ec := ecclient.NewClient(identity, t, c.MaxBufferMem)
	fc, err := infectious.NewFEC(c.MinThreshold, c.MaxThreshold)
	if err != nil {
		return nil, err
	}
	rs, err := eestream.NewRedundancyStrategy(eestream.NewRSScheme(fc, c.ErasureShareSize), c.RepairThreshold, c.SuccessThreshold)
	if err != nil {
		return nil, err
	}

	segments := segment.NewSegmentStore(oc, ec, pdb, rs, c.MaxInlineSize)

	if c.ErasureShareSize*c.MinThreshold%c.EncBlockSize != 0 {
		err = Error.New("EncryptionBlockSize must be a multiple of ErasureShareSize * RS MinThreshold")
		return nil, err
	}

	key := new(storj.Key)
	copy(key[:], c.EncKey)

	stream, err := streams.NewStreamStore(segments, c.SegmentSize, key, c.EncBlockSize, storj.Cipher(c.EncType))
	if err != nil {
		return nil, err
	}

	obj := objects.NewStore(stream)

	return buckets.NewStore(obj), nil
}

// NewGateway creates a new minio Gateway
func (c Config) NewGateway(ctx context.Context, identity *provider.FullIdentity) (gw minio.Gateway, err error) {
	defer mon.Task()(&ctx)(&err)

	bs, err := c.GetBucketStore(ctx, identity)
	if err != nil {
		return nil, err
	}

	return NewStorjGateway(bs), nil
}
