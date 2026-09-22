// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package repairer

import (
	"bytes"
	"context"
	"errors"
	"hash"
	"io"
	"math/bits"
	"sort"
	"sync"
	"sync/atomic"
	"time"

	"github.com/calebcase/tmpfile"
	"github.com/zeebo/errs"
	"go.uber.org/zap"

	"storj.io/common/errs2"
	"storj.io/common/fpath"
	"storj.io/common/memory"
	"storj.io/common/pb"
	"storj.io/common/rpc"
	"storj.io/common/rpc/rpcpool"
	"storj.io/common/signing"
	"storj.io/common/storj"
	"storj.io/common/sync2"
	"storj.io/common/sync2/race2"
	"storj.io/storj/satellite/audit"
	"storj.io/storj/satellite/metabase"
	"storj.io/storj/satellite/overlay"
	"storj.io/storj/shared/sync/kofn"
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
	dialer            rpc.Dialer
	satelliteSignee   signing.Signee
	dialTimeout       time.Duration
	downloadTimeout   time.Duration
	inmemoryDownload  bool
	inmemoryUpload    bool
	downloadLongTail  int
	downloadChunkSize int32

	// ownedPool is the connection pool this ECRepairer is responsible for
	// closing. It is set only when the pool was built for the repairer (see
	// mud.go); a pool that arrived on a shared dialer belongs to its owner.
	ownedPool *rpcpool.Pool

	// used only in tests, where we expect failures and want to wait for them
	minFailures int
}

// NewECRepairer creates a new repairer for interfacing with storagenodes.
func NewECRepairer(dialer rpc.Dialer, satelliteSignee signing.Signee, dialTimeout time.Duration, downloadTimeout time.Duration,
	inmemoryDownload, inmemoryUpload bool, downloadLongTail int, downloadChunkSize memory.Size) *ECRepairer {
	return &ECRepairer{
		dialer:            dialer,
		satelliteSignee:   satelliteSignee,
		dialTimeout:       dialTimeout,
		downloadTimeout:   downloadTimeout,
		inmemoryDownload:  inmemoryDownload,
		inmemoryUpload:    inmemoryUpload,
		downloadLongTail:  downloadLongTail,
		downloadChunkSize: downloadChunkSize.Int32(),
	}
}

// Close releases the resources owned by the ECRepairer.
func (ec *ECRepairer) Close() error {
	if ec.ownedPool == nil {
		return nil
	}
	return ec.ownedPool.Close()
}

func (ec *ECRepairer) dialPiecestore(ctx context.Context, n storj.NodeURL) (*piecestore.Client, error) {
	ctx = rpcpool.WithForceDial(ctx)
	hashAlgo := piecestore.GetPieceHashAlgo(ctx)
	piecestoreCfg := piecestore.DefaultConfig
	if ec.downloadChunkSize > 0 {
		piecestoreCfg.MaximumChunkSize = ec.downloadChunkSize
	}
	client, err := piecestore.Dial(ctx, ec.dialer, n, piecestoreCfg)
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

	successes, failures := kofn.Collect(
		ctx,
		kofn.Config{
			// Allow more concurrent downloads than required for completion.
			Concurrency:       es.RequiredCount(),
			LongTail:          ec.downloadLongTail,
			RequiredSuccesses: es.RequiredCount(),
			RequiredFailures:  ec.minFailures,
		},
		limits,
		func(limit *pb.AddressedOrderLimit) bool { return limit == nil },
		func(ctx context.Context, index int, limit *pb.AddressedOrderLimit) (io.ReadCloser, error) {
			return ec.downloadPiece(ctx, log, index, limit, cachedNodesInfo, privateKey, pieceSize)
		},
	)

	// Build pieceReaders map and FetchResultReport from racing results
	pieceReaders := make(map[int]io.ReadCloser)
	var pieces FetchResultReport

	for _, result := range successes {
		pieceReaders[result.Index] = result.Value
		pieces.Successful = append(pieces.Successful, PieceFetchResult{
			Piece: metabase.Piece{
				Number:      uint16(result.Index),
				StorageNode: limits[result.Index].GetLimit().StorageNodeId,
			},
		})
	}

	for _, result := range failures {
		limit := limits[result.Index]
		piece := metabase.Piece{
			Number:      uint16(result.Index),
			StorageNode: limit.GetLimit().StorageNodeId,
		}
		fetchResult := PieceFetchResult{Piece: piece, Err: result.Error}

		// Classify the error (logging already happened in downloadPiece)
		if errors.Is(result.Error, context.Canceled) {
			// Download was canceled due to racing, don't record as failure
			continue
		}

		if ErrPieceHashVerifyFailed.Has(result.Error) {
			pieces.Failed = append(pieces.Failed, fetchResult)
			continue
		}

		var pieceAudit audit.PieceAudit
		if ErrDownloadTimedOut.Has(result.Error) {
			pieceAudit = audit.PieceAuditContained
		} else {
			pieceAudit = audit.PieceAuditFromErr(result.Error)
		}

		switch pieceAudit {
		case audit.PieceAuditFailure:
			pieces.Failed = append(pieces.Failed, fetchResult)
		case audit.PieceAuditOffline:
			pieces.Offline = append(pieces.Offline, fetchResult)
		case audit.PieceAuditContained:
			pieces.Contained = append(pieces.Contained, fetchResult)
		case audit.PieceAuditUnknown:
			pieces.Unknown = append(pieces.Unknown, fetchResult)
		}
	}

	successfulPieces := len(successes)
	errorCount := len(pieces.Failed) + len(pieces.Offline) + len(pieces.Contained) + len(pieces.Unknown)

	if successfulPieces < es.RequiredCount() {
		mon.Meter("download_failed_not_enough_pieces_repair").Mark(1)
		closePieceReaders(pieceReaders)
		return nil, pieces, &irreparableError{
			piecesAvailable: int32(successfulPieces),
			piecesRequired:  int32(es.RequiredCount()),
		}
	}
	if errorCount < ec.minFailures {
		closePieceReaders(pieceReaders)
		return nil, pieces, Error.New("expected %d failures, but only observed %d", ec.minFailures, errorCount)
	}

	fec, err := eestream.NewFEC(es.RequiredCount(), es.TotalCount())
	if err != nil {
		closePieceReaders(pieceReaders)
		return nil, pieces, Error.Wrap(err)
	}

	esScheme := eestream.NewUnsafeRSScheme(fec, es.ErasureShareSize())
	expectedSize := pieceSize * int64(es.RequiredCount())

	ctx, cancel := context.WithCancel(ctx)
	decodeReader := eestream.DecodeReaders2(ctx, cancel, pieceReaders, esScheme, expectedSize, 0, false)

	return decodeReader, pieces, nil
}

