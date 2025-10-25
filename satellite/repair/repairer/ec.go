// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package repairer

import (
	"bytes"
	"context"
	"errors"
	"hash"
	"io"
	"sort"
	"sync"
	"sync/atomic"
	"time"

	"github.com/calebcase/tmpfile"
	"github.com/zeebo/errs"
	"go.uber.org/zap"

	"storj.io/common/errs2"
	"storj.io/common/fpath"
	"storj.io/common/pb"
	"storj.io/common/rpc"
	"storj.io/common/rpc/rpcpool"
	"storj.io/common/signing"
	"storj.io/common/storj"
	"storj.io/common/sync2"
	"storj.io/storj/satellite/audit"
	"storj.io/storj/satellite/metabase"
	"storj.io/storj/satellite/overlay"
	"storj.io/uplink/private/eestream"
	"storj.io/uplink/private/piecestore"
)

var (
	// ErrPieceHashVerifyFailed is the errs class when a piece hash downloaded from storagenode fails to match the original hash.
	ErrPieceHashVerifyFailed = errs.Class("piece hashes don't match")

	// ErrDialFailed is the errs class when a failure happens during Dial.
	ErrDialFailed = errs.Class("dial failure")

	// ErrDownloadTimedOut is the errs class when a download times out.
	ErrDownloadTimedOut = errs.Class("download timed out")
)

// ECRepairer allows the repairer to download, verify, and upload pieces from storagenodes.
type ECRepairer struct {
	dialer           rpc.Dialer
	satelliteSignee  signing.Signee
	dialTimeout      time.Duration
	downloadTimeout  time.Duration
	inmemoryDownload bool
	inmemoryUpload   bool
	downloadLongTail int

	// used only in tests, where we expect failures and want to wait for them
	minFailures int
}

// NewECRepairer creates a new repairer for interfacing with storagenodes.
func NewECRepairer(dialer rpc.Dialer, satelliteSignee signing.Signee, dialTimeout time.Duration, downloadTimeout time.Duration,
	inmemoryDownload, inmemoryUpload bool, downloadLongTail int) *ECRepairer {
	return &ECRepairer{
		dialer:           dialer,
		satelliteSignee:  satelliteSignee,
		dialTimeout:      dialTimeout,
		downloadTimeout:  downloadTimeout,
		inmemoryDownload: inmemoryDownload,
		inmemoryUpload:   inmemoryUpload,
		downloadLongTail: downloadLongTail,
	}
}

func (ec *ECRepairer) dialPiecestore(ctx context.Context, n storj.NodeURL) (*piecestore.Client, error) {
	ctx = rpcpool.WithForceDial(ctx)
	hashAlgo := piecestore.GetPieceHashAlgo(ctx)
	client, err := piecestore.Dial(ctx, ec.dialer, n, piecestore.DefaultConfig)
	if err != nil {
		return nil, ErrDialFailed.Wrap(err)
	}
	client.UploadHashAlgo = hashAlgo
	return client, nil
}

// TestingSetMinFailures sets the minFailures attribute, which tells the Repair machinery that we _expect_
// there to be failures and that we should wait for them if necessary. This is only used in tests.
func (ec *ECRepairer) TestingSetMinFailures(minFailures int) {
	ec.minFailures = minFailures
}

