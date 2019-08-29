// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package repairer

import (
	"context"
	"io"
	"time"

	"storj.io/storj/pkg/pb"
	"storj.io/storj/pkg/storj"
	"storj.io/storj/uplink/eestream"
)

// ECRepairer allows the repairer to download, verify, and upload pieces from storagenodes.
type ECRepairer struct {
}

// NewECRepairer creates a new repairer for interfacing with storagenodes.
func NewECRepairer() *ECRepairer {
	return &ECRepairer{}
}

// Get downloads pieces from storagenodes using the provided order limits, and decodes those pieces into a segment.
// It attempts to download from the minimum required number based on the redundancy scheme.
// After downloading a piece, the ECRepairer will verify the hash and original order limit for that piece.
// If verification fails, another piece will be downloaded until we reach the minimum required or run out of order limits.
func (*ECRepairer) Get(ctx context.Context, limits []*pb.AddressedOrderLimit, privateKey storj.PiecePrivateKey, es eestream.ErasureScheme, size int64) (data io.ReadCloser, err error) {
	defer mon.Task()(&ctx)(&err)

	// TODO (async) start a number of workers equal to the minimum number of required pieces
	// TODO for each worker, select an unused order limit and attempt to run downloadAndVerifyPiece
	// TODO if there is an error, select another unused order limit and re-attempt
	// TODO if at any point there is a failure and there are no more unused order limits, cancel all workers and return an error

	// TODO wait until context canceled, timeout expired, or we have downloaded and verified the minimum number of pieces
	// TODO decode pieces into a segment and return

	return nil, nil
}

// downloadAndVerifyPiece downloads a piece from a storagenode,
// expects the original order limit to have the correct piece public key,
// and expects the hash of the data to match the signed hash provided by the storagenode.
func (*ECRepairer) downloadAndVerifyPiece() (data io.ReadCloser, err error) {
	// TODO download piece and calculate hash (stop early if ctx canceled)
	// TODO get order limit and hash
	// TODO verify order limit validity
	// TODO verify hash validity
	return nil, nil
}

// Repair takes a provided segment, encodes it with the prolvided redundancy strategy,
// and uploads the pieces in need of repair to new nodes provided by order limits.
func (ec *ECRepairer) Repair(ctx context.Context, limits []*pb.AddressedOrderLimit, privateKey storj.PiecePrivateKey, rs eestream.RedundancyStrategy, data io.Reader, expiration time.Time, timeout time.Duration, path storj.Path) (successfulNodes []*pb.Node, successfulHashes []*pb.PieceHash, err error) {
	defer mon.Task()(&ctx)(&err)

	// TODO should be almost identical to ecclient Repair (remove ecclient Repair once implemented here)

	return nil, nil, nil
}
