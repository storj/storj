// Copyright (C) 2020 Storj Labs, Inc.
// See LICENSE for copying information.

package nodeapiversion

import (
	"context"

	"storj.io/common/storj"
)

// Version represents a node api version.
type Version int

// These constants describe versions of satellite APIs. You should add one when
// you are creating a feature that needs "ratcheting" behavior, meaning the node
// should no longer be allowed to use an old API after it has started using a
// new API. Later constants always imply earlier constants.
const (
	// HasAnything is the base case that every node will have.
	HasAnything Version = iota
	HasWindowedOrders
)

// DB is the interface to interact with the node api version database.
type DB interface {
	// UpdateVersionAtLeast sets the node version to be at least the passed in version.
	// Any existing entry for the node will never have the version decreased.
	UpdateVersionAtLeast(ctx context.Context, id storj.NodeID, version Version) error

	// VersionAtLeast returns true iff the recorded node version is greater than or equal
	// to the passed in version. VersionAtLeast always returns true if the passed in version
	// is HasAnything.
	VersionAtLeast(ctx context.Context, id storj.NodeID, version Version) (bool, error)
}
