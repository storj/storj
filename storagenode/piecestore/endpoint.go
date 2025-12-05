// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package piecestore

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"net"
	"reflect"
	"runtime/trace"
	"sync"
	"sync/atomic"
	"syscall"
	"time"

	"github.com/spacemonkeygo/monkit/v3"
	"github.com/zeebo/errs"
	"go.uber.org/zap"
	"golang.org/x/sync/errgroup"

	"storj.io/common/context2"
	"storj.io/common/errs2"
	"storj.io/common/identity"
	"storj.io/common/memory"
	"storj.io/common/pb"
	"storj.io/common/rpc/rpcstatus"
	"storj.io/common/signing"
	"storj.io/common/storj"
	"storj.io/common/sync2"
	"storj.io/drpc"
	"storj.io/drpc/drpcctx"
	"storj.io/storj/shared/bloomfilter"
	"storj.io/storj/storagenode/bandwidth"
	"storj.io/storj/storagenode/blobstore/filestore"
	"storj.io/storj/storagenode/monitor"
	"storj.io/storj/storagenode/orders"
	"storj.io/storj/storagenode/orders/ordersfile"
	"storj.io/storj/storagenode/piecestore/signaturecheck"
	"storj.io/storj/storagenode/piecestore/usedserials"
	"storj.io/storj/storagenode/retain"
	"storj.io/storj/storagenode/trust"
	"storj.io/uplink/private/piecestore"
)

var (
	mon = monkit.Package()
)

// OldConfig contains everything necessary for a server.
type OldConfig struct {
	Path               string      `help:"path to store data in" default:"$CONFDIR/storage"`
	AllocatedDiskSpace memory.Size `user:"true" help:"total allocated disk space in bytes" default:"1TB"`

	// deprecated flags
	WhitelistedSatellites  storj.NodeURLs `help:"a comma-separated list of approved satellite node urls (unused)" devDefault:"" releaseDefault:"" hidden:"true" deprecated:"true"`
	AllocatedBandwidth     memory.Size    `user:"true" help:"total allocated bandwidth in bytes (deprecated)" default:"0B" hidden:"true" deprecated:"true"`
	KBucketRefreshInterval time.Duration  `help:"how frequently Kademlia bucket should be refreshed with node stats (deprecated)" default:"1h0m0s" hidden:"true" deprecated:"true"`
}

// Config defines parameters for piecestore endpoint.
type Config struct {
	DatabaseDir             string        `help:"directory to store databases. if empty, uses data path" default:""`
	ExpirationGracePeriod   time.Duration `help:"how soon before expiration date should things be considered expired" default:"48h0m0s"`
	MaxConcurrentRequests   int           `help:"how many concurrent requests are allowed, before uploads are rejected. 0 represents unlimited." default:"0"`
	OrderLimitGracePeriod   time.Duration `help:"how long after OrderLimit creation date are OrderLimits no longer accepted" default:"1h0m0s"`
	CacheSyncInterval       time.Duration `help:"how often the space used cache is synced to persistent storage" releaseDefault:"1h0m0s" devDefault:"0h1m0s"`
	PieceScanOnStartup      bool          `help:"if set to true, all pieces disk usage is recalculated on startup" default:"true"`
	StreamOperationTimeout  time.Duration `help:"how long to spend waiting for a stream operation before canceling" default:"30m"`
	ReportCapacityThreshold memory.Size   `help:"threshold below which to immediately notify satellite of capacity" default:"5GB" hidden:"true"`
	MaxUsedSerialsSize      memory.Size   `help:"amount of memory allowed for used serials store - once surpassed, serials will be dropped at random" default:"1MB"`

	MinUploadSpeed                    memory.Size   `help:"a client upload speed should not be lower than MinUploadSpeed in bytes-per-second (E.g: 1Mb), otherwise, it will be flagged as slow-connection and potentially be closed" default:"0Mb"`
	MinUploadSpeedGraceDuration       time.Duration `help:"if MinUploadSpeed is configured, after a period of time after the client initiated the upload, the server will flag unusually slow upload client" default:"0h0m10s"`
	MinUploadSpeedCongestionThreshold float64       `help:"if the portion defined by the total number of alive connection per MaxConcurrentRequest reaches this threshold, a slow upload client will no longer be monitored and flagged" default:"0.8"`

	Trust   trust.Config
	Monitor monitor.Config
	Orders  orders.Config

	// deprecated flags
	DeleteWorkers      int           `help:"how many piece delete workers (unused)" default:"1" hidden:"true" deprecated:"true"`
	DeleteQueueSize    int           `help:"size of the piece delete queue (unused)" default:"10000" hidden:"true" deprecated:"true"`
	ExistsCheckWorkers int           `help:"how many workers to use to check if satellite pieces exists (unused)" default:"5" hidden:"true" deprecated:"true"`
	RetainTimeBuffer   time.Duration `help:"allows for small differences in the satellite and storagenode clocks" default:"48h0m0s" hidden:"true" deprecated:"true"`
}

// PingStatsSource stores the last time when the target was pinged.
type PingStatsSource interface {
	WasPinged(when time.Time)
}

// Endpoint implements uploading, downloading and deleting for a storage node..
//
// architecture: Endpoint
type Endpoint struct {
	pb.DRPCContactUnimplementedServer

	log    *zap.Logger
	config Config

	ident       *identity.FullIdentity
	trustSource trust.TrustedSatelliteSource
	monitor     *monitor.Service
	retain      []QueueRetain
	pingStats   PingStatsSource

	usage       bandwidth.Writer
	ordersStore *orders.FileStore
	usedSerials *usedserials.Table

	pieceBackend   PieceBackend
	signatureCheck signaturecheck.Check

	liveRequests int32
}

// QueueRetain is an interface for retaining pieces in the queue and checking status.
// A restricted view of retain.Service.
type QueueRetain interface {
	Queue(ctx context.Context, satelliteID storj.NodeID, req *pb.RetainRequest) error
	Status() retain.Status
}

// RestoreTrash is an interface for restoring trash.
type RestoreTrash interface {
	StartRestore(ctx context.Context, satellite storj.NodeID) error
}

