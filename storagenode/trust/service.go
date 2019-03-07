// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package piecestore

import (
	"context"

	"storj.io/storj/pkg/storj"
)

// Implementation for...

type Trust interface {
	VerifySatellite(context.Context, storj.NodeID) error
	VerifyUplink(context.Context, storj.NodeID) error
	VerifySignature(context.Context, []byte, storj.NodeID) error

	// check what's needed in piecestore
}
