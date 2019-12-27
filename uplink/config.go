// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package uplink

import (
	"io/ioutil"
	"strings"
	"time"

	"github.com/zeebo/errs"
	"gopkg.in/spacemonkeygo/monkit.v2"

	"storj.io/common/memory"
	"storj.io/common/peertls/tlsopts"
	"storj.io/common/storj"
	libuplink "storj.io/storj/lib/uplink"
)

var mon = monkit.Package()

// RSConfig is a configuration struct that keeps details about default
// redundancy strategy information
type RSConfig struct {
	MaxBufferMem     memory.Size `help:"maximum buffer memory (in bytes) to be allocated for read buffers" default:"4MiB" hidden:"true"`
	ErasureShareSize memory.Size `help:"the size of each new erasure share in bytes" default:"256B" hidden:"true"`
	MinThreshold     int         `help:"the minimum pieces required to recover a segment. k." releaseDefault:"29" devDefault:"4" hidden:"true"`
	RepairThreshold  int         `help:"the minimum safe pieces before a repair is triggered. m." releaseDefault:"35" devDefault:"6" hidden:"true"`
	SuccessThreshold int         `help:"the desired total pieces for a segment. o." releaseDefault:"80" devDefault:"8" hidden:"true"`
	MaxThreshold     int         `help:"the largest amount of pieces to encode to. n." releaseDefault:"95" devDefault:"10" hidden:"true"`
}

// EncryptionConfig is a configuration struct that keeps details about
// encrypting segments
type EncryptionConfig struct {
	DataType int `help:"Type of encryption to use for content and metadata (2=AES-GCM, 3=SecretBox)" default:"2"`
	PathType int `help:"Type of encryption to use for paths (1=Unencrypted, 2=AES-GCM, 3=SecretBox)" default:"2"`
}

// ClientConfig is a configuration struct for the uplink that controls how
// to talk to the rest of the network.
type ClientConfig struct {
	MaxInlineSize memory.Size   `help:"max inline segment size in bytes" default:"4KiB"`
	SegmentSize   memory.Size   `help:"the size of a segment in bytes" default:"64MiB"`
	DialTimeout   time.Duration `help:"timeout for dials" default:"0h2m00s"`
}

// Config uplink configuration
type Config struct {
	ScopeConfig
	Client ClientConfig
	RS     RSConfig
	Enc    EncryptionConfig
	TLS    tlsopts.Config
}

// ScopeConfig holds information about which scopes exist and are selected.
type ScopeConfig struct {
	Scopes map[string]string `internal:"true"`
	Scope  string            `help:"the serialized scope, or name of the scope to use" default:""`

	Legacy // Holds on to legacy configuration values
}

// Legacy holds deprecated configuration values
type Legacy struct {
	Client struct {
		APIKey        string `default:"" help:"the api key to use for the satellite (deprecated)" noprefix:"true" deprecated:"true"`
		SatelliteAddr string `releaseDefault:"127.0.0.1:7777" devDefault:"127.0.0.1:10000" help:"the address to use for the satellite (deprecated)" noprefix:"true"`
	}
	Enc struct {
		EncryptionKey     string `help:"the root key for encrypting the data which will be stored in KeyFilePath (deprecated)" setup:"true" deprecated:"true"`
		KeyFilepath       string `help:"the path to the file which contains the root key for encrypting the data (deprecated)" deprecated:"true"`
		EncAccessFilepath string `help:"the path to a file containing a serialized encryption access (deprecated)" deprecated:"true"`
	}
}

// GetScope returns the appropriate scope for the config.
func (c ScopeConfig) GetScope() (_ *libuplink.Scope, err error) {
	defer mon.Task()(nil)(&err)

	// if a scope exists for that name, try to load it.
	if data, ok := c.Scopes[c.Scope]; ok && c.Scope != "" {
		return libuplink.ParseScope(data)
	}

	// Otherwise, try to load the scope name as a serialized scope.
	if scope, err := libuplink.ParseScope(c.Scope); err == nil {
		return scope, nil
	}

	// fall back to trying to load the legacy values.
	apiKey, err := libuplink.ParseAPIKey(c.Legacy.Client.APIKey)
	if err != nil {
		return nil, err
	}

	satelliteAddr := c.Legacy.Client.SatelliteAddr
	if satelliteAddr == "" {
		return nil, errs.New("must specify a satellite address")
	}

	var encAccess *libuplink.EncryptionAccess
	if c.Legacy.Enc.EncAccessFilepath != "" {
		data, err := ioutil.ReadFile(c.Legacy.Enc.EncAccessFilepath)
		if err != nil {
			return nil, errs.Wrap(err)
		}
		encAccess, err = libuplink.ParseEncryptionAccess(strings.TrimSpace(string(data)))
		if err != nil {
			return nil, err
		}
	} else {
		data := []byte(c.Legacy.Enc.EncryptionKey)
		if c.Legacy.Enc.KeyFilepath != "" {
			data, err = ioutil.ReadFile(c.Legacy.Enc.KeyFilepath)
			if err != nil {
				return nil, errs.Wrap(err)
			}
		}
		key, err := storj.NewKey(data)
		if err != nil {
			return nil, errs.Wrap(err)
		}
		encAccess = libuplink.NewEncryptionAccessWithDefaultKey(*key)
	}

	return &libuplink.Scope{
		APIKey:           apiKey,
		SatelliteAddr:    satelliteAddr,
		EncryptionAccess: encAccess,
	}, nil
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