// NewEndpoint creates a new piecestore endpoint.
func NewEndpoint(log *zap.Logger, ident *identity.FullIdentity, trustSource trust.TrustedSatelliteSource, monitor *monitor.Service, retain []QueueRetain, pingStats PingStatsSource, pieceBackend PieceBackend, ordersStore *orders.FileStore, usage bandwidth.DB, usedSerials *usedserials.Table, signatureCheck signaturecheck.Check, config Config) (*Endpoint, error) {
	if signatureCheck == nil {
		signatureCheck = &signaturecheck.Full{}
	}
	return &Endpoint{
		log:    log,
		config: config,

		ident:       ident,
		trustSource: trustSource,
		monitor:     monitor,
		retain:      retain,
		pingStats:   pingStats,

		ordersStore: ordersStore,
		usage:       usage,
		usedSerials: usedSerials,

		pieceBackend:   pieceBackend,
		signatureCheck: signatureCheck,

		liveRequests: 0,
	}, nil
}

var monLiveRequests = mon.TaskNamed("live-request")

// Delete handles deleting a piece on piece store requested by uplink.
//
// Deprecated: use DeletePieces instead.
func (endpoint *Endpoint) Delete(ctx context.Context, delete *pb.PieceDeleteRequest) (_ *pb.PieceDeleteResponse, err error) {
	defer monLiveRequests(&ctx)(&err)
	defer mon.Task()(&ctx)(&err)

	return nil, rpcstatus.NamedErrorf("delete-unsuported", rpcstatus.Unimplemented, "delete is no longer supported")
}

// DeletePieces delete a list of pieces on satellite request.
func (endpoint *Endpoint) DeletePieces(
	ctx context.Context, req *pb.DeletePiecesRequest,
) (_ *pb.DeletePiecesResponse, err error) {
	defer mon.Task()(&ctx, req.PieceIds)(&err)

	return nil, rpcstatus.NamedErrorf("delete-pieces-unsupported", rpcstatus.Unimplemented, "delete pieces is no longer supported")
}

// Exists check if pieces from the list exists on storage node. Request will
// accept only connections from trusted satellite and will check pieces only
// for that satellite.
func (endpoint *Endpoint) Exists(
	ctx context.Context, req *pb.ExistsRequest,
) (_ *pb.ExistsResponse, err error) {
	defer mon.Task()(&ctx)(&err)

	return nil, rpcstatus.NamedErrorf("exists-unsupported", rpcstatus.Unimplemented, "exists is no longer supported")
}

var (
	monUploadHandleMessage      = mon.TaskNamed("upload-handle-message")
	monPieceWriterWrite         = mon.TaskNamed("piece-writer-write")
	monUploadStreamRecv         = mon.TaskNamed("upload-stream-recv")
	monUploadStreamSendAndClose = mon.TaskNamed("upload-stream-send-and-close")
)

