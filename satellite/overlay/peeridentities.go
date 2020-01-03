// Copyright (C) 2018 Storj Labs, Inc.
// See LICENSE for copying information.

package overlay

import (
	"context"

	"storj.io/common/identity"
	"storj.io/common/storj"
)

// PeerIdentities stores storagenode peer identities
//
// architecture: Database
type PeerIdentities interface {
	// Set adds a peer identity entry for a node
	Set(context.Context, storj.NodeID, *identity.PeerIdentity) error
	// Get gets peer identity
	Get(context.Context, storj.NodeID) (*identity.PeerIdentity, error)
	// BatchGet gets all nodes peer identities in a transaction
	BatchGet(context.Context, storj.NodeIDList) ([]*identity.PeerIdentity, error)
}
