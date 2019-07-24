// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package uplink

import (
	"time"

	"storj.io/storj/internal/memory"
	"storj.io/storj/pkg/peertls/tlsopts"
	"storj.io/storj/pkg/storj"
)

// RSConfig is a configuration struct that keeps details about default
// redundancy strategy information
type RSConfig struct {
	MaxBufferMem     memory.Size `help:"maximum buffer memory (in bytes) to be allocated for read buffers" default:"4MiB" hidden:"true"`
	ErasureShareSize memory.Size `help:"the size of each new erasure share in bytes" default:"256B" hidden:"true"`
	MinThreshold     int         `help:"the minimum pieces required to recover a segment. k." releaseDefault:"29" devDefault:"4" hidden:"true"`
	RepairThreshold  int         `help:"the minimum safe pieces before a repair is triggered. m." releaseDefault:"35" devDefault:"6" hidden:"true"`
	SuccessThreshold int         `help:"the desired total pieces for a segment. o." releaseDefault:"80" devDefault:"8" hidden:"true"`
	MaxThreshold     int         `help:"the largest amount of pieces to encode to. n." releaseDefault:"130" devDefault:"10" hidden:"true"`
}

// EncryptionConfig is a configuration struct that keeps details about
// encrypting segments
type EncryptionConfig struct {
	EncryptionKey     string `help:"the root key for encrypting the data which will be stored in KeyFilePath" setup:"true"`
	KeyFilepath       string `help:"the path to the file which contains the root key for encrypting the data"`
	EncAccessFilepath string `help:"the path to a file containing a serialized encryption access"`
	DataType          int    `help:"Type of encryption to use for content and metadata (2=AES-GCM, 3=SecretBox)" default:"2"`
	PathType          int    `help:"Type of encryption to use for paths (1=Unencrypted, 2=AES-GCM, 3=SecretBox)" default:"2"`
}

// ClientConfig is a configuration struct for the uplink that controls how
// to talk to the rest of the network.
type ClientConfig struct {
	APIKey         string        `default:"" help:"the api key to use for the satellite" noprefix:"true"`
	SatelliteAddr  string        `releaseDefault:"127.0.0.1:7777" devDefault:"127.0.0.1:10000" help:"the address to use for the satellite" noprefix:"true"`
	MaxInlineSize  memory.Size   `help:"max inline segment size in bytes" default:"4KiB"`
	SegmentSize    memory.Size   `help:"the size of a segment in bytes" default:"64MiB"`
	RequestTimeout time.Duration `help:"timeout for request" default:"0h2m00s"`
	DialTimeout    time.Duration `help:"timeout for dials" default:"0h2m00s"`
}

// Config uplink configuration
type Config struct {
	Client ClientConfig
	RS     RSConfig
	Enc    EncryptionConfig
	TLS    tlsopts.Config
}

// GetRedundancyScheme returns the configured redundancy scheme for new uploads
func (c Config) GetRedundancyScheme() storj.RedundancyScheme {
	return storj.RedundancyScheme{
		Algorithm:      storj.ReedSolomon,
		ShareSize:      c.RS.ErasureShareSize.Int32(),
		RequiredShares: int16(c.RS.MinThreshold),
		RepairShares:   int16(c.RS.RepairThreshold),
		OptimalShares:  int16(c.RS.SuccessThreshold),
		TotalShares:    int16(c.RS.MaxThreshold),
	}
}

// GetPathCipherSuite returns the cipher suite used for path encryption for bucket objects
func (c Config) GetPathCipherSuite() storj.CipherSuite {
	return storj.CipherSuite(c.Enc.PathType)
}

// GetEncryptionParameters returns the configured encryption scheme for new uploads
// Blocksize should align with the stripe size therefore multiples of stripes
// should fit in every encryption block. Instead of lettings users configure this
// multiple value, we hardcode stripesPerBlock as 2 for simplicity.
func (c Config) GetEncryptionParameters() storj.EncryptionParameters {
	const stripesPerBlock = 2
	return storj.EncryptionParameters{
		CipherSuite: storj.CipherSuite(c.Enc.DataType),
		BlockSize:   c.GetRedundancyScheme().StripeSize() * stripesPerBlock,
	}
}

// GetSegmentSize returns the segment size set in uplink config
func (c Config) GetSegmentSize() memory.Size {
	return c.Client.SegmentSize
}
