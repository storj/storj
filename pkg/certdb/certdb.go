// Copyright (C) 2018 Storj Labs, Inc.
// See LICENSE for copying information.

package certdb

import (
	"context"
	"crypto"

	"storj.io/storj/pkg/storj"
)

// DB stores uplink public keys.
type DB interface {
	// SavePublicKey adds a new bandwidth agreement.
	SavePublicKey(context.Context, storj.NodeID, crypto.PublicKey) error
	// GetPublicKey gets the public key of uplink corresponding to uplink id
	GetPublicKey(context.Context, storj.NodeID) (crypto.PublicKey, error)
}