// Upload handles uploading a piece on piece store.
func (endpoint *Endpoint) Upload(stream pb.DRPCPiecestore_UploadStream) (err error) {
	ctx := stream.Context()
	defer monLiveRequests(&ctx)(&err)
	defer mon.Task()(&ctx)(&err)

	if trace.IsEnabled() {
		if tr, ok := drpcctx.Transport(ctx); ok {
			if conn, ok := tr.(net.Conn); ok {
				trace.Logf(ctx, "connection-info", "local:%v remote:%v", conn.LocalAddr(), conn.RemoteAddr())
			}
		}
	}

	cancelStream, ok := getCanceler(stream)
	if !ok {
		return rpcstatus.NamedError("canceling-unsupported", rpcstatus.Unavailable, "stream does not support canceling")
	}

	liveRequests := atomic.AddInt32(&endpoint.liveRequests, 1)
	defer atomic.AddInt32(&endpoint.liveRequests, -1)

	endpoint.pingStats.WasPinged(time.Now())

	if endpoint.config.MaxConcurrentRequests > 0 && int(liveRequests) > endpoint.config.MaxConcurrentRequests {
		endpoint.log.Info("upload rejected, too many requests",
			zap.Int32("live requests", liveRequests),
			zap.Int("requestLimit", endpoint.config.MaxConcurrentRequests),
		)
		errMsg := fmt.Sprintf("storage node overloaded, request limit: %d", endpoint.config.MaxConcurrentRequests)
		return rpcstatus.NamedError("storagenode-overloaded", rpcstatus.Unavailable, errMsg)
	}

	startTime := time.Now().UTC()

	// TODO: set maximum message size

	message, err := withTimeout(ctx, endpoint.config.StreamOperationTimeout, cancelStream,
		func(ctx context.Context) (_ *pb.PieceUploadRequest, err error) {
			return stream.Recv()
		})
	switch {
	case err != nil:
		if errs2.IsCanceled(err) || errors.Is(err, io.ErrUnexpectedEOF) || errors.Is(err, io.EOF) {
			return rpcstatus.NamedWrap("canceled-or-eof", rpcstatus.Canceled, err)
		}
		if errors.Is(err, net.ErrClosed) {
			return rpcstatus.NamedWrap("closed", rpcstatus.Aborted, err)
		}
		endpoint.log.Error("upload internal error", zap.Error(err))
		return rpcstatus.NamedWrap("socket-read-failure", rpcstatus.Internal, err)
	case message == nil:
		return rpcstatus.NamedError("missing-message", rpcstatus.InvalidArgument, "expected a message")
	case message.Limit == nil:
		return rpcstatus.NamedError("missing-limit", rpcstatus.InvalidArgument, "expected order limit as the first message")
	}
	limit := message.Limit
	hashAlgorithm := message.HashAlgorithm

	if limit.Action != pb.PieceAction_PUT && limit.Action != pb.PieceAction_PUT_REPAIR {
		return rpcstatus.NamedErrorf("unexpected-action", rpcstatus.InvalidArgument, "expected put or put repair action got %v", limit.Action)
	}

	if err := endpoint.verifyOrderLimit(ctx, limit); err != nil {
		return err
	}

	availableSpace, err := endpoint.monitor.AvailableSpace(ctx)
	if err != nil {
		endpoint.log.Error("upload internal error", zap.Error(err))
		return rpcstatus.NamedWrap("available-space-failure", rpcstatus.Internal, err)
	}
	// if availableSpace has fallen below ReportCapacityThreshold, report capacity to satellites
	defer func() {
		if availableSpace < endpoint.config.ReportCapacityThreshold.Int64() {
			endpoint.monitor.NotifyLowDisk()
		}
	}()

	if availableSpace < limit.Limit {
		return rpcstatus.NamedErrorf("out-of-disk-space", rpcstatus.Aborted, "not enough available disk space, have: %v, need: %v", availableSpace, limit.Limit)
	}

	log := endpoint.log.With(
		zap.Stringer("Piece ID", limit.PieceId),
		zap.Stringer("Satellite ID", limit.SatelliteId),
		zap.Stringer("Action", limit.Action),
		zap.String("Remote Address", getRemoteAddr(ctx)))

	var pieceWriter PieceWriter
	// committed is set to true when the piece is committed.
	// It is used to distinguish successful pieces where the uplink cancels the connections,
	// and pieces that were actually canceled before being completed.
	var committed bool
	defer func() {
		endTime := time.Now().UTC()
		dt := endTime.Sub(startTime)
		uploadSize := int64(0)
		if pieceWriter != nil {
			uploadSize = pieceWriter.Size()
		}
		uploadRate := float64(0)
		if dt.Seconds() > 0 {
			uploadRate = float64(uploadSize) / dt.Seconds()
		}
		uploadDuration := dt.Nanoseconds()

		// we may return with specific named code, even if the real error is just cancelled
		if errs2.IsCanceled(err) {
			err = rpcstatus.NamedWrap("context-canceled", rpcstatus.Canceled, err)
		}

		if (errs2.IsCanceled(err) || drpc.ClosedError.Has(err)) && !committed {
			mon.Counter("upload_cancel_count").Inc(1)
			mon.Meter("upload_cancel_byte_meter").Mark64(uploadSize)
			mon.IntVal("upload_cancel_size_bytes").Observe(uploadSize)
			mon.IntVal("upload_cancel_duration_ns").Observe(uploadDuration)
			mon.FloatVal("upload_cancel_rate_bytes_per_sec").Observe(uploadRate)
			log.Info("upload canceled", zap.Int64("Size", uploadSize))
		} else if err != nil {
			mon.Counter("upload_failure_count").Inc(1)
			mon.Meter("upload_failure_byte_meter").Mark64(uploadSize)
			mon.IntVal("upload_failure_size_bytes").Observe(uploadSize)
			mon.IntVal("upload_failure_duration_ns").Observe(uploadDuration)
			mon.FloatVal("upload_failure_rate_bytes_per_sec").Observe(uploadRate)
			if errors.Is(err, context.Canceled) || errors.Is(err, io.ErrUnexpectedEOF) ||
				rpcstatus.Code(err) == rpcstatus.Canceled || rpcstatus.Code(err) == rpcstatus.Aborted {
				// This is common in normal operation, and shouldn't throw a full error.
				log.Debug("upload failed", zap.Int64("Size", uploadSize), zap.Error(err))

			} else {
				log.Error("upload failed", zap.Int64("Size", uploadSize), zap.Error(err))
			}

		} else {
			mon.Counter("upload_success_count").Inc(1)
			mon.Meter("upload_success_byte_meter").Mark64(uploadSize)
			mon.IntVal("upload_success_size_bytes").Observe(uploadSize)
			mon.IntVal("upload_success_duration_ns").Observe(uploadDuration)
			mon.FloatVal("upload_success_rate_bytes_per_sec").Observe(uploadRate)
			log.Info("uploaded", zap.Int64("Size", uploadSize))
		}
	}()

	log.Debug("upload started", zap.Int64("Available Space", availableSpace))
	mon.Counter("upload_started_count").Inc(1)

	pieceWriter, err = endpoint.pieceBackend.Writer(ctx, limit.SatelliteId, limit.PieceId, hashAlgorithm, limit.PieceExpiration)
	if err != nil {
		return rpcstatus.NamedWrap("disk-create-failure", rpcstatus.Internal, err)
	}
	defer func() {
		// cancel error if it hasn't been committed
		if cancelErr := pieceWriter.Cancel(ctx); cancelErr != nil {
			if errs2.IsCanceled(cancelErr) {
				return
			}
			log.Error("error during canceling a piece write", zap.Error(cancelErr))
		}
	}()

	// Ensure that the order is saved even in the face of an error. In the
	// success path, the order will be saved just before sending the response
	// and closing the stream (in which case, orderSaved will be true).
	commitOrderToStore, err := endpoint.beginSaveOrder(ctx, limit)
	if err != nil {
		return rpcstatus.NamedWrap("failed-to-save-order", rpcstatus.InvalidArgument, err)
	}
	largestOrder := pb.Order{}
	defer commitOrderToStore(ctx, &largestOrder, func() int64 {
		return pieceWriter.Size()
	})

	// monitor speed of upload client to flag out slow uploads.
	speedEstimate := speedEstimation{
		grace: endpoint.config.MinUploadSpeedGraceDuration,
		limit: endpoint.config.MinUploadSpeed,
	}

	handleMessage := func(ctx context.Context, message *pb.PieceUploadRequest) (done bool, err error) {
		defer monUploadHandleMessage(&ctx)(&err)

		if message.Order != nil {
			if err := endpoint.VerifyOrder(ctx, limit, message.Order, largestOrder.Amount); err != nil {
				return true, err
			}
			largestOrder = *message.Order
		}

		if message.Chunk != nil {
			if message.Chunk.Offset != pieceWriter.Size() {
				return true, rpcstatus.NamedError("out-of-order-chunk", rpcstatus.InvalidArgument, "chunk out of order")
			}

			chunkSize := int64(len(message.Chunk.Data))
			if largestOrder.Amount < pieceWriter.Size()+chunkSize {
				// TODO: should we write currently and give a chance for uplink to remedy the situation?
				return true, rpcstatus.NamedErrorf("not-enough-allocated", rpcstatus.InvalidArgument,
					"not enough allocated, allocated=%v writing=%v",
					largestOrder.Amount, pieceWriter.Size()+int64(len(message.Chunk.Data)))
			}

			availableSpace -= chunkSize
			if availableSpace < 0 {
				return true, rpcstatus.NamedError("out-of-space", rpcstatus.Internal, "out of space")
			}

			err := func() (err error) {
				defer monPieceWriterWrite(&ctx)(&err)

				_, err = pieceWriter.Write(message.Chunk.Data)
				return err
			}()
			if err != nil {
				return true, rpcstatus.NamedWrap("disk-write-failure", rpcstatus.Internal, err)
			}
		}

		if message.Done == nil {
			return false, nil
		}

		if message.Done.HashAlgorithm != hashAlgorithm {
			return true, rpcstatus.NamedWrap("hash-algo-mismatch", rpcstatus.Internal, errs.New("Hash algorithm in the first and last upload message are different %s %s", hashAlgorithm, message.Done.HashAlgorithm))
		}

		calculatedHash := pieceWriter.Hash()
		if err := endpoint.VerifyPieceHash(ctx, limit, message.Done, calculatedHash); err != nil {
			return true, rpcstatus.NamedWrap("piece-hash-verify-fail", rpcstatus.Internal, err)
		}
		if message.Done.PieceSize != pieceWriter.Size() {
			return true, rpcstatus.NamedErrorf("size-mismatch", rpcstatus.InvalidArgument,
				"Size of finished piece does not match size declared by uplink! %d != %d",
				message.Done.PieceSize, pieceWriter.Size())
		}

		{
			info := &pb.PieceHeader{
				Hash:          calculatedHash,
				HashAlgorithm: hashAlgorithm,
				CreationTime:  message.Done.Timestamp,
				Signature:     message.Done.GetSignature(),
				OrderLimit:    *limit,
			}
			if err := pieceWriter.Commit(ctx, info); err != nil {
				return true, rpcstatus.NamedWrap("commit-failure", rpcstatus.Internal, err)
			}
			committed = true
		}

		storageNodeHash, err := signing.SignPieceHash(ctx, signing.SignerFromFullIdentity(endpoint.ident), &pb.PieceHash{
			PieceId:       limit.PieceId,
			Hash:          calculatedHash,
			HashAlgorithm: hashAlgorithm,
			PieceSize:     pieceWriter.Size(),
			Timestamp:     time.Now(),
		})
		if err != nil {
			return true, rpcstatus.NamedWrap("sign-piece-hash-failure", rpcstatus.Internal, err)
		}

		_, closeErr := withTimeout(ctx, endpoint.config.StreamOperationTimeout, cancelStream,
			func(ctx context.Context) (_ any, err error) {
				defer monUploadStreamSendAndClose(&ctx)(&err)

				return nil, stream.SendAndClose(&pb.PieceUploadResponse{
					Done:          storageNodeHash,
					NodeCertchain: identity.EncodePeerIdentity(endpoint.ident.PeerIdentity()),
				})
			})
		if errs.Is(closeErr, io.EOF) {
			closeErr = nil
		}
		if closeErr != nil {
			if errs2.IsCanceled(closeErr) {
				return true, rpcstatus.NamedWrap("context-canceled", rpcstatus.Canceled, closeErr)
			}
			if errors.Is(closeErr, net.ErrClosed) {
				return true, rpcstatus.NamedWrap("closed", rpcstatus.Aborted, closeErr)
			}
			endpoint.log.Error("upload internal error", zap.Error(closeErr))
			return true, rpcstatus.NamedWrap("send-and-close-fail", rpcstatus.Internal, closeErr)
		}
		return true, nil
	}

	// handle any data in the first message we already received.
	if done, err := handleMessage(ctx, message); err != nil || done {
		return err
	}

	for {
		if endpoint.config.MinUploadSpeed > 0 {
			if err := speedEstimate.EnsureLimit(memory.Size(pieceWriter.Size()), endpoint.isCongested(), time.Now()); err != nil {
				return rpcstatus.NamedWrap("client-too-slow", rpcstatus.Aborted, err)
			}
		}

		// TODO: reuse messages to avoid allocations

		message, err := withTimeout(ctx, endpoint.config.StreamOperationTimeout, cancelStream,
			func(ctx context.Context) (_ *pb.PieceUploadRequest, err error) {
				defer monUploadStreamRecv(&ctx)(&err)

				return stream.Recv()
			})
		if errs.Is(err, io.EOF) || errors.Is(err, io.ErrUnexpectedEOF) {
			return rpcstatus.NamedError("unexpected-eof", rpcstatus.Aborted, "unexpected EOF")
		} else if err != nil {
			if errs2.IsCanceled(err) {
				return rpcstatus.NamedWrap("context-canceled", rpcstatus.Canceled, err)
			}
			endpoint.log.Error("upload internal error", zap.Error(err))
			return rpcstatus.NamedWrap("data-read-failure", rpcstatus.Internal, err)
		}
		if message == nil {
			return rpcstatus.NamedError("missing-message", rpcstatus.InvalidArgument, "expected a message")
		}

		if done, err := handleMessage(ctx, message); err != nil || done {
			return err
		}
	}
}