// closePieceReaders closes the pieces a download gives up on, so that their
// pooled buffers go back to the pool, and the temporary files of the on-disk
// path release their space, instead of waiting for the garbage collector.
func closePieceReaders(pieceReaders map[int]io.ReadCloser) {
	for _, pieceReader := range pieceReaders {
		_ = pieceReader.Close()
	}
}

// downloadPiece downloads a single piece from a storage node, handling LastIPPort retry logic.
func (ec *ECRepairer) downloadPiece(ctx context.Context, log *zap.Logger, index int, limit *pb.AddressedOrderLimit, cachedNodesInfo map[storj.NodeID]overlay.NodeReputation, privateKey storj.PiecePrivateKey, pieceSize int64) (io.ReadCloser, error) {
	info := cachedNodesInfo[limit.GetLimit().StorageNodeId]
	address := limit.GetStorageNodeAddress().GetAddress()
	var triedLastIPPort bool
	if info.LastIPPort != "" && info.LastIPPort != address {
		address = info.LastIPPort
		triedLastIPPort = true
	}

	log.Debug("attempting to fetch piece for repair",
		zap.Stringer("node_id", limit.GetLimit().StorageNodeId),
		zap.Stringer("piece_id", limit.Limit.PieceId),
		zap.Int("piece_index", index),
		zap.String("address", limit.GetStorageNodeAddress().Address),
		zap.String("last_ip_port", info.LastIPPort),
		zap.Binary("serial", limit.Limit.SerialNumber[:]))

	pieceReadCloser, _, _, err := ec.downloadAndVerifyPiece(ctx, limit, address, privateKey, "", pieceSize)
	// if piecestore dial with last ip:port failed try again with node address
	if triedLastIPPort && ErrDialFailed.Has(err) {
		if pieceReadCloser != nil {
			_ = pieceReadCloser.Close()
		}
		log.Info("repair get failed; retrying with specified hostname", zap.Error(err), zap.String("last_ip_port", info.LastIPPort), zap.String("hostname", limit.GetStorageNodeAddress().GetAddress()))
		pieceReadCloser, _, _, err = ec.downloadAndVerifyPiece(ctx, limit, limit.GetStorageNodeAddress().GetAddress(), privateKey, "", pieceSize)
	}

	if err != nil {
		if pieceReadCloser != nil {
			_ = pieceReadCloser.Close()
		}

		// Log the error when it happens
		if errors.Is(err, context.Canceled) {
			log.Debug("Download canceled due to racing",
				zap.Stringer("node_id", limit.GetLimit().StorageNodeId),
				zap.Stringer("piece_id", limit.Limit.PieceId))
			return nil, err
		}

		if ErrPieceHashVerifyFailed.Has(err) {
			log.Info("audit failed",
				zap.Stringer("node_id", limit.GetLimit().StorageNodeId),
				zap.Stringer("piece_id", limit.Limit.PieceId),
				zap.String("reason", err.Error()))
			return nil, err
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
				zap.Stringer("node_id", limit.GetLimit().StorageNodeId),
				zap.Stringer("piece_id", limit.Limit.PieceId),
				zap.Error(err))

		case audit.PieceAuditOffline:
			log.Debug("Failed to download piece for repair: dial timeout (offline)",
				zap.Stringer("node_id", limit.GetLimit().StorageNodeId),
				zap.Stringer("piece_id", limit.Limit.PieceId),
				zap.Error(err))

		case audit.PieceAuditContained:
			log.Info("Failed to download piece for repair: download timeout (contained)",
				zap.Stringer("node_id", limit.GetLimit().StorageNodeId),
				zap.Stringer("piece_id", limit.Limit.PieceId),
				zap.Error(err))

		case audit.PieceAuditUnknown:
			log.Info("Failed to download piece for repair: unknown transport error (skipped)",
				zap.Stringer("node_id", limit.GetLimit().StorageNodeId),
				zap.Stringer("piece_id", limit.Limit.PieceId),
				zap.Error(err))
		}

		return nil, err
	}

	return pieceReadCloser, nil
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

