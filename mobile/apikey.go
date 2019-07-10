// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package mobile

import (
	"time"

	libuplink "storj.io/storj/lib/uplink"
	"storj.io/storj/pkg/macaroon"
)

// Caveat TODO
type Caveat struct {
	DisallowReads   bool
	DisallowWrites  bool
	DisallowLists   bool
	DisallowDeletes bool
	AllowedPaths    []*CaveatPath
	// if set, the validity time window
	NotAfter  int64
	NotBefore int64
	// nonce is set to some random bytes so that you can make arbitrarily
	// many restricted macaroons with the same (or no) restrictions.
	Nonce []byte
}

// CaveatPath If any entries exist, require all access to happen in at least
// one of them.
type CaveatPath struct {
	Bucket              []byte
	EncryptedPathPrefix []byte
}

// NewCaveat TODO
func NewCaveat() *Caveat {
	return &Caveat{
		AllowedPaths: make([]*CaveatPath, 0),
	}
}

// AddCaveatPath TODO
func (caveat Caveat) AddCaveatPath(path *CaveatPath) {
	caveat.AllowedPaths = append(caveat.AllowedPaths, path)
}

// APIKey represents an access credential to certain resources
type APIKey struct {
	lib *libuplink.APIKey
}

// Serialize serializes the API key to a string
func (a APIKey) Serialize() string {
	return a.lib.Serialize()
}

// IsZero returns if the api key is an uninitialized value
func (a *APIKey) IsZero() bool {
	return a.IsZero()
}

// ParseAPIKey parses an API key
func ParseAPIKey(val string) (*APIKey, error) {
	k, err := libuplink.ParseAPIKey(val)
	if err != nil {
		return nil, safeError(err)
	}
	return &APIKey{lib: &k}, nil
}

// Restrict generates a new APIKey with the provided Caveat attached.
func (a APIKey) Restrict(caveat *Caveat) (*APIKey, error) {
	paths := make([]*macaroon.Caveat_Path, 0)
	for _, path := range caveat.AllowedPaths {
		paths = append(paths, &macaroon.Caveat_Path{
			Bucket:              path.Bucket,
			EncryptedPathPrefix: path.EncryptedPathPrefix,
		})
	}
	libCaveat := macaroon.Caveat{
		DisallowReads:   caveat.DisallowReads,
		DisallowWrites:  caveat.DisallowWrites,
		DisallowLists:   caveat.DisallowLists,
		DisallowDeletes: caveat.DisallowDeletes,
		AllowedPaths:    paths,
		Nonce:           caveat.Nonce,
	}

	if caveat.NotAfter != 0 {
		notAfter := time.Unix(caveat.NotAfter, 0)
		libCaveat.NotAfter = &notAfter
	}
	if caveat.NotBefore != 0 {
		notBefore := time.Unix(caveat.NotBefore, 0)
		libCaveat.NotBefore = &notBefore
	}

	k, err := a.lib.Restrict(libCaveat)
	if err != nil {
		return nil, safeError(err)
	}
	return &APIKey{lib: &k}, nil
}