// isCongested identifies state of congestion. If the total number of
// connections is above 80% of the MaxConcurrentRequests, then it is defined
// as congestion.
func (endpoint *Endpoint) isCongested() bool {

	requestCongestionThreshold := int32(float64(endpoint.config.MaxConcurrentRequests) * endpoint.config.MinUploadSpeedCongestionThreshold)

	connectionCount := atomic.LoadInt32(&endpoint.liveRequests)
	return connectionCount > requestCongestionThreshold
}

// Download handles Downloading a piece on piecestore.
func (endpoint *Endpoint) Download(stream pb.DRPCPiecestore_DownloadStream) (err error) {
	ctx := stream.Context()
	defer monLiveRequests(&ctx)(&err)
	defer mon.Task()(&ctx)(&err)

	ttfb := newTimer(mon.DurationVal("download_time_to_first_byte_sent"))

	if trace.IsEnabled() {
		if tr, ok := drpcctx.Transport(ctx); ok {
			if conn, ok := tr.(net.Conn); ok {
				trace.Logf(ctx, "connection-info", "local:%v remote:%v", conn.LocalAddr(), conn.RemoteAddr())
			}
		}
	}

	cancelStream, ok := getCanceler(stream)
	if !ok {
		return rpcstatus.NamedError("cancel-unsupported", rpcstatus.Unavailable, "stream does not support canceling")
	}

	atomic.AddInt32(&endpoint.liveRequests, 1)
	defer atomic.AddInt32(&endpoint.liveRequests, -1)

	startTime := time.Now().UTC()

	endpoint.pingStats.WasPinged(time.Now())

	// TODO: set maximum message size

	message, err := withTimeout(ctx, endpoint.config.StreamOperationTimeout, cancelStream,
		func(ctx context.Context) (_ *pb.PieceDownloadRequest, err error) {
			return stream.Recv()
		})
	if err != nil {
		return rpcstatus.NamedWrap("failed-receive", rpcstatus.Internal, err)
	}
	if message.Limit == nil || message.Chunk == nil {
		return rpcstatus.NamedError("missing-limit-or-chunk", rpcstatus.InvalidArgument, "expected order limit and chunk as the first message")
	}
	limit, chunk := message.Limit, message.Chunk

	if limit.Action != pb.PieceAction_GET && limit.Action != pb.PieceAction_GET_REPAIR && limit.Action != pb.PieceAction_GET_AUDIT {
		return rpcstatus.NamedErrorf("wrong-action", rpcstatus.InvalidArgument,
			"expected get or get repair or audit action got %v", limit.Action)
	}

	if chunk.ChunkSize > limit.Limit {
		return rpcstatus.NamedErrorf("limit-exceeded", rpcstatus.InvalidArgument,
			"requested more that order limit allows, limit=%v requested=%v", limit.Limit, chunk.ChunkSize)
	}

	maximumChunkSize := 1 * memory.MiB.Int64()
	if memory.KiB.Int32() < message.MaximumChunkSize && message.MaximumChunkSize < memory.MiB.Int32() {
		maximumChunkSize = int64(message.MaximumChunkSize)
	}

	actionSeriesTag := monkit.NewSeriesTag("action", limit.Action.String())

	remoteAddr := getRemoteAddr(ctx)
	log := endpoint.log.With(
		zap.Stringer("Piece ID", limit.PieceId),
		zap.Stringer("Satellite ID", limit.SatelliteId),
		zap.Stringer("Action", limit.Action),
		zap.Int64("Offset", chunk.Offset),
		zap.Int64("Size", chunk.ChunkSize),
		zap.String("Remote Address", remoteAddr))

	log.Debug("download started")

	mon.Counter("download_started_count", actionSeriesTag).Inc(1)

	if err := endpoint.verifyOrderLimit(ctx, limit); err != nil {
		mon.Counter("download_failure_count", actionSeriesTag).Inc(1)
		mon.Meter("download_verify_orderlimit_failed", actionSeriesTag).Mark(1)
		return err
	}

	var pieceReader PieceReader
	downloadedBytes := make(chan int64, 1)
	largestOrder := pb.Order{}
	defer func() {
		endTime := time.Now().UTC()
		dt := endTime.Sub(startTime)
		downloadRate := float64(0)
		downloadSize := <-downloadedBytes
		if dt.Seconds() > 0 {
			downloadRate = float64(downloadSize) / dt.Seconds()
		}
		downloadDuration := dt.Nanoseconds()
		// NOTE: Check if the `switch` statement inside of this conditional block must be updated if you
		// change this condition.
		if errs2.IsCanceled(err) || drpc.ClosedError.Has(err) || (err == nil && chunk.ChunkSize != downloadSize) || errors.Is(err, syscall.ECONNRESET) || errors.Is(err, net.ErrClosed) {
			mon.Counter("download_cancel_count", actionSeriesTag).Inc(1)
			mon.Meter("download_cancel_byte_meter", actionSeriesTag).Mark64(downloadSize)
			mon.IntVal("download_cancel_size_bytes", actionSeriesTag).Observe(downloadSize)
			mon.IntVal("download_cancel_duration_ns", actionSeriesTag).Observe(downloadDuration)
			mon.FloatVal("download_cancel_rate_bytes_per_sec", actionSeriesTag).Observe(downloadRate)

			var reason string

			// NOTE: This switch must capture all the possible reasons for a download to be canceled and
			// set the `reason` variable to an appropriate message in order of never reaching the
			// `default` case.
			switch {
			case errs2.IsCanceled(err):
				reason = "context canceled"
			case drpc.ClosedError.Has(err):
				reason = "stream closed by peer"
			case err == nil && chunk.ChunkSize != downloadSize:
				reason = fmt.Sprintf(
					"downloaded size (%d bytes) does not match received message size (%d bytes)", downloadSize, chunk.ChunkSize,
				)
			default:
				// This counter should always be 0, if it's not, it means that there is a bug in the code.
				// If we found that's greater than 0 and we change the code to potentially fix the bug, then
				// we should increment the counter's name post fix vX as a way to reset back to 0 to see if
				// the new changes fixed the bug.
				mon.Counter("download_cancel_unknown_reason_v1", actionSeriesTag).Inc(1)
				reason = "unknown reason bug in code, please report"
			}

			log.Info("download canceled", zap.String("reason", reason))
		} else if err != nil {
			mon.Counter("download_failure_count", actionSeriesTag).Inc(1)
			mon.Meter("download_failure_byte_meter", actionSeriesTag).Mark64(downloadSize)
			mon.IntVal("download_failure_size_bytes", actionSeriesTag).Observe(downloadSize)
			mon.IntVal("download_failure_duration_ns", actionSeriesTag).Observe(downloadDuration)
			mon.FloatVal("download_failure_rate_bytes_per_sec", actionSeriesTag).Observe(downloadRate)
			if errors.Is(err, context.Canceled) {
				log.Info("download canceled (race lost or node shutdown)")
				log.Debug("download canceled", zap.Error(err))
			} else {
				log.Error("download failed", zap.Error(err))
			}
		} else {
			mon.Counter("download_success_count", actionSeriesTag).Inc(1)
			mon.Meter("download_success_byte_meter", actionSeriesTag).Mark64(downloadSize)
			mon.IntVal("download_success_size_bytes", actionSeriesTag).Observe(downloadSize)
			mon.IntVal("download_success_duration_ns", actionSeriesTag).Observe(downloadDuration)
			mon.FloatVal("download_success_rate_bytes_per_sec", actionSeriesTag).Observe(downloadRate)
			log.Info("downloaded")
		}
		mon.IntVal("download_orders_amount", actionSeriesTag).Observe(largestOrder.Amount)

		if pieceReader != nil && pieceReader.Trash() {
			mon.Meter("download_file_in_trash", monkit.NewSeriesTag("namespace", limit.SatelliteId.String())).Mark(1)
			filestore.MonFileInTrash(limit.SatelliteId[:]).Mark(1)
			log.Warn("file found in trash")
		}
	}()
	defer func() {
		close(downloadedBytes)
	}()

	pieceReader, err = endpoint.pieceBackend.Reader(ctx, limit.SatelliteId, limit.PieceId)
	if err != nil {
		if errs.Is(err, fs.ErrNotExist) {
			return rpcstatus.NamedWrap("file-not-found", rpcstatus.NotFound, err)
		}
		return rpcstatus.NamedWrap("open-failed", rpcstatus.Internal, err)
	}
	defer func() {
		err := pieceReader.Close() // similarly how transaction Rollback works
		if err != nil {
			if errs2.IsCanceled(err) {
				return
			}
			// no reason to report this error to the uplink
			log.Error("failed to close piece reader", zap.Error(err))
		}
	}()

	// for repair traffic, send along the PieceHash and original OrderLimit for validation
	// before sending the piece itself
	if message.Limit.Action == pb.PieceAction_GET_REPAIR {
		pieceHash, orderLimit, err := pieceHashAndOrderLimitFromReader(pieceReader)
		if err != nil {
			log.Error("could not get hash and order limit", zap.Error(err))
			return rpcstatus.NamedWrap("hash-and-order-read-failure", rpcstatus.Internal, err)
		}

		_, err = withTimeout(ctx, endpoint.config.StreamOperationTimeout, cancelStream,
			func(ctx context.Context) (_ any, err error) {
				return nil, stream.Send(&pb.PieceDownloadResponse{
					Hash:              &pieceHash,
					Limit:             &orderLimit,
					RestoredFromTrash: pieceReader.Trash(),
				})
			})
		if err != nil {
			log.Error("error sending hash and order limit", zap.Error(err))
			return rpcstatus.NamedWrap("hash-and-order-send-failure", rpcstatus.Internal, err)
		}
	} else if pieceReader.Trash() {
		// notify that the piece was restored from trash
		_, err = withTimeout(ctx, endpoint.config.StreamOperationTimeout, cancelStream,
			func(ctx context.Context) (_ any, err error) {
				return nil, stream.Send(&pb.PieceDownloadResponse{
					RestoredFromTrash: pieceReader.Trash(),
				})
			})
		if err != nil {
			log.Error("error sending response", zap.Error(err))
			return rpcstatus.NamedWrap("send-failure", rpcstatus.Internal, err)
		}
	}

	// TODO: verify chunk.Size behavior logic with regards to reading all
	if chunk.Offset+chunk.ChunkSize > pieceReader.Size() {
		return rpcstatus.NamedErrorf("file-size-exceeded", rpcstatus.InvalidArgument,
			"requested more data than available, requesting=%v available=%v",
			chunk.Offset+chunk.ChunkSize, pieceReader.Size())
	}

	throttle := sync2.NewThrottle()
	// TODO: see whether this can be implemented without a goroutine

	group, ctx := errgroup.WithContext(ctx)
	group.Go(func() (err error) {
		currentOffset := chunk.Offset
		unsentAmount := chunk.ChunkSize

		defer func() {
			downloadedBytes <- chunk.ChunkSize - unsentAmount
		}()

		for unsentAmount > 0 {
			tryToSend := min(unsentAmount, maximumChunkSize)

			// TODO: add timeout here
			chunkSize, err := throttle.ConsumeOrWait(tryToSend)
			if err != nil {
				// this can happen only because uplink decided to close the connection
				return nil // We don't need to return an error when client cancels.
			}

			done, err := endpoint.sendData(ctx, log, stream, pieceReader, currentOffset, chunkSize)
			ttfb.Trigger()
			if err != nil || done {
				return err
			}

			currentOffset += chunkSize
			unsentAmount -= chunkSize
		}
		return nil
	})

	recvErr := func() (err error) {
		commitOrderToStore, err := endpoint.beginSaveOrder(ctx, limit)
		if err != nil {
			return err
		}
		defer func() {
			order := &largestOrder
			commitOrderToStore(ctx, order, func() int64 {
				// for downloads, we store the order amount for the egress graph instead
				// of the bytes actually downloaded
				return order.Amount
			})
		}()

		// ensure that we always terminate sending goroutine
		defer throttle.Fail(io.EOF)

		handleOrder := func(order *pb.Order) error {
			if err := endpoint.VerifyOrder(ctx, limit, order, largestOrder.Amount); err != nil {
				return err
			}
			chunkSize := order.Amount - largestOrder.Amount
			if err := throttle.Produce(chunkSize); err != nil {
				// shouldn't happen since only receiving side is calling Fail
				return rpcstatus.NamedWrap("throttle-produce-fail", rpcstatus.Internal, err)
			}
			largestOrder = *order
			return nil
		}

		if message.Order != nil {
			if err = handleOrder(message.Order); err != nil {
				return err
			}
		}

		for {
			message, err := withTimeout(ctx, endpoint.config.StreamOperationTimeout, cancelStream,
				func(ctx context.Context) (_ *pb.PieceDownloadRequest, err error) {
					return stream.Recv()
				})
			if errs.Is(err, io.EOF) {
				// err is io.EOF or canceled when uplink closed the connection, no need to return error
				return nil
			}
			if errs2.IsCanceled(err) {
				return nil
			}
			if err != nil {
				return rpcstatus.NamedWrap("data-receive-fail", rpcstatus.Internal, err)
			}

			if message == nil || message.Order == nil {
				return rpcstatus.NamedError("missing-order", rpcstatus.InvalidArgument, "expected order as the message")
			}

			if err = handleOrder(message.Order); err != nil {
				return err
			}
		}
	}()

	// ensure we wait for sender to complete
	sendErr := group.Wait()
	return rpcstatus.NamedWrap("send-or-recv-fail", rpcstatus.Internal, errs.Combine(sendErr, recvErr))
}

