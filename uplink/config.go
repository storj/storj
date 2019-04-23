// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package uplink

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"time"

	"github.com/vivint/infectious"
	"github.com/zeebo/errs"
	monkit "gopkg.in/spacemonkeygo/monkit.v2"

	"storj.io/storj/internal/memory"
	"storj.io/storj/pkg/eestream"
	"storj.io/storj/pkg/encryption"
	"storj.io/storj/pkg/identity"
	"storj.io/storj/pkg/metainfo/kvmetainfo"
	"storj.io/storj/pkg/peertls/tlsopts"
	"storj.io/storj/pkg/storage/buckets"
	ecclient "storj.io/storj/pkg/storage/ec"
	"storj.io/storj/pkg/storage/segments"
	"storj.io/storj/pkg/storage/streams"
	"storj.io/storj/pkg/storj"
	"storj.io/storj/pkg/transport"
	"storj.io/storj/uplink/metainfo"
)

// RSConfig is a configuration struct that keeps details about default
// redundancy strategy information
type RSConfig struct {
	MaxBufferMem     memory.Size `help:"maximum buffer memory (in bytes) to be allocated for read buffers" default:"4MiB"`
	ErasureShareSize memory.Size `help:"the size of each new erasure sure in bytes" default:"1KiB"`
	MinThreshold     int         `help:"the minimum pieces required to recover a segment. k." releaseDefault:"29" devDefault:"4"`
	RepairThreshold  int         `help:"the minimum safe pieces before a repair is triggered. m." releaseDefault:"35" devDefault:"6"`
	SuccessThreshold int         `help:"the desired total pieces for a segment. o." releaseDefault:"80" devDefault:"8"`
	MaxThreshold     int         `help:"the largest amount of pieces to encode to. n." releaseDefault:"95" devDefault:"10"`
}

// EncryptionConfig is a configuration struct that keeps details about
// encrypting segments
type EncryptionConfig struct {
	// TODO: WIP#if/v3-1541 read TODO in Key method why the following field cannot
	// be added
	// RootKey     *storj.Key  // The root key for encrypting the data and read it from KeyFilepath, see Key method.
	KeyFilepath string      `help:"the path to the file which contains the root key for encrypting the data"`
	BlockSize   memory.Size `help:"size (in bytes) of encrypted blocks" default:"1KiB"`
	DataType    int         `help:"Type of encryption to use for content and metadata (1=AES-GCM, 2=SecretBox)" default:"1"`
	PathType    int         `help:"Type of encryption to use for paths (0=Unencrypted, 1=AES-GCM, 2=SecretBox)" default:"1"`
}

// Key returns the root key for encrypting the data which is stored in the path
// indicated by KeyFilepath field.
//
// It returns an error if:
//
// * The file doesn't exist.
// * The file doesn't have exactly 0400 permissions.
// * There is an I/O error.
//
// TODO: WIP#if/v3-1541 refactor cache the key after the firs read. The problem
// is with pkg/cfgstruct which is inflexible on accepting unexported keys nor
// several different types as a type of storj.Key, [32]byte, etc.
func (encCfg *EncryptionConfig) Key() (storj.Key, error) {
	if encCfg.KeyFilepath == "" {
		return storj.Key{}, nil
	}

	f, err := os.Open(encCfg.KeyFilepath)
	if err != nil {
		if os.IsNotExist(err) {
			return storj.Key{}, Error.Wrap(fmt.Errorf("not found key file %q", encCfg.KeyFilepath))
		}

		return storj.Key{}, errs.Wrap(err)
	}

	fi, err := f.Stat()
	if err != nil {
		return storj.Key{}, errs.Wrap(err)
	}

	if p := fi.Mode().Perm(); p != os.FileMode(0400) {
		return storj.Key{}, Error.Wrap(
			fmt.Errorf("permissions '%#o' for key file %q are too open", p, encCfg.KeyFilepath),
		)
	}

	var key storj.Key
	if _, err := f.Read(key[:]); err != nil && err != io.EOF {
		return storj.Key{}, errs.Wrap(err)
	}

	return key, nil
}

