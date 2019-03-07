// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package trust

import (
	"context"

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
	panic("TODO")
}

func (pool *Pool) VerifySignatureWithID(ctx context.Context, data, signature []byte, id storj.NodeID) error {
	// lookup peer identity with id
	// then call VerifySignature(ctx, data, signature, peer)
	panic("TODO")
}

func (pool *Pool) VerifySignature(ctx context.Context, data, signature []byte, peer *identity.PeerIdentity) error {
	panic("TODO")
}