func (endpoint *Endpoint) sendData(ctx context.Context, log *zap.Logger, stream pb.DRPCPiecestore_DownloadStream, pieceReader PieceReader, currentOffset int64, chunkSize int64) (result bool, err error) {
	defer mon.Task()(&ctx)(&err)

	cancelStream, ok := getCanceler(stream)
	if !ok {
		return true, rpcstatus.NamedError("cancel-unsupported", rpcstatus.Unavailable, "stream does not support canceling")
	}

	chunkData := make([]byte, chunkSize)
	_, err = pieceReader.Seek(currentOffset, io.SeekStart)
	if err != nil {
		log.Error("error seeking on piecereader", zap.Error(err))
		return true, rpcstatus.NamedWrap("seek-fail", rpcstatus.Internal, err)
	}

	// ReadFull is required to ensure we are sending the right amount of data.
	_, err = io.ReadFull(pieceReader, chunkData)
	if err != nil {
		// Client closed the connection before we had a chance to send back drpc answer.
		if errors.Is(err, net.ErrClosed) {
			return true, rpcstatus.NamedWrap("closed", rpcstatus.Aborted, err)
		}
		log.Error("error reading from piecereader", zap.Error(err))
		return true, rpcstatus.NamedWrap("read-fail", rpcstatus.Internal, err)
	}

	_, err = withTimeout(ctx, endpoint.config.StreamOperationTimeout, cancelStream,
		func(ctx context.Context) (_ any, err error) {
			return nil, stream.Send(&pb.PieceDownloadResponse{
				Chunk: &pb.PieceDownloadResponse_Chunk{
					Offset: currentOffset,
					Data:   chunkData,
				},
			})
		})
	if errs.Is(err, io.EOF) {
		// err is io.EOF when uplink asked for a piece, but decided not to retrieve it,
		// no need to propagate it
		return true, nil
	}
	if err != nil {
		if errs.Is(err, context.DeadlineExceeded) {
			return true, rpcstatus.NamedWrap("send-deadline-exceeded", rpcstatus.DeadlineExceeded, err)
		}
		return true, rpcstatus.NamedWrap("send-fail", rpcstatus.Internal, err)
	}
	return false, nil
}

