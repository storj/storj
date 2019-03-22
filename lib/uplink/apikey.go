// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package uplink

import (
	"time"

	"storj.io/storj/pkg/storj"
)

// APIKey represents an access credential to certain resources
type APIKey struct {
	key string
}

// Serialize serializes the API Key to a string
func (a APIKey) Serialize() string {
	return a.key
}

// ParseAPIKey parses an API Key
func ParseAPIKey(val string) (APIKey, error) {
	return APIKey{key: val}, nil
}

// Share returns a new API Key with the given caveat restrictions added
func (a APIKey) Share(caveats ...Caveat) APIKey {
	panic("TODO")
}

// A caveat restricts access
type Caveat struct {
	DisallowReads  bool
	DisallowWrites bool
	DisallowLists  bool

	Buckets               []string
	EncryptedPathPrefixes []storj.Path

	NotBefore, NotAfter time.Time

	URL string
}
