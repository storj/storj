// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package uplink

import (
	"storj.io/storj/pkg/macaroon"
)

// APIKey represents an access credential to certain resources
type APIKey struct {
	key *macaroon.APIKey
}

// Serialize serializes the API key to a string
func (a APIKey) Serialize() string {
	return a.key.Serialize()
}

// ParseAPIKey parses an API key
func ParseAPIKey(val string) (APIKey, error) {
	k, err := macaroon.ParseAPIKey(val)
	if err != nil {
		return APIKey{}, err
	}
	return APIKey{key: k}, nil
}

// Restrict generates a new APIKey with the provided Caveat attached.
func (a APIKey) Restrict(caveat macaroon.Caveat) (APIKey, error) {
	k, err := a.key.Restrict(caveat)
	if err != nil {
		return APIKey{}, err
	}
	return APIKey{key: k}, nil
}