// With repairer.in-memory-repair a piece is buffered whole -- it cannot be
// checked against its signed hash until the last byte arrives -- so every
// repair allocates required-count buffers of hundreds of kilobytes to a few
// megabytes, big enough to go straight to the heap's large object path. Pool
// them by size class.
//
// The classes step through each power of two in pieceBufferSubclasses even
// steps instead of jumping straight to the next one, because in-flight
// buffers alone can fill the GOMEMLIMIT the repairer runs under: a piece of a
// maximum sized segment is a little over 2 MiB, and rounding it up to 4 MiB
// charges the limit for nearly twice what the repair needs. The tail of an
// oversized buffer is never written, so it costs no resident memory, but the
// runtime still counts it and collects harder to stay under the limit. Even
// steps cap the rounding at 1/pieceBufferSubclasses.
const (
	pieceBufferSubclassShift = 2
	pieceBufferSubclasses    = 1 << pieceBufferSubclassShift

	minPooledPieceBufferShift = 15 // 32 KiB
	maxPooledPieceBufferShift = 22 // 4 MiB, above the piece size of a maximum sized segment

	minPooledPieceBuffer = 1 << minPooledPieceBufferShift
	maxPooledPieceBuffer = 1 << maxPooledPieceBufferShift
)

var pieceBufferPools [(maxPooledPieceBufferShift-minPooledPieceBufferShift)*pieceBufferSubclasses + 1]sync.Pool

// pieceBufferClass returns the index of the pool serving buffers of at least
// size bytes, or -1 when size falls outside the pooled range.
func pieceBufferClass(size int64) int {
	if size < minPooledPieceBuffer || size > maxPooledPieceBuffer {
		return -1
	}
	// size falls in (1<<(shift-1), 1<<shift], an octave the classes divide
	// into pieceBufferSubclasses steps of step bytes.
	shift := bits.Len64(uint64(size - 1))
	step := int64(1) << (shift - pieceBufferSubclassShift - 1)
	sub := int((size+step-1)/step) - pieceBufferSubclasses
	return (shift-1-minPooledPieceBufferShift)*pieceBufferSubclasses + sub
}

// pieceBufferClassSize returns the size of the buffers a class serves.
func pieceBufferClassSize(class int) int64 {
	octave, sub := class/pieceBufferSubclasses, class%pieceBufferSubclasses
	return int64(pieceBufferSubclasses+sub) << (minPooledPieceBufferShift - pieceBufferSubclassShift + octave)
}

// getPieceBuffer returns a buffer of size bytes. It must be handed back with
// putPieceBuffer once nothing references it any more.
func getPieceBuffer(size int64) *[]byte {
	class := pieceBufferClass(size)
	if class < 0 {
		buffer := make([]byte, size)
		return &buffer
	}
	buffer, _ := pieceBufferPools[class].Get().(*[]byte)
	if buffer == nil {
		allocated := make([]byte, pieceBufferClassSize(class))
		buffer = &allocated
	}
	*buffer = (*buffer)[:size]
	return buffer
}

// putPieceBuffer returns a buffer from getPieceBuffer to its pool. A buffer
// that never came from one -- its capacity is not one of the class sizes -- is
// dropped.
func putPieceBuffer(buffer *[]byte) {
	size := cap(*buffer)
	class := pieceBufferClass(int64(size))
	if class < 0 || int64(size) != pieceBufferClassSize(class) {
		return
	}
	*buffer = (*buffer)[:size]
	// let the race detector catch any use of the buffer past this point.
	race2.WriteSlice(*buffer)
	pieceBufferPools[class].Put(buffer)
}