var monBeginSaveOrder = mon.Task()

// beginSaveOrder saves the order with all necessary information. It assumes it has been already verified.
func (endpoint *Endpoint) beginSaveOrder(ctx context.Context, limit *pb.OrderLimit) (_commit func(ctx context.Context, order *pb.Order, amountFunc func() int64), err error) {
	defer monBeginSaveOrder(&ctx)(&err)

	commit, err := endpoint.ordersStore.BeginEnqueue(limit.SatelliteId, limit.OrderCreation)
	if err != nil {
		return nil, err
	}

	done := false
	return func(ctx context.Context, order *pb.Order, amountFunc func() int64) {
		if done {
			return
		}
		done = true

		if order == nil || order.Amount <= 0 {
			// free unsent orders file for sending without writing anything
			err = commit(nil)
			if err != nil {
				endpoint.log.Error("failed to unlock orders file", zap.Error(err))
			}
			return
		}

		err = commit(&ordersfile.Info{Limit: limit, Order: order})
		if err != nil {
			endpoint.log.Error("failed to add order", zap.Error(err))
		} else if endpoint.usage != nil {
			amount := order.Amount
			if amountFunc != nil {
				amount = amountFunc()
			}
			err = endpoint.usage.Add(context2.WithoutCancellation(ctx), limit.SatelliteId, limit.Action, amount, time.Now())
			if err != nil {
				endpoint.log.Error("failed to add bandwidth usage", zap.Error(err))
			}
		}

	}, nil
}

