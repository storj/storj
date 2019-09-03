// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package repairer

import (
	"bytes"
	"context"
	"io"
	"io/ioutil"
	"sort"
	"sync/atomic"
	"time"

	"github.com/vivint/infectious"
	"github.com/zeebo/errs"
	"go.uber.org/zap"

	"storj.io/storj/internal/errs2"
	"storj.io/storj/internal/sync2"
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
func (ec *ECRepairer) Get(ctx context.Context, limits []*pb.AddressedOrderLimit, privateKey storj.PiecePrivateKey, es eestream.ErasureScheme, size int64) (_ io.ReadCloser, err error) {
	defer mon.Task()(&ctx)(&err)

	if len(limits) != es.TotalCount() {
		return nil, Error.New("size of limits slice (%d) does not match total count (%d) of erasure scheme", len(limits), es.TotalCount())
	}

	if nonNilCount(limits) < es.RequiredCount() {
		return nil, Error.New("number of non-nil limits (%d) is less than required count (%d) of erasure scheme", nonNilCount(limits), es.RequiredCount())
	}

	pieceSize := eestream.CalcPieceSize(size, es)

	// TODO: make these steps async so we can download from multiple nodes at the same time

	var successfulPieces, currentLimitIndex int
	shares := make([]infectious.Share, 0, es.RequiredCount())
	for successfulPieces < es.RequiredCount() && currentLimitIndex < len(limits) {
		limit := limits[currentLimitIndex]
		if limit == nil {
			currentLimitIndex++
			continue
		}

		downloadedPiece, err := ec.downloadAndVerifyPiece(ctx, limit, privateKey, pieceSize)
		if err != nil {
			// TODO: add error to a errgroup, return that errgroup if successfulPieces < es.RequiredCount() is true after for loop
			currentLimitIndex++
			continue
		}

		shares = append(shares, infectious.Share{
			Number: currentLimitIndex,
			Data:   downloadedPiece,
		})
		currentLimitIndex++
		successfulPieces++
	}
	if successfulPieces < es.RequiredCount() {
		return nil, Error.New("couldn't download enough pieces, number of successful downloaded pieces (%d) is less than required number (%d)", successfulPieces, es.RequiredCount())
	}

	fec, err := infectious.NewFEC(es.RequiredCount(), es.TotalCount())
	if err != nil {
		return nil, Error.Wrap(err)
	}

	// reconstruct original stripe
	segment, err := rebuildStripe(ctx, fec, shares, int(pieceSize))
	if err != nil {
		return nil, Error.Wrap(err)
	}

	return ioutil.NopCloser(bytes.NewReader(segment)), nil
}

// downloadAndVerifyPiece downloads a piece from a storagenode,
// expects the original order limit to have the correct piece public key,
// and expects the hash of the data to match the signed hash provided by the storagenode.
func (ec *ECRepairer) downloadAndVerifyPiece(ctx context.Context, limit *pb.AddressedOrderLimit, privateKey storj.PiecePrivateKey, pieceSize int64) (data []byte, err error) {
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
	defer func() { err = errs.Combine(err, downloader.Close()) }()

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

	return pieceBytes, nil
}

func rebuildStripe(ctx context.Context, fec *infectious.FEC, shares []infectious.Share, shareSize int) (_ []byte, err error) {
	defer mon.Task()(&ctx)(&err)
	stripe := make([]byte, fec.Required()*shareSize)
	err = fec.Rebuild(shares, func(share infectious.Share) {
		copy(stripe[share.Number*shareSize:], share.Data)
	})
	if err != nil {
		return nil, err
	}
	return stripe, nil
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

// Repair takes a provided segment, encodes it with the prolvided redundancy strategy,
// and uploads the pieces in need of repair to new nodes provided by order limits.
func (ec *ECRepairer) Repair(ctx context.Context, limits []*pb.AddressedOrderLimit, privateKey storj.PiecePrivateKey, rs eestream.RedundancyStrategy, data io.Reader, expiration time.Time, timeout time.Duration, path storj.Path) (successfulNodes []*pb.Node, successfulHashes []*pb.PieceHash, err error) {
	defer mon.Task()(&ctx)(&err)

	pieceCount := len(limits)
	if pieceCount != rs.TotalCount() {
		return nil, nil, Error.New("size of limits slice (%d) does not match total count (%d) of erasure scheme", pieceCount, rs.TotalCount())
	}

	if !unique(limits) {
		return nil, nil, Error.New("duplicated nodes are not allowed")
	}

	// TODO remove commented code; Get() does not unpad the data so we should not need to pad it here
	// padded := eestream.PadReader(ioutil.NopCloser(data), rs.StripeSize())
	readers, err := eestream.EncodeReader(ctx, ec.log, ioutil.NopCloser(data), rs)
	if err != nil {
		return nil, nil, err
	}

	type info struct {
		i    int
		err  error
		hash *pb.PieceHash
	}
	infos := make(chan info, pieceCount)

	psCtx, cancel := context.WithCancel(ctx)
	defer cancel()

	for i, addressedLimit := range limits {
		go func(i int, addressedLimit *pb.AddressedOrderLimit) {
			hash, err := ec.putPiece(psCtx, ctx, addressedLimit, privateKey, readers[i], expiration)
			infos <- info{i: i, err: err, hash: hash}
		}(i, addressedLimit)
	}
	ec.log.Info("Starting a timer for repair so that the number of pieces will be closer to the success threshold",
		zap.Duration("Timer", timeout),
		zap.String("Path", path),
		zap.Int("Node Count", nonNilCount(limits)),
		zap.Int("Optimal Threshold", rs.OptimalThreshold()),
	)

	var successfulCount, failureCount, cancellationCount int32
	timer := time.AfterFunc(timeout, func() {
		if ctx.Err() != context.Canceled {
			ec.log.Info("Timer expired. Canceling the long tail...",
				zap.String("Path", path),
				zap.Int32("Successfully repaired", atomic.LoadInt32(&successfulCount)),
			)
			cancel()
		}
	})

	successfulNodes = make([]*pb.Node, pieceCount)
	successfulHashes = make([]*pb.PieceHash, pieceCount)

	for range limits {
		info := <-infos

		if limits[info.i] == nil {
			continue
		}

		if info.err != nil {
			if !errs2.IsCanceled(info.err) {
				failureCount++
			} else {
				cancellationCount++
			}
			ec.log.Debug("Repair to storage node failed",
				zap.String("Path", path),
				zap.String("NodeID", limits[info.i].GetLimit().StorageNodeId.String()),
				zap.Error(info.err),
			)
			continue
		}

		successfulNodes[info.i] = &pb.Node{
			Id:      limits[info.i].GetLimit().StorageNodeId,
			Address: limits[info.i].GetStorageNodeAddress(),
		}
		successfulHashes[info.i] = info.hash
		successfulCount++
	}

	// Ensure timer is stopped
	_ = timer.Stop()

	// TODO: clean up the partially uploaded segment's pieces
	defer func() {
		select {
		case <-ctx.Done():
			err = Error.New("repair cancelled")
			// ec.Delete(context.Background(), nodes, pieceID, pba.SatelliteId), //TODO
		default:
		}
	}()

	if successfulCount == 0 {
		return nil, nil, Error.New("repair %v to all nodes failed", path)
	}

	ec.log.Info("Successfully repaired",
		zap.String("Path", path),
		zap.Int32("Success Count", atomic.LoadInt32(&successfulCount)),
	)

	mon.IntVal("repair_segment_pieces_total").Observe(int64(pieceCount))
	mon.IntVal("repair_segment_pieces_successful").Observe(int64(successfulCount))
	mon.IntVal("repair_segment_pieces_failed").Observe(int64(failureCount))
	mon.IntVal("repair_segment_pieces_canceled").Observe(int64(cancellationCount))

	return successfulNodes, successfulHashes, nil
}

// TODO limit duplicate code with ecclient

// copied from ecclient
func (ec *ECRepairer) putPiece(ctx, parent context.Context, limit *pb.AddressedOrderLimit, privateKey storj.PiecePrivateKey, data io.ReadCloser, expiration time.Time) (hash *pb.PieceHash, err error) {
	nodeName := "nil"
	if limit != nil {
		nodeName = limit.GetLimit().StorageNodeId.String()[0:8]
	}
	defer mon.Task()(&ctx, "node: "+nodeName)(&err)
	defer func() { err = errs.Combine(err, data.Close()) }()

	if limit == nil {
		_, _ = io.Copy(ioutil.Discard, data)
		return nil, nil
	}

	storageNodeID := limit.GetLimit().StorageNodeId
	pieceID := limit.GetLimit().PieceId
	ps, err := ec.dialPiecestore(ctx, &pb.Node{
		Id:      storageNodeID,
		Address: limit.GetStorageNodeAddress(),
	})
	if err != nil {
		ec.log.Debug("Failed dialing for putting piece to node",
			zap.String("PieceID", pieceID.String()),
			zap.String("NodeID", storageNodeID.String()),
			zap.Error(err),
		)
		return nil, err
	}
	defer func() { err = errs.Combine(err, ps.Close()) }()

	upload, err := ps.Upload(ctx, limit.GetLimit(), privateKey)
	if err != nil {
		ec.log.Debug("Failed requesting upload of pieces to node",
			zap.String("PieceID", pieceID.String()),
			zap.String("NodeID", storageNodeID.String()),
			zap.Error(err),
		)
		return nil, err
	}
	defer func() {
		if ctx.Err() != nil || err != nil {
			hash = nil
			err = errs.Combine(err, upload.Cancel(ctx))
			return
		}
		h, closeErr := upload.Commit(ctx)
		hash = h
		err = errs.Combine(err, closeErr)
	}()

	_, err = sync2.Copy(ctx, upload, data)
	// Canceled context means the piece upload was interrupted by user or due
	// to slow connection. No error logging for this case.
	if ctx.Err() == context.Canceled {
		if parent.Err() == context.Canceled {
			ec.log.Info("Upload to node canceled by user", zap.String("NodeID", storageNodeID.String()))
		} else {
			ec.log.Debug("Node cut from upload due to slow connection", zap.String("NodeID", storageNodeID.String()))
		}
		err = context.Canceled
	} else if err != nil {
		nodeAddress := "nil"
		if limit.GetStorageNodeAddress() != nil {
			nodeAddress = limit.GetStorageNodeAddress().GetAddress()
		}

		ec.log.Debug("Failed uploading piece to node",
			zap.String("PieceID", pieceID.String()),
			zap.String("NodeID", storageNodeID.String()),
			zap.String("Node Address", nodeAddress),
			zap.Error(err),
		)
	}

	return hash, err
}

// copied from ecclient
func nonNilCount(limits []*pb.AddressedOrderLimit) int {
	total := 0
	for _, limit := range limits {
		if limit != nil {
			total++
		}
	}
	return total
}

// copied from ecclient
func unique(limits []*pb.AddressedOrderLimit) bool {
	if len(limits) < 2 {
		return true
	}
	ids := make(storj.NodeIDList, len(limits))
	for i, addressedLimit := range limits {
		if addressedLimit != nil {
			ids[i] = addressedLimit.GetLimit().StorageNodeId
		}
	}

	// sort the ids and check for identical neighbors
	sort.Sort(ids)
	// sort.Slice(ids, func(i, k int) bool { return ids[i].Less(ids[k]) })
	for i := 1; i < len(ids); i++ {
		if ids[i] != (storj.NodeID{}) && ids[i] == ids[i-1] {
			return false
		}
	}

	return true
}
