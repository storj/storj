// Copyright (C) 2018 Storj Labs, Inc.
// See LICENSE for copying information.

package uplink

// APIKey is an interface for authenticating with the Satellite
type APIKey interface {
	Serialize() ([]byte, error)
}
