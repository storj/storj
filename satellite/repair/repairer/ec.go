// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package repairer

import (
	"bytes"
	"context"
	"io"
	"io/ioutil"
	"sort"
	"sync"
	"sync/atomic"
	"time"

	"github.com/vivint/infectious"
	"github.com/zeebo/errs"
	"go.uber.org/zap"

	"storj.io/common/errs2"
	"storj.io/common/pb"
	"storj.io/common/pkcrypto"
	"storj.io/common/rpc"
	"storj.io/common/signing"
	"storj.io/common/storj"
	"storj.io/common/sync2"
	"storj.io/storj/uplink/eestream"
	"storj.io/storj/uplink/piecestore"
)

// ErrPieceHashVerifyFailed is the errs class when a piece hash downloaded from storagenode fails to match the original hash.
var ErrPieceHashVerifyFailed = errs.Class("piece hashes don't match")

// ECRepairer allows the repairer to download, verify, and upload pieces from storagenodes.
type ECRepairer struct {
	log             *zap.Logger
	dialer          rpc.Dialer
	satelliteSignee signing.Signee
	downloadTimeout time.Duration
}

// NewECRepairer creates a new repairer for interfacing with storagenodes.
func NewECRepairer(log *zap.Logger, dialer rpc.Dialer, satelliteSignee signing.Signee, downloadTimeout time.Duration) *ECRepairer {
	return &ECRepairer{
		log:             log,
		dialer:          dialer,
		satelliteSignee: satelliteSignee,
		downloadTimeout: downloadTimeout,
	}
}

func (ec *ECRepairer) dialPiecestore(ctx context.Context, n *pb.Node) (*piecestore.Client, error) {
	logger := ec.log.Named(n.Id.String())
	return piecestore.Dial(ctx, ec.dialer, n, logger, piecestore.DefaultConfig)
}

// Get downloads pieces from storagenodes using the provided order limits, and decodes those pieces into a segment.
// It attempts to download from the minimum required number based on the redundancy scheme.
// After downloading a piece, the ECRepairer will verify the hash and original order limit for that piece.
// If verification fails, another piece will be downloaded until we reach the minimum required or run out of order limits.
// If piece hash verification fails, it will return all failed node IDs.
func (ec *ECRepairer) Get(ctx context.Context, limits []*pb.AddressedOrderLimit, privateKey storj.PiecePrivateKey, es eestream.ErasureScheme, dataSize int64, path storj.Path) (_ io.ReadCloser, failedPieces []*pb.RemotePiece, err error) {
	defer mon.Task()(&ctx)(&err)

	if len(limits) != es.TotalCount() {
		return nil, nil, Error.New("number of limits slice (%d) does not match total count (%d) of erasure scheme", len(limits), es.TotalCount())
	}

	nonNilLimits := nonNilCount(limits)

	if nonNilLimits < es.RequiredCount() {
		return nil, nil, Error.New("number of non-nil limits (%d) is less than required count (%d) of erasure scheme", nonNilCount(limits), es.RequiredCount())
	}

	pieceSize := eestream.CalcPieceSize(dataSize, es)

	var successfulPieces, inProgress int
	unusedLimits := nonNilLimits
	pieceReaders := make(map[int]io.ReadCloser)

	limiter := sync2.NewLimiter(es.RequiredCount())
	cond := sync.NewCond(&sync.Mutex{})

	for currentLimitIndex, limit := range limits {
		if limit == nil {
			continue
		}

		currentLimitIndex, limit := currentLimitIndex, limit
		limiter.Go(ctx, func() {
			cond.L.Lock()
			defer cond.Signal()
			defer cond.L.Unlock()

			for {
				if successfulPieces >= es.RequiredCount() {
					// already downloaded minimum number of pieces
					cond.Broadcast()
					return
				}
				if successfulPieces+inProgress+unusedLimits < es.RequiredCount() {
					// not enough available limits left to get required number of pieces
					cond.Broadcast()
					return
				}

				if successfulPieces+inProgress >= es.RequiredCount() {
					cond.Wait()
					continue
				}

				unusedLimits--
				inProgress++
				cond.L.Unlock()

				downloadedPiece, err := ec.downloadAndVerifyPiece(ctx, limit, privateKey, pieceSize)
				cond.L.Lock()
				inProgress--
				if err != nil {
					// gather nodes where the calculated piece hash doesn't match the uplink signed piece hash
					if ErrPieceHashVerifyFailed.Has(err) {
						failedPieces = append(failedPieces, &pb.RemotePiece{
							PieceNum: int32(currentLimitIndex),
							NodeId:   limit.GetLimit().StorageNodeId,
						})
					} else {
						ec.log.Debug("Failed to download pieces for repair",
							zap.Binary("Segment", []byte(path)),
							zap.Error(err))
					}
					return
				}

				pieceReaders[currentLimitIndex] = ioutil.NopCloser(bytes.NewReader(downloadedPiece))
				successfulPieces++

				return
			}
		})
	}

	limiter.Wait()

	if successfulPieces < es.RequiredCount() {
		mon.Meter("download_failed_not_enough_pieces_repair").Mark(1) //locked
		return nil, failedPieces, Error.New("couldn't download enough pieces for segment: %s, number of successful downloaded pieces (%d) is less than required number (%d)", path, successfulPieces, es.RequiredCount())
	}

	fec, err := infectious.NewFEC(es.RequiredCount(), es.TotalCount())
	if err != nil {
		return nil, failedPieces, Error.Wrap(err)
	}

	esScheme := eestream.NewUnsafeRSScheme(fec, es.ErasureShareSize())
	expectedSize := pieceSize * int64(es.RequiredCount())

	ctx, cancel := context.WithCancel(ctx)
	decodeReader := eestream.DecodeReaders(ctx, cancel, ec.log.Named("decode readers"), pieceReaders, esScheme, expectedSize, 0, false)

	return decodeReader, failedPieces, nil
}