// pooledPieceReader reads a downloaded piece out of a pooled buffer, returning
// the buffer to the pool when it is closed.
//
// A closed reader can still be read from: the decoder closes the piece readers
// it was handed without waiting for the goroutines reading them to stop --
// eestream.StripeReader.Close only wakes those goroutines up, so one of them
// can be inside a Read, or enter one, while Close runs. The mutex orders the
// two: a Read either finishes before the buffer goes back to the pool, or finds
// the reader closed and reports EOF without touching the recycled memory.
type pooledPieceReader struct {
	mu     sync.Mutex
	reader bytes.Reader
	buffer *[]byte
}

func newPooledPieceReader(buffer *[]byte) *pooledPieceReader {
	reader := &pooledPieceReader{buffer: buffer}
	reader.reader.Reset(*buffer)
	return reader
}

// Read reads the piece out of the buffer, and reports EOF once the reader has
// been closed. It is safe to call concurrently with Close.
func (reader *pooledPieceReader) Read(p []byte) (int, error) {
	reader.mu.Lock()
	defer reader.mu.Unlock()

	if reader.buffer == nil {
		return 0, io.EOF
	}
	return reader.reader.Read(p)
}

// Close returns the buffer to the pool. It is safe to call more than once and
// concurrently with Read.
func (reader *pooledPieceReader) Close() error {
	reader.mu.Lock()
	defer reader.mu.Unlock()

	if reader.buffer != nil {
		// leave the reader empty rather than pointing at a recycled buffer.
		reader.reader.Reset(nil)
		putPieceBuffer(reader.buffer)
		reader.buffer = nil
	}
	return nil
}

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
		buffer := getPieceBuffer(pieceSize)
		n, err := io.ReadFull(downloadReader, *buffer)
		if err != nil {
			putPieceBuffer(buffer)
			return nil, nil, nil, err
		}
		downloadedPieceSize = int64(n)
		pieceReadCloser = newPooledPieceReader(buffer)
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
		zap.Duration("timer", timeout),
		zap.Int("node_count", nonNilCount(limits)),
		zap.Int("optimal_threshold", rs.OptimalThreshold()),
	)

	var successfulCount, failureCount, cancellationCount atomic.Int32
	timer := time.AfterFunc(timeout, func() {
		if !errors.Is(ctx.Err(), context.Canceled) {
			log.Debug("Timer expired. Canceling the long tail...",
				zap.Int32("successfully_repaired", successfulCount.Load()),
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
				failureCount.Add(1)
				log.Warn("Repair to a storage node failed",
					zap.Stringer("node_id", limits[info.i].GetLimit().StorageNodeId),
					zap.Error(info.err),
				)
			} else {
				cancellationCount.Add(1)
				log.Debug("Repair to storage node cancelled",
					zap.Stringer("node_id", limits[info.i].GetLimit().StorageNodeId),
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
		successCount := successfulCount.Add(1)

		if successCount >= int32(successfulNeeded) {
			// if this is logged more than once for a given repair operation, it is because
			// an upload succeeded right after we called cancel(), before that upload could
			// actually be canceled. So, successfulCount should increase by one with each
			// repeated logging.
			log.Debug("Number of successful uploads met. Canceling the long tail...",
				zap.Int32("successfully_repaired", successCount),
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

	if successfulCount.Load() == 0 {
		return nil, nil, Error.New("repair to all nodes failed")
	}

	log.Debug("Successfully repaired",
		zap.Int32("success_count", successfulCount.Load()),
	)

	mon.IntVal("repair_segment_pieces_total").Observe(int64(pieceCount))
	mon.IntVal("repair_segment_pieces_successful").Observe(int64(successfulCount.Load()))
	mon.IntVal("repair_segment_pieces_failed").Observe(int64(failureCount.Load()))
	mon.IntVal("repair_segment_pieces_canceled").Observe(int64(cancellationCount.Load()))

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
			zap.Stringer("piece_id", pieceID),
			zap.Stringer("node_id", storageNodeID),
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
					zap.Stringer("node_id", storageNodeID),
					zap.Stringer("piece_id", pieceID))
			} else {
				log.Debug("Node cut from upload due to slow connection",
					zap.Stringer("node_id", storageNodeID),
					zap.Stringer("piece_id", pieceID))
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
				zap.Stringer("piece_id", pieceID),
				zap.Stringer("node_id", storageNodeID),
				zap.String("node_address", nodeAddress),
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