// Get downloads pieces from storagenodes using the provided order limits, and decodes those pieces into a segment.
// It attempts to download from the minimum required number based on the redundancy scheme. It will further wait
// for additional error/failure results up to minFailures, for testing purposes. Under normal conditions,
// minFailures will be 0.
//
// After downloading a piece, the ECRepairer will verify the hash and original order limit for that piece.
// If verification fails, another piece will be downloaded until we reach the minimum required or run out of order limits.
// If piece hash verification fails, it will return all failed node IDs.
func (ec *ECRepairer) Get(ctx context.Context, log *zap.Logger, limits []*pb.AddressedOrderLimit, cachedNodesInfo map[storj.NodeID]overlay.NodeReputation, privateKey storj.PiecePrivateKey, es eestream.ErasureScheme, dataSize int64) (_ io.ReadCloser, _ FetchResultReport, err error) {
	defer mon.Task()(&ctx)(&err)

	if len(limits) != es.TotalCount() {
		return nil, FetchResultReport{}, Error.New("number of limits slice (%d) does not match total count (%d) of erasure scheme", len(limits), es.TotalCount())
	}

	nonNilLimits := nonNilCount(limits)

	if nonNilLimits < es.RequiredCount()+ec.minFailures {
		return nil, FetchResultReport{}, Error.New("number of non-nil limits (%d) is less than requested result count (%d)", nonNilCount(limits), es.RequiredCount()+ec.minFailures)
	}

	mon.IntVal("ECRepairer_Get_nonNilLimits").Observe(int64(nonNilLimits))

	pieceSize := eestream.CalcPieceSize(dataSize, es)

	errorCount := 0
	var successfulPieces, inProgress int
	unusedLimits := nonNilLimits
	pieceReaders := make(map[int]io.ReadCloser)
	var pieces FetchResultReport

	// Allow more concurrent downloads than required for racing
	downloadConcurrency := min(es.RequiredCount()+ec.downloadLongTail, nonNilLimits)
	limiter := sync2.NewLimiter(downloadConcurrency)
	cond := sync.NewCond(&sync.Mutex{})

	// Track download contexts for cancellation
	downloadCtxs := make(map[int]context.CancelFunc)

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
				if successfulPieces >= es.RequiredCount() && errorCount >= ec.minFailures {
					// already downloaded required number of pieces

					// Cancel all remaining downloads
					for _, cancelFunc := range downloadCtxs {
						cancelFunc()
					}
					cond.Broadcast()
					return
				}
				if successfulPieces+inProgress+unusedLimits < es.RequiredCount() || errorCount+inProgress+unusedLimits < ec.minFailures {
					// not enough available limits left to get required number of pieces
					cond.Broadcast()
					return
				}

				if successfulPieces+inProgress >= es.RequiredCount() && errorCount+inProgress >= ec.minFailures {
					// we know that inProgress > 0 here, since we didn't return on the
					// "successfulPieces >= es.RequiredCount() && errorCount >= ec.minFailures" check earlier.
					// There may be enough downloads in progress to meet all of our needs, so we won't
					// start any more immediately. Instead, wait until all needs are met (in which case
					// cond.Broadcast() will be called) or until one of the inProgress workers exits
					// (in which case cond.Signal() will be called, waking up one waiter) so we can
					// reevaluate the situation.
					cond.Wait()
					continue
				}

				unusedLimits--
				inProgress++

				// Create a cancellable context for this download
				var downloadCtx context.Context
				var downloadCancel context.CancelFunc
				downloadCtx, downloadCancel = context.WithCancel(ctx)
				downloadCtxs[currentLimitIndex] = downloadCancel

				cond.L.Unlock()

				info := cachedNodesInfo[limit.GetLimit().StorageNodeId]
				address := limit.GetStorageNodeAddress().GetAddress()
				var triedLastIPPort bool
				if info.LastIPPort != "" && info.LastIPPort != address {
					address = info.LastIPPort
					triedLastIPPort = true
				}

				log.Debug("attempting to fetch piece for repair",
					zap.Stringer("Node ID", limit.GetLimit().StorageNodeId),
					zap.Stringer("Piece ID", limit.Limit.PieceId),
					zap.Int("piece index", currentLimitIndex),
					zap.String("address", limit.GetStorageNodeAddress().Address),
					zap.String("last_ip_port", info.LastIPPort),
					zap.Binary("serial", limit.Limit.SerialNumber[:]))

				pieceReadCloser, _, _, err := ec.downloadAndVerifyPiece(downloadCtx, limit, address, privateKey, "", pieceSize)
				// if piecestore dial with last ip:port failed try again with node address
				if triedLastIPPort && ErrDialFailed.Has(err) {
					if pieceReadCloser != nil {
						_ = pieceReadCloser.Close()
					}
					log.Info("repair get failed; retrying with specified hostname", zap.Error(err), zap.String("last_ip_port", info.LastIPPort), zap.String("hostname", limit.GetStorageNodeAddress().GetAddress()))
					pieceReadCloser, _, _, err = ec.downloadAndVerifyPiece(downloadCtx, limit, limit.GetStorageNodeAddress().GetAddress(), privateKey, "", pieceSize)
				}

				downloadCancel()

				cond.L.Lock()
				inProgress--
				piece := metabase.Piece{
					Number:      uint16(currentLimitIndex),
					StorageNode: limit.GetLimit().StorageNodeId,
				}

				if err != nil {
					if pieceReadCloser != nil {
						_ = pieceReadCloser.Close()
					}

					// If download was canceled due to racing (we already have enough pieces), just return
					if errors.Is(err, context.Canceled) && downloadCtx.Err() != nil {
						log.Debug("Download canceled due to racing",
							zap.Stringer("Node ID", limit.GetLimit().StorageNodeId),
							zap.Stringer("Piece ID", limit.Limit.PieceId))
						return
					}

					// gather nodes where the calculated piece hash doesn't match the uplink signed piece hash
					if ErrPieceHashVerifyFailed.Has(err) {
						log.Info("audit failed",
							zap.Stringer("node ID", limit.GetLimit().StorageNodeId),
							zap.Stringer("Piece ID", limit.Limit.PieceId),
							zap.String("reason", err.Error()))
						pieces.Failed = append(pieces.Failed, PieceFetchResult{Piece: piece, Err: err})
						errorCount++
						return
					}

					var pieceAudit audit.PieceAudit
					if ErrDownloadTimedOut.Has(err) {
						pieceAudit = audit.PieceAuditContained
					} else {
						pieceAudit = audit.PieceAuditFromErr(err)
					}

					switch pieceAudit {
					case audit.PieceAuditFailure:
						log.Debug("Failed to download piece for repair: piece not found (audit failed)",
							zap.Stringer("Node ID", limit.GetLimit().StorageNodeId),
							zap.Stringer("Piece ID", limit.Limit.PieceId),
							zap.Error(err))
						pieces.Failed = append(pieces.Failed, PieceFetchResult{Piece: piece, Err: err})
						errorCount++

					case audit.PieceAuditOffline:
						log.Debug("Failed to download piece for repair: dial timeout (offline)",
							zap.Stringer("Node ID", limit.GetLimit().StorageNodeId),
							zap.Stringer("Piece ID", limit.Limit.PieceId),
							zap.Error(err))
						pieces.Offline = append(pieces.Offline, PieceFetchResult{Piece: piece, Err: err})
						errorCount++

					case audit.PieceAuditContained:
						log.Info("Failed to download piece for repair: download timeout (contained)",
							zap.Stringer("Node ID", limit.GetLimit().StorageNodeId),
							zap.Stringer("Piece ID", limit.Limit.PieceId),
							zap.Error(err))
						pieces.Contained = append(pieces.Contained, PieceFetchResult{Piece: piece, Err: err})
						errorCount++

					case audit.PieceAuditUnknown:
						log.Info("Failed to download piece for repair: unknown transport error (skipped)",
							zap.Stringer("Node ID", limit.GetLimit().StorageNodeId),
							zap.Stringer("Piece ID", limit.Limit.PieceId),
							zap.Error(err))
						pieces.Unknown = append(pieces.Unknown, PieceFetchResult{Piece: piece, Err: err})
						errorCount++
					}

					return
				}

				pieceReaders[currentLimitIndex] = pieceReadCloser
				pieces.Successful = append(pieces.Successful, PieceFetchResult{Piece: piece})
				successfulPieces++
				return
			}
		})
	}

	limiter.Wait()

	if successfulPieces < es.RequiredCount() {
		mon.Meter("download_failed_not_enough_pieces_repair").Mark(1)
		return nil, pieces, &irreparableError{
			piecesAvailable: int32(successfulPieces),
			piecesRequired:  int32(es.RequiredCount()),
		}
	}
	if errorCount < ec.minFailures {
		return nil, pieces, Error.New("expected %d failures, but only observed %d", ec.minFailures, errorCount)
	}

	fec, err := eestream.NewFEC(es.RequiredCount(), es.TotalCount())
	if err != nil {
		return nil, pieces, Error.Wrap(err)
	}

	esScheme := eestream.NewUnsafeRSScheme(fec, es.ErasureShareSize())
	expectedSize := pieceSize * int64(es.RequiredCount())

	ctx, cancel := context.WithCancel(ctx)
	decodeReader := eestream.DecodeReaders2(ctx, cancel, pieceReaders, esScheme, expectedSize, 0, false)

	return decodeReader, pieces, nil
}

