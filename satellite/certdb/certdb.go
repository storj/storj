// Copyright (C) 2018 Storj Labs, Inc.
// See LICENSE for copying information.

package certdb

import (
	"context"

	"storj.io/storj/pkg/identity"
	"storj.io/storj/pkg/storj"
)

// DB stores uplink public keys.
type DB interface {
	// Set adds a new bandwidth agreement.
	Set(context.Context, storj.NodeID, *identity.PeerIdentity) error
	// Get gets one latest public key of a node
	Get(context.Context, storj.NodeID) (*identity.PeerIdentity, error)
}