// RestoreTrash restores all trashed items for the satellite issuing the call.
func (endpoint *Endpoint) RestoreTrash(ctx context.Context, restoreTrashReq *pb.RestoreTrashRequest) (res *pb.RestoreTrashResponse, err error) {
	defer mon.Task()(&ctx)(&err)

	peer, err := identity.PeerIdentityFromContext(ctx)
	if err != nil {
		return nil, rpcstatus.NamedWrap("no-peer", rpcstatus.Unauthenticated, err)
	}

	err = endpoint.trustSource.VerifySatelliteID(ctx, peer.ID)
	if err != nil {
		return nil, rpcstatus.NamedError("untrusted-sat", rpcstatus.PermissionDenied, "RestoreTrash called with untrusted ID")
	}

	err = endpoint.pieceBackend.StartRestore(ctx, peer.ID)
	if err != nil {
		return nil, rpcstatus.NamedError("restore-fail", rpcstatus.Internal, "failed to start restore")
	}

	return &pb.RestoreTrashResponse{}, nil
}

// Retain keeps only piece ids specified in the request.
func (endpoint *Endpoint) Retain(ctx context.Context, retainReq *pb.RetainRequest) (res *pb.RetainResponse, err error) {
	defer mon.Task()(&ctx)(&err)
	peer, err := identity.PeerIdentityFromContext(ctx)
	if err != nil {
		return nil, rpcstatus.NamedWrap("no-peer", rpcstatus.Unauthenticated, err)
	}

	err = endpoint.trustSource.VerifySatelliteID(ctx, peer.ID)
	if err != nil {
		return nil, rpcstatus.NamedErrorf("untrusted-sat", rpcstatus.PermissionDenied, "retain called with untrusted ID")
	}

	if len(retainReq.Hash) > 0 {
		hasher := pb.NewHashFromAlgorithm(retainReq.HashAlgorithm)
		_, err := hasher.Write(retainReq.GetFilter())
		if err != nil {
			return nil, rpcstatus.NamedWrap("hash-internal-err", rpcstatus.Internal, err)
		}
		if !bytes.Equal(retainReq.Hash, hasher.Sum(nil)) {
			return nil, rpcstatus.NamedWrap("hash-mismatch", rpcstatus.Internal, errs.New("hash mismatch"))
		}
	}

	return endpoint.processRetainReq(ctx, peer.ID, retainReq)
}

