// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package uplink

import (
	"context"
	"errors"

	"github.com/vivint/infectious"
	"github.com/zeebo/errs"
	monkit "gopkg.in/spacemonkeygo/monkit.v2"

	"storj.io/storj/internal/memory"
	"storj.io/storj/pkg/eestream"
	"storj.io/storj/pkg/identity"
	"storj.io/storj/pkg/metainfo/kvmetainfo"
	"storj.io/storj/pkg/overlay"
	"storj.io/storj/pkg/pointerdb/pdbclient"
	"storj.io/storj/pkg/storage/buckets"
	ecclient "storj.io/storj/pkg/storage/ec"
	"storj.io/storj/pkg/storage/segments"
	"storj.io/storj/pkg/storage/streams"
	"storj.io/storj/pkg/storj"
)

// RSConfig is a configuration struct that keeps details about default
// redundancy strategy information
type RSConfig struct {
	MaxBufferMem     memory.Size `help:"maximum buffer memory (in bytes) to be allocated for read buffers" default:"4MiB"`
	ErasureShareSize memory.Size `help:"the size of each new erasure sure in bytes" default:"1KiB"`
	MinThreshold     int         `help:"the minimum pieces required to recover a segment. k." default:"29"`
	RepairThreshold  int         `help:"the minimum safe pieces before a repair is triggered. m." default:"35"`
	SuccessThreshold int         `help:"the desired total pieces for a segment. o." default:"80"`
	MaxThreshold     int         `help:"the largest amount of pieces to encode to. n." default:"95"`
}

// EncryptionConfig is a configuration struct that keeps details about
// encrypting segments
type EncryptionConfig struct {
	Key       string      `help:"root key for encrypting the data"`
	BlockSize memory.Size `help:"size (in bytes) of encrypted blocks" default:"1KiB"`
	DataType  int         `help:"Type of encryption to use for content and metadata (1=AES-GCM, 2=SecretBox)" default:"1"`
	PathType  int         `help:"Type of encryption to use for paths (0=Unencrypted, 1=AES-GCM, 2=SecretBox)" default:"1"`
}

// ClientConfig is a configuration struct for the uplink that controls how
// to talk to the rest of the network.
type ClientConfig struct {
	// TODO(jt): these should probably be the same
	OverlayAddr   string `help:"Address to contact overlay server through"`
	PointerDBAddr string `help:"Address to contact pointerdb server through"`

	APIKey        string      `help:"API Key (TODO: this needs to change to macaroons somehow)"`
	MaxInlineSize memory.Size `help:"max inline segment size in bytes" default:"4KiB"`
	SegmentSize   memory.Size `help:"the size of a segment in bytes" default:"64MiB"`
}

// Config uplink configuration
type Config struct {
	Client ClientConfig
	RS     RSConfig
	Enc    EncryptionConfig
}

var (
	mon = monkit.Package()

	// Error is the errs class of standard End User Client errors
	Error = errs.Class("Uplink configuration error")
)

// GetMetainfo returns an implementation of storj.Metainfo
func (c Config) GetMetainfo(ctx context.Context, identity *identity.FullIdentity) (db storj.Metainfo, ss streams.Store, err error) {
	defer mon.Task()(&ctx)(&err)

	if c.Client.OverlayAddr == "" || c.Client.PointerDBAddr == "" {
		var errlist errs.Group
		if c.Client.OverlayAddr == "" {
			errlist.Add(errors.New("overlay address not specified"))
		}
		if c.Client.PointerDBAddr == "" {
			errlist.Add(errors.New("pointerdb address not specified"))
		}
		return nil, nil, errlist.Err()
	}

	oc, err := overlay.NewClient(identity, c.Client.OverlayAddr)
	if err != nil {
		return nil, nil, Error.New("failed to connect to overlay: %v", err)
	}

	pdb, err := pdbclient.NewClient(identity, c.Client.PointerDBAddr, c.Client.APIKey)
	if err != nil {
		return nil, nil, Error.New("failed to connect to pointer DB: %v", err)
	}

	ec := ecclient.NewClient(identity, c.RS.MaxBufferMem.Int())
	fc, err := infectious.NewFEC(c.RS.MinThreshold, c.RS.MaxThreshold)
	if err != nil {
		return nil, nil, Error.New("failed to create erasure coding client: %v", err)
	}
	rs, err := eestream.NewRedundancyStrategy(eestream.NewRSScheme(fc, c.RS.ErasureShareSize.Int()), c.RS.RepairThreshold, c.RS.SuccessThreshold)
	if err != nil {
		return nil, nil, Error.New("failed to create redundancy strategy: %v", err)
	}

	segments := segments.NewSegmentStore(oc, ec, pdb, rs, c.Client.MaxInlineSize.Int())

	if c.RS.ErasureShareSize.Int()*c.RS.MinThreshold%c.Enc.BlockSize.Int() != 0 {
		err = Error.New("EncryptionBlockSize must be a multiple of ErasureShareSize * RS MinThreshold")
		return nil, nil, err
	}

	key := new(storj.Key)
	copy(key[:], c.Enc.Key)

	streams, err := streams.NewStreamStore(segments, c.Client.SegmentSize.Int64(), key, c.Enc.BlockSize.Int(), storj.Cipher(c.Enc.DataType))
	if err != nil {
		return nil, nil, Error.New("failed to create stream store: %v", err)
	}

	buckets := buckets.NewStore(streams)

	return kvmetainfo.New(buckets, streams, segments, pdb, key), streams, nil
}

// GetRedundancyScheme returns the configured redundancy scheme for new uploads
func (c Config) GetRedundancyScheme() storj.RedundancyScheme {
	return storj.RedundancyScheme{
		Algorithm:      storj.ReedSolomon,
		RequiredShares: int16(c.RS.MinThreshold),
		RepairShares:   int16(c.RS.RepairThreshold),
		OptimalShares:  int16(c.RS.SuccessThreshold),
		TotalShares:    int16(c.RS.MaxThreshold),
	}
}

// GetEncryptionScheme returns the configured encryption scheme for new uploads
func (c Config) GetEncryptionScheme() storj.EncryptionScheme {
	return storj.EncryptionScheme{
		Cipher:    storj.Cipher(c.Enc.DataType),
		BlockSize: int32(c.Enc.BlockSize),
	}
}
