// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package uplink

import (
	"storj.io/common/macaroon"
)

// APIKey represents an access credential to certain resources
type APIKey struct {
	key *macaroon.APIKey
}

// Serialize serializes the API key to a string
func (a APIKey) Serialize() string {
	return a.key.Serialize()
}

func (a APIKey) serializeRaw() []byte {
	return a.key.SerializeRaw()
}

// IsZero returns if the api key is an uninitialized value
func (a *APIKey) IsZero() bool {
	return a.key == nil
}

// ParseAPIKey parses an API key
func ParseAPIKey(val string) (APIKey, error) {
	k, err := macaroon.ParseAPIKey(val)
	if err != nil {
		return APIKey{}, err
	}
	return APIKey{key: k}, nil
}

func parseRawAPIKey(data []byte) (APIKey, error) {
	k, err := macaroon.ParseRawAPIKey(data)
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
