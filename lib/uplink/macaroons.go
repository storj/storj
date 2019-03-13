// Copyright (C) 2018 Storj Labs, Inc.
// See LICENSE for copying information.

package uplink

import "errors"

// A Macaroon represents an access credential to certain resources
type Macaroon interface {
	Serialize() ([]byte, error)
	Restrict(caveats ...Caveat) Macaroon
}

// Caveat could be a read-only restriction, a time-bound
// restriction, a bucket-specific restriction, a path-prefix restriction, a
// full path restriction, etc.
type Caveat interface {
}

// ParseAccess parses a serialized Access
func ParseAccess(data []byte) (Access, error) {
	return Access{}, errors.New("not implemented")
}

// Serialize serializes an Access message
func (a *Access) Serialize() ([]byte, error) {
	return []byte{}, errors.New("not implemented")
}