// ClientConfig is a configuration struct for the uplink that controls how
// to talk to the rest of the network.
type ClientConfig struct {
	APIKey        string        `default:"" help:"the api key to use for the satellite" noprefix:"true"`
	SatelliteAddr string        `releaseDefault:"127.0.0.1:7777" devDefault:"127.0.0.1:10000" help:"the address to use for the satellite" noprefix:"true"`
	MaxInlineSize memory.Size   `help:"max inline segment size in bytes" default:"4KiB"`
	SegmentSize   memory.Size   `help:"the size of a segment in bytes" default:"64MiB"`
	Timeout       time.Duration `help:"timeout for request" default:"0h0m20s"`
}

// Config uplink configuration
type Config struct {
	Client ClientConfig
	RS     RSConfig
	Enc    EncryptionConfig
	TLS    tlsopts.Config
}

var (
	mon = monkit.Package()

	// Error is the errs class of standard End User Client errors
	Error = errs.Class("Uplink configuration error")
)

// GetMetainfo returns an implementation of storj.Metainfo
func (c Config) GetMetainfo(ctx context.Context, identity *identity.FullIdentity) (db storj.Metainfo, ss streams.Store, err error) {
	defer mon.Task()(&ctx)(&err)

	tlsOpts, err := tlsopts.NewOptions(identity, c.TLS)
	if err != nil {
		return nil, nil, err
	}

	// ToDo: Handle Versioning for Uplinks here

	tc := transport.NewClientWithTimeout(tlsOpts, c.Client.Timeout)

	if c.Client.SatelliteAddr == "" {
		return nil, nil, errors.New("satellite address not specified")
	}

	metainfo, err := metainfo.NewClient(ctx, tc, c.Client.SatelliteAddr, c.Client.APIKey)
	if err != nil {
		return nil, nil, Error.New("failed to connect to metainfo service: %v", err)
	}

	ec := ecclient.NewClient(tc, c.RS.MaxBufferMem.Int())
	fc, err := infectious.NewFEC(c.RS.MinThreshold, c.RS.MaxThreshold)
	if err != nil {
		return nil, nil, Error.New("failed to create erasure coding client: %v", err)
	}
	rs, err := eestream.NewRedundancyStrategy(eestream.NewRSScheme(fc, c.RS.ErasureShareSize.Int()), c.RS.RepairThreshold, c.RS.SuccessThreshold)
	if err != nil {
		return nil, nil, Error.New("failed to create redundancy strategy: %v", err)
	}

	maxEncryptedSegmentSize, err := encryption.CalcEncryptedSize(c.Client.SegmentSize.Int64(), c.GetEncryptionScheme())
	if err != nil {
		return nil, nil, Error.New("failed to calculate max encrypted segment size: %v", err)
	}
	segments := segments.NewSegmentStore(metainfo, ec, rs, c.Client.MaxInlineSize.Int(), maxEncryptedSegmentSize)

	if c.RS.ErasureShareSize.Int()*c.RS.MinThreshold%c.Enc.BlockSize.Int() != 0 {
		err = Error.New("EncryptionBlockSize must be a multiple of ErasureShareSize * RS MinThreshold")
		return nil, nil, err
	}

	key, err := c.Enc.Key()
	if err != nil {
		return nil, nil, err
	}

	streams, err := streams.NewStreamStore(segments, c.Client.SegmentSize.Int64(), &key, c.Enc.BlockSize.Int(), storj.Cipher(c.Enc.DataType))
	if err != nil {
		return nil, nil, Error.New("failed to create stream store: %v", err)
	}

	buckets := buckets.NewStore(streams)

	return kvmetainfo.New(metainfo, buckets, streams, segments, &key, c.Enc.BlockSize.Int32(), rs, c.Client.SegmentSize.Int64()), streams, nil
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
