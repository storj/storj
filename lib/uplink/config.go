// Copyright (C) 2018 Storj Labs, Inc.
// See LICENSE for copying information.

package uplink

// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

import (
	"context"

	"storj.io/storj/internal/memory"
	"storj.io/storj/pkg/identity"
	"storj.io/storj/pkg/peertls/tlsopts"
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
type config struct {
	Client ClientConfig
	RS     RSConfig
	Enc    EncryptionConfig
	TLS    tlsopts.Config
}

// GetMetainfo returns an implementation of storj.Metainfo
func (c config) getMetaInfo(ctx context.Context, identity *identity.FullIdentity) (db storj.Metainfo, ss streams.Store, err error) {
	defer mon.Task()(&ctx)(&err)
	return c.getMetaInfo(ctx, identity)
}

// GetRedundancyScheme returns the configured redundancy scheme for new uploads
func (c config) GetRedundancyScheme() storj.RedundancyScheme {
	return storj.RedundancyScheme{
		Algorithm:      storj.ReedSolomon,
		RequiredShares: int16(c.RS.MinThreshold),
		RepairShares:   int16(c.RS.RepairThreshold),
		OptimalShares:  int16(c.RS.SuccessThreshold),
		TotalShares:    int16(c.RS.MaxThreshold),
	}
}

// GetEncryptionScheme returns the configured encryption scheme for new uploads
func (c config) GetEncryptionScheme() storj.EncryptionScheme {
	return storj.EncryptionScheme{
		Cipher:    storj.Cipher(c.Enc.DataType),
		BlockSize: int32(c.Enc.BlockSize),
	}
}