// downloadAndVerifyPiece downloads a piece from a storagenode,
// expects the original order limit to have the correct piece public key,
// and expects the hash of the data to match the signed hash provided by the storagenode.
func (ec *ECRepairer) downloadAndVerifyPiece(ctx context.Context, limit *pb.AddressedOrderLimit, privateKey storj.PiecePrivateKey, pieceSize int64) (data []byte, err error) {
	// contact node
	downloadCtx, cancel := context.WithTimeout(ctx, ec.downloadTimeout)
	defer cancel()

	ps, err := ec.dialPiecestore(downloadCtx, &pb.Node{
		Id:      limit.GetLimit().StorageNodeId,
		Address: limit.GetStorageNodeAddress(),
	})
	if err != nil {
		return nil, err
	}
	defer func() { err = errs.Combine(err, ps.Close()) }()

	downloader, err := ps.Download(downloadCtx, limit.GetLimit(), privateKey, 0, pieceSize)
	if err != nil {
		return nil, err
	}
	defer func() { err = errs.Combine(err, downloader.Close()) }()

	pieceBytes, err := ioutil.ReadAll(downloader)
	if err != nil {
		return nil, err
	}

	if int64(len(pieceBytes)) != pieceSize {
		return nil, Error.New("didn't download the correct amount of data, want %d, got %d", pieceSize, len(pieceBytes))
	}

	// get signed piece hash and original order limit
	hash, originalLimit := downloader.GetHashAndLimit()
	if hash == nil {
		return nil, Error.New("hash was not sent from storagenode")
	}
	if originalLimit == nil {
		return nil, Error.New("original order limit was not sent from storagenode")
	}

	// verify order limit from storage node is signed by the satellite
	if err := verifyOrderLimitSignature(ctx, ec.satelliteSignee, originalLimit); err != nil {
		return nil, err
	}

	// verify the hashes from storage node
	calculatedHash := pkcrypto.SHA256Hash(pieceBytes)
	if err := verifyPieceHash(ctx, originalLimit, hash, calculatedHash); err != nil {
		return nil, ErrPieceHashVerifyFailed.Wrap(err)
	}

	return pieceBytes, nil
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

// Repair takes a provided segment, encodes it with the provided redundancy strategy,
// and uploads the pieces in need of repair to new nodes provided by order limits.
func (ec *ECRepairer) Repair(ctx context.Context, limits []*pb.AddressedOrderLimit, privateKey storj.PiecePrivateKey, rs eestream.RedundancyStrategy, data io.Reader, timeout time.Duration, path storj.Path) (successfulNodes []*pb.Node, successfulHashes []*pb.PieceHash, err error) {
	defer mon.Task()(&ctx)(&err)

	pieceCount := len(limits)
	if pieceCount != rs.TotalCount() {
		return nil, nil, Error.New("size of limits slice (%d) does not match total count (%d) of erasure scheme", pieceCount, rs.TotalCount())
	}

	if !unique(limits) {
		return nil, nil, Error.New("duplicated nodes are not allowed")
	}

	readers, err := eestream.EncodeReader(ctx, ec.log, ioutil.NopCloser(data), rs)
	if err != nil {
		return nil, nil, err
	}

	// info contains data about a single piece transfer
	type info struct {
		i    int
		err  error
		hash *pb.PieceHash
	}
	// this channel is used to synchronize concurrently uploaded pieces with the overall repair
	infos := make(chan info, pieceCount)

	psCtx, cancel := context.WithCancel(ctx)
	defer cancel()

	for i, addressedLimit := range limits {
		go func(i int, addressedLimit *pb.AddressedOrderLimit) {
			hash, err := ec.putPiece(psCtx, ctx, addressedLimit, privateKey, readers[i], path)
			infos <- info{i: i, err: err, hash: hash}
		}(i, addressedLimit)
	}
	ec.log.Info("Starting a timer for repair so that the number of pieces will be closer to the success threshold",
		zap.Binary("Segment", []byte(path)),
		zap.Duration("Timer", timeout),
		zap.Int("Node Count", nonNilCount(limits)),
		zap.Int("Optimal Threshold", rs.OptimalThreshold()),
	)

	var successfulCount, failureCount, cancellationCount int32
	timer := time.AfterFunc(timeout, func() {
		if ctx.Err() != context.Canceled {
			ec.log.Info("Timer expired. Canceling the long tail...",
				zap.Binary("Segment", []byte(path)),
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
				zap.Binary("Segment", []byte(path)),
				zap.Stringer("Node ID", limits[info.i].GetLimit().StorageNodeId),
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
		default:
		}
	}()

	if successfulCount == 0 {
		return nil, nil, Error.New("repair to all nodes failed")
	}

	ec.log.Info("Successfully repaired",
		zap.Binary("Segment", []byte(path)),
		zap.Int32("Success Count", atomic.LoadInt32(&successfulCount)),
	)

	mon.IntVal("repair_segment_pieces_total").Observe(int64(pieceCount))
	mon.IntVal("repair_segment_pieces_successful").Observe(int64(successfulCount))
	mon.IntVal("repair_segment_pieces_failed").Observe(int64(failureCount))
	mon.IntVal("repair_segment_pieces_canceled").Observe(int64(cancellationCount))

	return successfulNodes, successfulHashes, nil
}

func (ec *ECRepairer) putPiece(ctx, parent context.Context, limit *pb.AddressedOrderLimit, privateKey storj.PiecePrivateKey, data io.ReadCloser, path storj.Path) (hash *pb.PieceHash, err error) {
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
			zap.Binary("Segment", []byte(path)),
			zap.Stringer("Piece ID", pieceID),
			zap.Stringer("Node ID", storageNodeID),
			zap.Error(err),
		)
		return nil, err
	}
	defer func() { err = errs.Combine(err, ps.Close()) }()

	upload, err := ps.Upload(ctx, limit.GetLimit(), privateKey)
	if err != nil {
		ec.log.Debug("Failed requesting upload of pieces to node",
			zap.Binary("Segment", []byte(path)),
			zap.Stringer("Piece ID", pieceID),
			zap.Stringer("Node ID", storageNodeID),
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
			ec.log.Info("Upload to node canceled by user",
				zap.Binary("Segment", []byte(path)),
				zap.Stringer("Node ID", storageNodeID))
		} else {
			ec.log.Debug("Node cut from upload due to slow connection",
				zap.Binary("Segment", []byte(path)),
				zap.Stringer("Node ID", storageNodeID))
		}
		err = context.Canceled
	} else if err != nil {
		nodeAddress := "nil"
		if limit.GetStorageNodeAddress() != nil {
			nodeAddress = limit.GetStorageNodeAddress().GetAddress()
		}

		ec.log.Debug("Failed uploading piece to node",
			zap.Binary("Segment", []byte(path)),
			zap.Stringer("Piece ID", pieceID),
			zap.Stringer("Node ID", storageNodeID),
			zap.String("Node Address", nodeAddress),
			zap.Error(err),
		)
	}

	return hash, err
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