// lazyHashWriter is a writer which can get the hash algorithm just before the first write.
type lazyHashWriter struct {
	hasher     hash.Hash
	downloader *piecestore.Download
}

func (l *lazyHashWriter) Write(p []byte) (n int, err error) {
	// hash is available only after receiving the first message.
	if l.hasher == nil {
		h, _ := l.downloader.GetHashAndLimit()
		l.hasher = pb.NewHashFromAlgorithm(h.HashAlgorithm)
	}
	return l.hasher.Write(p)
}

// Sum delegates hash calculation to the real hash algorithm.
func (l *lazyHashWriter) Sum(b []byte) []byte {
	if l.hasher == nil {
		return []byte{}
	}
	return l.hasher.Sum(b)
}

var _ io.Writer = &lazyHashWriter{}

// downloadAndVerifyPiece downloads a piece from a storagenode,
// expects the original order limit to have the correct piece public key,
// and expects the hash of the data to match the signed hash provided by the storagenode.
func (ec *ECRepairer) downloadAndVerifyPiece(ctx context.Context, limit *pb.AddressedOrderLimit, address string, privateKey storj.PiecePrivateKey, tmpDir string, pieceSize int64) (pieceReadCloser io.ReadCloser, hash *pb.PieceHash, originalLimit *pb.OrderLimit, err error) {
	defer mon.Task()(&ctx)(&err)

	// contact node
	dialCtx, dialCancel := context.WithTimeout(ctx, ec.dialTimeout)
	defer dialCancel()

	ps, err := ec.dialPiecestore(dialCtx, storj.NodeURL{
		ID:      limit.GetLimit().StorageNodeId,
		Address: address,
	})
	if err != nil {
		return nil, nil, nil, err
	}
	defer func() { err = errs.Combine(err, ps.Close()) }()

	downloadCtx, cancel := context.WithTimeout(ctx, ec.downloadTimeout)
	defer cancel()

	downloader, err := ps.Download(downloadCtx, limit.GetLimit(), privateKey, 0, pieceSize)
	if err != nil {
		if errs.Is(err, context.DeadlineExceeded) {
			return nil, nil, nil, ErrDownloadTimedOut.Wrap(err)
		}
		return nil, nil, nil, err
	}
	defer func() { err = errs.Combine(err, downloader.Close()) }()

	hashWriter := &lazyHashWriter{
		downloader: downloader,
	}
	downloadReader := io.TeeReader(downloader, hashWriter)
	var downloadedPieceSize int64

	if ec.inmemoryDownload {
		// allocate whole buffer in advance
		buffer := make([]byte, pieceSize)
		n, err := io.ReadFull(downloadReader, buffer)
		if err != nil {
			return nil, nil, nil, err
		}
		downloadedPieceSize = int64(n)
		pieceReadCloser = io.NopCloser(bytes.NewReader(buffer[:n]))
	} else {
		tempfile, err := tmpfile.New(tmpDir, "satellite-repair-*")
		if err != nil {
			return nil, nil, nil, err
		}
		// no defer tempfile.Close() here; caller is responsible for closing
		// the file, even if an error results (the caller might want the data
		// even if there is a verification error).

		downloadedPieceSize, err = sync2.Copy(ctx, tempfile, downloadReader)
		if err != nil {
			return tempfile, nil, nil, err
		}

		// seek to beginning of file so the repair job starts at the beginning of the piece
		_, err = tempfile.Seek(0, io.SeekStart)
		if err != nil {
			return tempfile, nil, nil, err
		}
		pieceReadCloser = tempfile
	}

	mon.Meter("repair_bytes_downloaded").Mark64(downloadedPieceSize)

	if downloadedPieceSize != pieceSize {
		return pieceReadCloser, nil, nil, Error.New("didn't download the correct amount of data, want %d, got %d", pieceSize, downloadedPieceSize)
	}

	// get signed piece hash and original order limit
	hash, originalLimit = downloader.GetHashAndLimit()
	if hash == nil {
		return pieceReadCloser, hash, originalLimit, Error.New("hash was not sent from storagenode")
	}
	if originalLimit == nil {
		return pieceReadCloser, hash, originalLimit, Error.New("original order limit was not sent from storagenode")
	}

	// verify order limit from storage node is signed by the satellite
	if err := verifyOrderLimitSignature(ctx, ec.satelliteSignee, originalLimit); err != nil {
		return pieceReadCloser, hash, originalLimit, err
	}

	// verify the hashes from storage node
	calculatedHash := hashWriter.Sum(nil)
	if err := verifyPieceHash(ctx, originalLimit, hash, calculatedHash); err != nil {

		return pieceReadCloser, hash, originalLimit, ErrPieceHashVerifyFailed.Wrap(err)
	}

	return pieceReadCloser, hash, originalLimit, nil
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
		return Error.New("hash from storage node, %x, does not match calculated hash, %x", hash.Hash, expectedHash)
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
func (ec *ECRepairer) Repair(ctx context.Context, log *zap.Logger, limits []*pb.AddressedOrderLimit, privateKey storj.PiecePrivateKey, rs eestream.RedundancyStrategy, data io.Reader, timeout time.Duration, successfulNeeded int) (successfulNodes []*pb.Node, successfulHashes []*pb.PieceHash, err error) {
	defer mon.Task()(&ctx)(&err)

	pieceCount := len(limits)
	if pieceCount != rs.TotalCount() {
		return nil, nil, Error.New("size of limits slice (%d) does not match total count (%d) of erasure scheme", pieceCount, rs.TotalCount())
	}

	if !unique(limits) {
		return nil, nil, Error.New("duplicated nodes are not allowed")
	}

	if ec.inmemoryUpload {
		ctx = fpath.WithTempData(ctx, "", true)
	}

	readers, err := eestream.EncodeReader2(ctx, io.NopCloser(data), rs)
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
			hash, err := ec.putPiece(psCtx, ctx, log, addressedLimit, privateKey, readers[i])
			infos <- info{i: i, err: err, hash: hash}
		}(i, addressedLimit)
	}
	log.Debug("Starting a timer for repair so that the number of pieces will be closer to the success threshold",
		zap.Duration("Timer", timeout),
		zap.Int("Node Count", nonNilCount(limits)),
		zap.Int("Optimal Threshold", rs.OptimalThreshold()),
	)

	var successfulCount, failureCount, cancellationCount int32
	timer := time.AfterFunc(timeout, func() {
		if !errors.Is(ctx.Err(), context.Canceled) {
			log.Debug("Timer expired. Canceling the long tail...",
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
				log.Warn("Repair to a storage node failed",
					zap.Stringer("Node ID", limits[info.i].GetLimit().StorageNodeId),
					zap.Error(info.err),
				)
			} else {
				cancellationCount++
				log.Debug("Repair to storage node cancelled",
					zap.Stringer("Node ID", limits[info.i].GetLimit().StorageNodeId),
					zap.Error(info.err),
				)
			}
			continue
		}

		successfulNodes[info.i] = &pb.Node{
			Id:      limits[info.i].GetLimit().StorageNodeId,
			Address: limits[info.i].GetStorageNodeAddress(),
		}
		successfulHashes[info.i] = info.hash
		successfulCount++

		if successfulCount >= int32(successfulNeeded) {
			// if this is logged more than once for a given repair operation, it is because
			// an upload succeeded right after we called cancel(), before that upload could
			// actually be canceled. So, successfulCount should increase by one with each
			// repeated logging.
			log.Debug("Number of successful uploads met. Canceling the long tail...",
				zap.Int32("Successfully repaired", atomic.LoadInt32(&successfulCount)),
			)
			cancel()
		}
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

	log.Debug("Successfully repaired",
		zap.Int32("Success Count", atomic.LoadInt32(&successfulCount)),
	)

	mon.IntVal("repair_segment_pieces_total").Observe(int64(pieceCount))
	mon.IntVal("repair_segment_pieces_successful").Observe(int64(successfulCount))
	mon.IntVal("repair_segment_pieces_failed").Observe(int64(failureCount))
	mon.IntVal("repair_segment_pieces_canceled").Observe(int64(cancellationCount))

	return successfulNodes, successfulHashes, nil
}

func (ec *ECRepairer) putPiece(ctx, parent context.Context, log *zap.Logger, limit *pb.AddressedOrderLimit, privateKey storj.PiecePrivateKey, data io.ReadCloser) (hash *pb.PieceHash, err error) {
	defer mon.Task()(&ctx)(&err)

	nodeName := "nil"
	if limit != nil {
		nodeName = limit.GetLimit().StorageNodeId.String()[0:8]
	}
	defer mon.Task()(&ctx, "node: "+nodeName)(&err)
	defer func() { err = errs.Combine(err, data.Close()) }()

	if limit == nil {
		_, _ = io.Copy(io.Discard, data)
		return nil, nil
	}

	storageNodeID := limit.GetLimit().StorageNodeId
	pieceID := limit.GetLimit().PieceId

	dialCtx, dialCancel := context.WithTimeout(ctx, ec.dialTimeout)
	defer dialCancel()

	ps, err := ec.dialPiecestore(dialCtx, storj.NodeURL{
		ID:      storageNodeID,
		Address: limit.GetStorageNodeAddress().Address,
	})
	if err != nil {
		log.Debug("Failed dialing for putting piece to node",
			zap.Stringer("Piece ID", pieceID),
			zap.Stringer("Node ID", storageNodeID),
			zap.Error(err),
		)
		return nil, err
	}
	defer func() { err = errs.Combine(err, ps.Close()) }()

	hash, err = ps.UploadReader(ctx, limit.GetLimit(), privateKey, data)
	if err != nil {
		if errors.Is(ctx.Err(), context.Canceled) {
			// Canceled context means the piece upload was interrupted by user or due
			// to slow connection. No error logging for this case.
			if errors.Is(parent.Err(), context.Canceled) {
				log.Debug("Upload to node canceled by user",
					zap.Stringer("Node ID", storageNodeID),
					zap.Stringer("Piece ID", pieceID))
			} else {
				log.Debug("Node cut from upload due to slow connection",
					zap.Stringer("Node ID", storageNodeID),
					zap.Stringer("Piece ID", pieceID))
			}

			// make sure context.Canceled is the primary error in the error chain
			// for later errors.Is/errs2.IsCanceled checking
			err = errs.Combine(context.Canceled, err)
		} else {
			nodeAddress := "nil"
			if limit.GetStorageNodeAddress() != nil {
				nodeAddress = limit.GetStorageNodeAddress().GetAddress()
			}

			log.Debug("Failed uploading piece to node",
				zap.Stringer("Piece ID", pieceID),
				zap.Stringer("Node ID", storageNodeID),
				zap.String("Node Address", nodeAddress),
				zap.Error(err),
			)
		}
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
