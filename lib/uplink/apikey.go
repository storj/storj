// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package uplink

// APIKey represents an access credential to certain resources
type APIKey struct {
	key string
}

// Serialize serializes the API key to a string
func (a APIKey) Serialize() string {
	return a.key
}

// ParseAPIKey parses an API key
func ParseAPIKey(val string) (APIKey, error) {
	return APIKey{key: val}, nil
}
