// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package trust

import (
	"context"

	"storj.io/storj/pkg/auth/signing"
	"storj.io/storj/pkg/identity"
	"storj.io/storj/pkg/storj"
)

type Pool struct {
	trustedSatellites map[storj.NodeID]struct{}

	cache map[storj.NodeID]*identity.PeerIdentity
}

func (pool *Pool) VerifySatelliteID(ctx context.Context, id storj.NodeID) error {
	panic("TODO")
}

func (pool *Pool) VerifyUplinkID(ctx context.Context, id storj.NodeID) error {
	// trusting all the uplinks for now
	return nil
}

func (pool *Pool) GetSignee(ctx context.Context, id storj.NodeID) (signing.Signee, error) {
	// lookup peer identity with id
	// then call VerifySignature(ctx, data, signature, peer)
	panic("TODO")
}
