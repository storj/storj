// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package testplanet

import (
	"io/ioutil"
	"strings"
	"time"

	"github.com/zeebo/errs"

	"storj.io/common/memory"
	"storj.io/common/peertls/tlsopts"
	"storj.io/common/storj"
	libuplink "storj.io/storj/lib/uplink"
	"storj.io/uplink"
)

// RSConfig is a configuration struct that keeps details about default
// redundancy strategy information
type RSConfig struct {
	MaxBufferMem     memory.Size `help:"maximum buffer memory (in bytes) to be allocated for read buffers" default:"4MiB" hidden:"true"`
	ErasureShareSize memory.Size `help:"the size of each new erasure share in bytes" default:"256B" hidden:"true"`
	MinThreshold     int         `help:"the minimum pieces required to recover a segment. k." releaseDefault:"29" devDefault:"4" hidden:"true"`
	RepairThreshold  int         `help:"the minimum safe pieces before a repair is triggered. m." releaseDefault:"35" devDefault:"6" hidden:"true"`
	SuccessThreshold int         `help:"the desired total pieces for a segment. o." releaseDefault:"80" devDefault:"8" hidden:"true"`
	MaxThreshold     int         `help:"the largest amount of pieces to encode to. n." releaseDefault:"110" devDefault:"10" hidden:"true"`
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

// UplinkConfig uplink configuration
type UplinkConfig struct {
	AccessConfig
	Client ClientConfig
	RS     RSConfig
	Enc    EncryptionConfig
	TLS    tlsopts.Config
}

// AccessConfig holds information about which accesses exist and are selected.
type AccessConfig struct {
	Accesses map[string]string `internal:"true"`
	Access   string            `help:"the serialized access, or name of the access to use" default:"" basic-help:"true"`

	// used for backward compatibility
	Scopes map[string]string `internal:"true"` // deprecated
	Scope  string            `internal:"true"` // deprecated

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

// normalize looks for usage of deprecated config values and sets the respective
// non-deprecated config values accordingly and returns them in a copy of the config.
func (a AccessConfig) normalize() (_ AccessConfig) {
	// fallback to scope if access not found
	if a.Access == "" {
		a.Access = a.Scope
	}

	if a.Accesses == nil {
		a.Accesses = make(map[string]string)
	}

	// fallback to scopes if accesses not found
	if len(a.Accesses) == 0 {
		for name, access := range a.Scopes {
			a.Accesses[name] = access
		}
	}

	return a
}

// GetAccess returns the appropriate access for the config.
func (a AccessConfig) GetAccess() (_ *libuplink.Scope, err error) {
	a = a.normalize()

	access, err := a.GetNamedAccess(a.Access)
	if err != nil {
		return nil, err
	}
	if access != nil {
		return access, nil
	}

	// Otherwise, try to load the access name as a serialized access.
	if access, err := libuplink.ParseScope(a.Access); err == nil {
		return access, nil
	}

	// fall back to trying to load the legacy values.
	apiKey, err := libuplink.ParseAPIKey(a.Legacy.Client.APIKey)
	if err != nil {
		return nil, err
	}

	satelliteAddr := a.Legacy.Client.SatelliteAddr
	if satelliteAddr == "" {
		return nil, errs.New("must specify a satellite address")
	}

	var encAccess *libuplink.EncryptionAccess
	if a.Legacy.Enc.EncAccessFilepath != "" {
		data, err := ioutil.ReadFile(a.Legacy.Enc.EncAccessFilepath)
		if err != nil {
			return nil, errs.Wrap(err)
		}
		encAccess, err = libuplink.ParseEncryptionAccess(strings.TrimSpace(string(data)))
		if err != nil {
			return nil, err
		}
	} else {
		data := []byte(a.Legacy.Enc.EncryptionKey)
		if a.Legacy.Enc.KeyFilepath != "" {
			data, err = ioutil.ReadFile(a.Legacy.Enc.KeyFilepath)
			if err != nil {
				return nil, errs.Wrap(err)
			}
		}
		key, err := storj.NewKey(data)
		if err != nil {
			return nil, errs.Wrap(err)
		}
		encAccess = libuplink.NewEncryptionAccessWithDefaultKey(*key)
		encAccess.SetDefaultPathCipher(storj.EncAESGCM)
	}

	return &libuplink.Scope{
		APIKey:           apiKey,
		SatelliteAddr:    satelliteAddr,
		EncryptionAccess: encAccess,
	}, nil
}

// GetNewAccess returns the appropriate access for the config.
func (a AccessConfig) GetNewAccess() (_ *uplink.Access, err error) {
	oldAccess, err := a.GetAccess()
	if err != nil {
		return nil, err
	}

	serializedOldAccess, err := oldAccess.Serialize()
	if err != nil {
		return nil, err
	}

	access, err := uplink.ParseAccess(serializedOldAccess)
	if err != nil {
		return nil, err
	}
	return access, nil
}

// GetNamedAccess returns named access if exists.
func (a AccessConfig) GetNamedAccess(name string) (_ *libuplink.Scope, err error) {
	// if an access exists for that name, try to load it.
	if data, ok := a.Accesses[name]; ok {
		return libuplink.ParseScope(data)
	}
	return nil, nil
}

// GetRedundancyScheme returns the configured redundancy scheme for new uploads
func (c UplinkConfig) GetRedundancyScheme() storj.RedundancyScheme {
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
func (c UplinkConfig) GetPathCipherSuite() storj.CipherSuite {
	return storj.CipherSuite(c.Enc.PathType)
}

// GetEncryptionParameters returns the configured encryption scheme for new uploads
// Blocksize should align with the stripe size therefore multiples of stripes
// should fit in every encryption block. Instead of lettings users configure this
// multiple value, we hardcode stripesPerBlock as 2 for simplicity.
func (c UplinkConfig) GetEncryptionParameters() storj.EncryptionParameters {
	const stripesPerBlock = 2
	return storj.EncryptionParameters{
		CipherSuite: storj.CipherSuite(c.Enc.DataType),
		BlockSize:   c.GetRedundancyScheme().StripeSize() * stripesPerBlock,
	}
}

// GetSegmentSize returns the segment size set in uplink config
func (c UplinkConfig) GetSegmentSize() memory.Size {
	return c.Client.SegmentSize
}