func (endpoint *Endpoint) processRetainReq(ctx context.Context, peerID storj.NodeID, retainReq *pb.RetainRequest) (res *pb.RetainResponse, err error) {
	filter, err := bloomfilter.NewFromBytes(retainReq.GetFilter())
	if err != nil {
		return nil, rpcstatus.NamedWrap("invalid-bf", rpcstatus.InvalidArgument, err)
	}
	filterHashCount, _ := filter.Parameters()
	mon.IntVal("retain_filter_size").Observe(filter.Size())
	mon.IntVal("retain_filter_hash_count").Observe(int64(filterHashCount))
	mon.IntVal("retain_creation_date").Observe(retainReq.CreationDate.Unix())

	endpoint.log.Info("New bloomfilter is received", zap.Stringer("satellite", peerID), zap.Time("creation", retainReq.CreationDate))

	for _, qr := range endpoint.retain {
		// the queue function will update the created before time based on the configurable retain buffer
		if qr.Status() != retain.Disabled {
			err = qr.Queue(ctx, peerID, retainReq)
			if err != nil {
				endpoint.log.Info("failed to set bloom filter", zap.Error(err), zap.String("queue", fmt.Sprintf("%T", qr)))
			}
		}
	}

	return &pb.RetainResponse{}, err
}

// RetainBig keeps only piece ids specified in the request, supports big bloom filters.
func (endpoint *Endpoint) RetainBig(stream pb.DRPCPiecestore_RetainBigStream) (err error) {
	ctx := stream.Context()
	defer mon.Task()(&ctx)(&err)
	defer func() {
		_ = stream.Close()
	}()

	peer, err := identity.PeerIdentityFromContext(ctx)
	if err != nil {
		return rpcstatus.NamedWrap("no-peer", rpcstatus.Unauthenticated, err)
	}

	err = endpoint.trustSource.VerifySatelliteID(ctx, peer.ID)
	if err != nil {
		return rpcstatus.NamedErrorf("untrusted-sat", rpcstatus.PermissionDenied, "retain called with untrusted ID")
	}
	retainReq, err := piecestore.RetainRequestFromStream(stream)
	if err != nil {
		return rpcstatus.NamedWrap("unable-to-get-retain", rpcstatus.Internal, err)
	}
	_, err = endpoint.processRetainReq(ctx, peer.ID, &retainReq)
	return err
}

// TestLiveRequestCount returns the current number of live requests.
func (endpoint *Endpoint) TestLiveRequestCount() int32 {
	return atomic.LoadInt32(&endpoint.liveRequests)
}

// speedEstimation monitors state of incoming traffic. It would signal slow-speed
// client in non-congested traffic condition.
type speedEstimation struct {
	// grace indicates a certain period of time before the observator kicks in
	grace time.Duration
	// limit for flagging slow connections. Speed below this limit is considered to be slow.
	limit memory.Size
	// uncongestedTime indicates the duration of connection, measured in non-congested state
	uncongestedTime time.Duration
	lastChecked     time.Time
}

// EnsureLimit makes sure that in non-congested condition, a slow-upload client will be flagged out.
func (estimate *speedEstimation) EnsureLimit(transferred memory.Size, congested bool, now time.Time) error {
	if estimate.limit == 0 {
		return nil
	}

	if estimate.lastChecked.IsZero() {
		estimate.lastChecked = now
		return nil
	}

	delta := now.Sub(estimate.lastChecked)
	estimate.lastChecked = now

	// In congested condition, the speed check would produce false-positive results,
	// thus it shall be skipped.
	if congested {
		return nil
	}

	estimate.uncongestedTime += delta
	if estimate.uncongestedTime <= 0 || estimate.uncongestedTime <= estimate.grace {
		// not enough data
		return nil
	}
	bytesPerSec := float64(transferred) / estimate.uncongestedTime.Seconds()

	if bytesPerSec < float64(estimate.limit) {
		return errs.New("speed too low, current:%v < limit:%v", bytesPerSec, estimate.limit)
	}

	return nil
}

// getRemoteAddr returns the remote address from the request context.
func getRemoteAddr(ctx context.Context) string {
	if transport, ok := drpcctx.Transport(ctx); ok {
		if conn, ok := transport.(net.Conn); ok {
			return conn.RemoteAddr().String()
		}
	}
	return ""
}

// getCanceler takes in a drpc.Stream and returns the first `Cancel(err) bool` method found during
// unwrapping the stream with the `Stream() drpc.Stream` method.
func getCanceler(stream drpc.Stream) (func(error) bool, bool) {
	for {
		canceler, ok := stream.(interface{ Cancel(err error) bool })
		if ok {
			return canceler.Cancel, true
		}

		// try to unwrap with GetStream if possible
		streamer, ok := stream.(interface{ GetStream() drpc.Stream })
		if ok {
			stream = streamer.GetStream()
			continue
		}

		// try to unwrap by looking for a Stream field
		next, ok := func() (s drpc.Stream, ok bool) {
			defer func() { _ = recover() }()
			s, ok = reflect.ValueOf(stream).Elem().FieldByName("Stream").Interface().(drpc.Stream)
			return s, ok
		}()
		if ok {
			stream = next
			continue
		}

		return nil, false
	}
}

// withTimeout runs fn and calls cancel in its own goroutine after the timeout has passed or the
// parent context is canceled. It only ever returns the error from fn.
func withTimeout[T any](ctx context.Context, timeout time.Duration, cancel func(error) bool, fn func(context.Context) (T, error)) (T, error) {
	ctx, cleanup := context.WithTimeout(ctx, timeout)
	defer cleanup()

	var once sync.Once
	defer once.Do(func() {})

	go func() {
		<-ctx.Done()
		once.Do(func() { cancel(ctx.Err()) })
	}()

	return fn(ctx)
}
