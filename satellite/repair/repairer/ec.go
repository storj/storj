// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package repairer

import (
	"bytes"
	"context"
	"io"
	"time"

	"go.uber.org/zap"

	"storj.io/storj/pkg/pb"
	"storj.io/storj/pkg/pkcrypto"
	"storj.io/storj/pkg/signing"
	"storj.io/storj/pkg/storj"
	"storj.io/storj/pkg/transport"
	"storj.io/storj/uplink/eestream"
	"storj.io/storj/uplink/piecestore"
)

// ECRepairer allows the repairer to download, verify, and upload pieces from storagenodes.
type ECRepairer struct {
	log             *zap.Logger
	transport       transport.Client
	satelliteSignee signing.Signee
}

// NewECRepairer creates a new repairer for interfacing with storagenodes.
func NewECRepairer(log *zap.Logger, tc transport.Client, satelliteSignee signing.Signee) *ECRepairer {
	return &ECRepairer{
		log:             log,
		transport:       tc,
		satelliteSignee: satelliteSignee,
	}
}

func (ec *ECRepairer) dialPiecestore(ctx context.Context, n *pb.Node) (*piecestore.Client, error) {
	logger := ec.log.Named(n.Id.String())
	return piecestore.Dial(ctx, ec.transport, n, logger, piecestore.DefaultConfig)
}

// Get downloads pieces from storagenodes using the provided order limits, and decodes those pieces into a segment.
// It attempts to download from the minimum required number based on the redundancy scheme.
// After downloading a piece, the ECRepairer will verify the hash and original order limit for that piece.
// If verification fails, another piece will be downloaded until we reach the minimum required or run out of order limits.
func (ec *ECRepairer) Get(ctx context.Context, limits []*pb.AddressedOrderLimit, privateKey storj.PiecePrivateKey, es eestream.ErasureScheme, size int64) (data io.ReadCloser, err error) {
	defer mon.Task()(&ctx)(&err)

	if len(limits) != es.TotalCount() {
		return nil, Error.New("size of limits slice (%d) does not match total count (%d) of erasure scheme", len(limits), es.TotalCount())
	}

	if nonNilCount(limits) < es.RequiredCount() {
		return nil, Error.New("number of non-nil limits (%d) is less than required count (%d) of erasure scheme", nonNilCount(limits), es.RequiredCount())
	}

	paddedSize := calcPadded(size, es.StripeSize())
	pieceSize := paddedSize / int64(es.RequiredCount())

	// temp
	limit := limits[0]
	_, err = ec.downloadAndVerifyPiece(ctx, limit, privateKey, pieceSize)
	if err != nil {
		return nil, err
	}

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
func (ec *ECRepairer) downloadAndVerifyPiece(ctx context.Context, limit *pb.AddressedOrderLimit, privateKey storj.PiecePrivateKey, pieceSize int64) (data io.ReadCloser, err error) {
	// contact node
	ps, err := ec.dialPiecestore(ctx, &pb.Node{
		Id:      limit.GetLimit().StorageNodeId,
		Address: limit.GetStorageNodeAddress(),
	})
	if err != nil {
		return nil, err
	}

	downloader, err := ps.Download(ctx, limit.GetLimit(), privateKey, 0, pieceSize)
	if err != nil {
		return nil, err
	}

	var calculatedHash []byte
	pieceBytes := make([]byte, 0, pieceSize)
	// TODO: figure out the buffer size
	buffer := make([]byte, 1024)
	newHash := pkcrypto.NewHash()

	for {
		// download full piece
		n, readErr := downloader.Read(buffer)
		if readErr == io.EOF {
			calculatedHash = newHash.Sum(buffer[:n])
			break
		}
		if readErr != nil {
			return nil, readErr
		}

		// add new data to hash calculation
		_, _ = newHash.Write(buffer[:n]) // guaranteed not to return an error
		// add new data to piece bytes
		pieceBytes = append(pieceBytes, buffer[:n]...)
	}

	if int64(len(pieceBytes)) != pieceSize {
		return nil, Error.New("didn't download the correct amount of data, want %d, got %d", pieceSize, len(pieceBytes))
	}

	// get signed piece hash and original order limit
	hash, originalLimit := downloader.GetHashAndLimit()
	if hash == nil {
		return nil, Error.New("Hash was not sent from storagenode.")
	}
	if originalLimit == nil {
		return nil, Error.New("Original order limit was not sent from storagenode.")
	}

	// verify the hashes from storage node
	if err := verifyPieceHash(ctx, originalLimit, hash, calculatedHash); err != nil {
		return nil, err
	}

	// verify order limit from storage node is signed by the satellite
	if err := verifyOrderLimitSignature(ctx, ec.satelliteSignee, originalLimit); err != nil {
		return nil, err
	}

	return nil, nil
}

// Repair takes a provided segment, encodes it with the prolvided redundancy strategy,
// and uploads the pieces in need of repair to new nodes provided by order limits.
func (ec *ECRepairer) Repair(ctx context.Context, limits []*pb.AddressedOrderLimit, privateKey storj.PiecePrivateKey, rs eestream.RedundancyStrategy, data io.Reader, expiration time.Time, timeout time.Duration, path storj.Path) (successfulNodes []*pb.Node, successfulHashes []*pb.PieceHash, err error) {
	defer mon.Task()(&ctx)(&err)

	// TODO should be almost identical to ecclient Repair (remove ecclient Repair once implemented here)

	return nil, nil, nil
}

func verifyPieceHash(ctx context.Context, limit *pb.OrderLimit, hash *pb.PieceHash, expectedHash []byte) (err error) {
	defer mon.Task()(&ctx)(&err)

	if limit == nil || hash == nil || len(expectedHash) == 0 {
		return Error.New("invalid arguments")
	}
	if limit.PieceId != hash.PieceId {
		return Error.New("piece id changed")
	}
	if !bytes.Equal(hash.Hash, expectedHash) {
		return Error.New("hashes don't match")
	}

	if err := signing.VerifyUplinkPieceHashSignature(ctx, limit.UplinkPublicKey, hash); err != nil {
		return Error.New("invalid piece hash signature")
	}

	return nil
}

func verifyOrderLimitSignature(ctx context.Context, satellite signing.Signee, limit *pb.OrderLimit) (err error) {
	if err := signing.VerifyOrderLimitSignature(ctx, satellite, limit); err != nil {
		return Error.New("invalid order limit signature: %v", err)
	}

	return nil
}

func calcPadded(size int64, blockSize int) int64 {
	mod := size % int64(blockSize)
	if mod == 0 {
		return size
	}
	return size + int64(blockSize) - mod
}

func nonNilCount(limits []*pb.AddressedOrderLimit) int {
	total := 0
	for _, limit := range limits {
		if limit != nil {
			total++
		}
	}
	return total
}
