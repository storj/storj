// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package trust

import (
	"context"

	"storj.io/storj/pkg/identity"
)

// CertDB is a database of peer identities.
type CertDB interface {
	Include(ctx context.Context, pi *identity.PeerIdentity) (certid int64, err error)
	LookupByCertID(ctx context.Context, id int64) (*identity.PeerIdentity, error)
}
