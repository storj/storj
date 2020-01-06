// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package piecestore

import (
	"context"
	"io"
	"os"
	"strings"
	"sync/atomic"
	"time"

	"github.com/zeebo/errs"
	"go.uber.org/zap"
	"golang.org/x/sync/errgroup"
	monkit "gopkg.in/spacemonkeygo/monkit.v2"

	"storj.io/common/bloomfilter"
	"storj.io/common/errs2"
	"storj.io/common/identity"
	"storj.io/common/memory"
	"storj.io/common/pb"
	"storj.io/common/rpc/rpcstatus"
	"storj.io/common/signing"
	"storj.io/common/storj"
	"storj.io/common/sync2"
	"storj.io/storj/storagenode/bandwidth"
	"storj.io/storj/storagenode/monitor"
	"storj.io/storj/storagenode/orders"
	"storj.io/storj/storagenode/pieces"
	"storj.io/storj/storagenode/retain"
	"storj.io/storj/storagenode/trust"
)

var (
	mon = monkit.Package()

	// Error is the default error class for piecestore errors
	Error = errs.Class("piecestore")
	// ErrProtocol is the default error class for protocol errors.
	ErrProtocol = errs.Class("piecestore protocol")
	// ErrInternal is the default error class for internal piecestore errors.
	ErrInternal = errs.Class("piecestore internal")
)

var _ pb.PiecestoreServer = (*Endpoint)(nil)

// OldConfig contains everything necessary for a server
type OldConfig struct {
	Path                   string         `help:"path to store data in" default:"$CONFDIR/storage"`
	WhitelistedSatellites  storj.NodeURLs `help:"a comma-separated list of approved satellite node urls (unused)" devDefault:"" releaseDefault:""`
	AllocatedDiskSpace     memory.Size    `user:"true" help:"total allocated disk space in bytes" default:"1TB"`
	AllocatedBandwidth     memory.Size    `user:"true" help:"total allocated bandwidth in bytes" default:"2TB"`
	KBucketRefreshInterval time.Duration  `help:"how frequently Kademlia bucket should be refreshed with node stats" default:"1h0m0s"`
}

// Config defines parameters for piecestore endpoint.
type Config struct {
	ExpirationGracePeriod time.Duration `help:"how soon before expiration date should things be considered expired" default:"48h0m0s"`
	MaxConcurrentRequests int           `help:"how many concurrent requests are allowed, before uploads are rejected. 0 represents unlimited." default:"0"`
	OrderLimitGracePeriod time.Duration `help:"how long after OrderLimit creation date are OrderLimits no longer accepted" default:"24h0m0s"`
	CacheSyncInterval     time.Duration `help:"how often the space used cache is synced to persistent storage" releaseDefault:"1h0m0s" devDefault:"0h1m0s"`

	RetainTimeBuffer time.Duration `help:"allows for small differences in the satellite and storagenode clocks" default:"48h0m0s"`

	Trust trust.Config

	Monitor monitor.Config
	Orders  orders.Config
}

type pingStatsSource interface {
	WasPinged(when time.Time)
}

// Endpoint implements uploading, downloading and deleting for a storage node..
//
// architecture: Endpoint
type Endpoint struct {
	log          *zap.Logger
	config       Config
	grpcReqLimit int

	signer    signing.Signer
	trust     *trust.Pool
	monitor   *monitor.Service
	retain    *retain.Service
	pingStats pingStatsSource

	store       *pieces.Store
	orders      orders.DB
	usage       bandwidth.DB
	usedSerials UsedSerials

	// liveRequests tracks the total number of incoming rpc requests. For gRPC
	// requests only, this number is compared to config.MaxConcurrentRequests
	// and limits the number of gRPC requests. dRPC requests are tracked but
	// not limited.
	liveRequests int32
}

// drpcEndpoint wraps streaming methods so that they can be used with drpc
type drpcEndpoint struct{ *Endpoint }

// DRPC returns a DRPC form of the endpoint.
func (endpoint *Endpoint) DRPC() pb.DRPCPiecestoreServer { return &drpcEndpoint{Endpoint: endpoint} }

// NewEndpoint creates a new piecestore endpoint.
func NewEndpoint(log *zap.Logger, signer signing.Signer, trust *trust.Pool, monitor *monitor.Service, retain *retain.Service, pingStats pingStatsSource, store *pieces.Store, orders orders.DB, usage bandwidth.DB, usedSerials UsedSerials, config Config) (*Endpoint, error) {
	// If config.MaxConcurrentRequests is set we want to repsect it for grpc.
	// However, if it is 0 (unlimited) we force a limit.
	grpcReqLimit := config.MaxConcurrentRequests
	if grpcReqLimit <= 0 {
		grpcReqLimit = 7
	}

	return &Endpoint{
		log:          log,
		config:       config,
		grpcReqLimit: grpcReqLimit,

		signer:    signer,
		trust:     trust,
		monitor:   monitor,
		retain:    retain,
		pingStats: pingStats,

		store:       store,
		orders:      orders,
		usage:       usage,
		usedSerials: usedSerials,

		liveRequests: 0,
	}, nil
}

var monLiveRequests = mon.TaskNamed("live-request")

// Delete handles deleting a piece on piece store requested by uplink.
//
// DEPRECATED in favor of DeletePieces.
func (endpoint *Endpoint) Delete(ctx context.Context, delete *pb.PieceDeleteRequest) (_ *pb.PieceDeleteResponse, err error) {
	defer monLiveRequests(&ctx)(&err)
	defer mon.Task()(&ctx)(&err)

	atomic.AddInt32(&endpoint.liveRequests, 1)
	defer atomic.AddInt32(&endpoint.liveRequests, -1)

	endpoint.pingStats.WasPinged(time.Now())

	if delete.Limit.Action != pb.PieceAction_DELETE {
		return nil, Error.New("expected delete action got %v", delete.Limit.Action) // TODO: report rpc status unauthorized or bad request
	}

	if err := endpoint.verifyOrderLimit(ctx, delete.Limit); err != nil {
		// TODO: report rpc status unauthorized or bad request
		return nil, Error.Wrap(err)
	}

	if err := endpoint.store.Delete(ctx, delete.Limit.SatelliteId, delete.Limit.PieceId); err != nil {
		// explicitly ignoring error because the errors

		// TODO: https://storjlabs.atlassian.net/browse/V3-3222
		// report rpc status of internal server error or not found error,
		// e.g. not found might happen when we get a deletion request after garbage
		// collection has deleted it
		endpoint.log.Error("delete failed", zap.Stringer("Satellite ID", delete.Limit.SatelliteId), zap.Stringer("Piece ID", delete.Limit.PieceId), zap.Error(err))
	} else {
		endpoint.log.Info("deleted", zap.Stringer("Satellite ID", delete.Limit.SatelliteId), zap.Stringer("Piece ID", delete.Limit.PieceId))
	}

	return &pb.PieceDeleteResponse{}, nil
}

// DeletePieces delete a list of pieces on satellite request.
func (endpoint *Endpoint) DeletePieces(
	ctx context.Context, req *pb.DeletePiecesRequest,
) (_ *pb.DeletePiecesResponse, err error) {
	defer mon.Task()(&ctx, req.PieceIds)(&err)

	peer, err := identity.PeerIdentityFromContext(ctx)
	if err != nil {
		return nil, rpcstatus.Error(rpcstatus.Unauthenticated, Error.Wrap(err).Error())
	}

	err = endpoint.trust.VerifySatelliteID(ctx, peer.ID)
	if err != nil {
		return nil, rpcstatus.Error(rpcstatus.PermissionDenied,
			Error.New("%s", "delete pieces called with untrusted ID").Error(),
		)
	}

	for _, pieceID := range req.PieceIds {
		err = endpoint.store.Delete(ctx, peer.ID, pieceID)
		if err != nil {
			// If a piece cannot be deleted, we just log the error.
			// No error is returned to the caller.
			endpoint.log.Error("delete failed",
				zap.Stringer("Satellite ID", peer.ID),
				zap.Stringer("Piece ID", pieceID),
				zap.Error(Error.Wrap(err)),
			)
		} else {
			endpoint.log.Info("deleted",
				zap.Stringer("Satellite ID", peer.ID),
				zap.Stringer("Piece ID", pieceID),
			)
		}
	}
	return &pb.DeletePiecesResponse{}, nil
}

// DeletePiece handles deleting a piece on piece store requested by satellite.
//
// DEPRECATED in favor of DeletePieces.
func (endpoint *Endpoint) DeletePiece(
	ctx context.Context, req *pb.PieceDeletePieceRequest,
) (_ *pb.PieceDeletePieceResponse, err error) {
	defer mon.Task()(&ctx, req.PieceId.String())(&err)

	peer, err := identity.PeerIdentityFromContext(ctx)
	if err != nil {
		return nil, rpcstatus.Error(rpcstatus.Unauthenticated, Error.Wrap(err).Error())
	}

	err = endpoint.trust.VerifySatelliteID(ctx, peer.ID)
	if err != nil {
		return nil, rpcstatus.Error(rpcstatus.PermissionDenied,
			Error.New("%s", "delete piece called with untrusted ID").Error(),
		)
	}

	err = endpoint.store.Delete(ctx, peer.ID, req.PieceId)
	if err != nil {
		// TODO: https://storjlabs.atlassian.net/browse/V3-3222
		// Once this method returns error classes change the following conditional
		if strings.Contains(err.Error(), "file does not exist") {
			return nil, rpcstatus.Error(rpcstatus.NotFound, "piece not found")
		}

		endpoint.log.Error("delete failed",
			zap.Error(Error.Wrap(err)),
			zap.Stringer("Satellite ID", peer.ID),
			zap.Stringer("Piece ID", req.PieceId),
		)

		return nil, rpcstatus.Error(rpcstatus.Internal,
			Error.New("%s", "delete failed").Error(),
		)
	}

	endpoint.log.Info("deleted",
		zap.Stringer("Satellite ID", peer.ID),
		zap.Stringer("Piece ID", req.PieceId),
	)

	return &pb.PieceDeletePieceResponse{}, nil
}

// Upload handles uploading a piece on piece store.
func (endpoint *Endpoint) Upload(stream pb.Piecestore_UploadServer) (err error) {
	return endpoint.doUpload(stream, endpoint.grpcReqLimit)
}

// Upload handles uploading a piece on piece store.
func (endpoint *drpcEndpoint) Upload(stream pb.DRPCPiecestore_UploadStream) (err error) {
	return endpoint.doUpload(stream, endpoint.config.MaxConcurrentRequests)
}

// uploadStream is the minimum interface required to perform settlements.
type uploadStream interface {
	Context() context.Context
	Recv() (*pb.PieceUploadRequest, error)
	SendAndClose(*pb.PieceUploadResponse) error
}

// doUpload handles uploading a piece on piece store.
func (endpoint *Endpoint) doUpload(stream uploadStream, requestLimit int) (err error) {
	ctx := stream.Context()
	defer monLiveRequests(&ctx)(&err)
	defer mon.Task()(&ctx)(&err)

	liveRequests := atomic.AddInt32(&endpoint.liveRequests, 1)
	defer atomic.AddInt32(&endpoint.liveRequests, -1)

	endpoint.pingStats.WasPinged(time.Now())

	if requestLimit > 0 && int(liveRequests) > requestLimit {
		endpoint.log.Error("upload rejected, too many requests", zap.Int32("live requests", liveRequests))
		return rpcstatus.Error(rpcstatus.Unavailable, "storage node overloaded")
	}

	startTime := time.Now().UTC()

	// TODO: set connection timeouts
	// TODO: set maximum message size

	var message *pb.PieceUploadRequest

	message, err = stream.Recv()
	switch {
	case err != nil:
		return ErrProtocol.Wrap(err)
	case message == nil:
		return ErrProtocol.New("expected a message")
	case message.Limit == nil:
		return ErrProtocol.New("expected order limit as the first message")
	}
	limit := message.Limit
	endpoint.log.Info("upload started", zap.Stringer("Piece ID", limit.PieceId), zap.Stringer("Satellite ID", limit.SatelliteId), zap.Stringer("Action", limit.Action))

	// TODO: verify that we have have expected amount of storage before continuing

	if limit.Action != pb.PieceAction_PUT && limit.Action != pb.PieceAction_PUT_REPAIR {
		return ErrProtocol.New("expected put or put repair action got %v", limit.Action) // TODO: report rpc status unauthorized or bad request
	}

	if err := endpoint.verifyOrderLimit(ctx, limit); err != nil {
		return err
	}

	var pieceWriter *pieces.Writer
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

		if err != nil {
			mon.Meter("upload_failure_byte_meter").Mark64(uploadSize)
			mon.IntVal("upload_failure_size_bytes").Observe(uploadSize)
			mon.IntVal("upload_failure_duration_ns").Observe(uploadDuration)
			mon.FloatVal("upload_failure_rate_bytes_per_sec").Observe(uploadRate)
			endpoint.log.Info("upload failed", zap.Stringer("Piece ID", limit.PieceId), zap.Stringer("Satellite ID", limit.SatelliteId), zap.Stringer("Action", limit.Action), zap.Error(err))
		} else {
			mon.Meter("upload_success_byte_meter").Mark64(uploadSize)
			mon.IntVal("upload_success_size_bytes").Observe(uploadSize)
			mon.IntVal("upload_success_duration_ns").Observe(uploadDuration)
			mon.FloatVal("upload_success_rate_bytes_per_sec").Observe(uploadRate)
			endpoint.log.Info("uploaded", zap.Stringer("Piece ID", limit.PieceId), zap.Stringer("Satellite ID", limit.SatelliteId), zap.Stringer("Action", limit.Action))
		}
	}()

	pieceWriter, err = endpoint.store.Writer(ctx, limit.SatelliteId, limit.PieceId)
	if err != nil {
		return ErrInternal.Wrap(err) // TODO: report rpc status internal server error
	}
	defer func() {
		// cancel error if it hasn't been committed
		if cancelErr := pieceWriter.Cancel(ctx); cancelErr != nil {
			endpoint.log.Error("error during canceling a piece write", zap.Error(cancelErr))
		}
	}()

	availableBandwidth, err := endpoint.monitor.AvailableBandwidth(ctx)
	if err != nil {
		return ErrInternal.Wrap(err)
	}

	availableSpace, err := endpoint.monitor.AvailableSpace(ctx)
	if err != nil {
		return ErrInternal.Wrap(err)
	}

	orderSaved := false
	largestOrder := pb.Order{}
	// Ensure that the order is saved even in the face of an error. In the
	// success path, the order will be saved just before sending the response
	// and closing the stream (in which case, orderSaved will be true).
	defer func() {
		if !orderSaved {
			endpoint.saveOrder(ctx, limit, &largestOrder)
		}
	}()

	for {
		message, err = stream.Recv() // TODO: reuse messages to avoid allocations
		if err == io.EOF {
			return ErrProtocol.New("unexpected EOF")
		} else if err != nil {
			return ErrProtocol.Wrap(err) // TODO: report rpc status bad message
		}
		if message == nil {
			return ErrProtocol.New("expected a message") // TODO: report rpc status bad message
		}
		if message.Order == nil && message.Chunk == nil && message.Done == nil {
			return ErrProtocol.New("expected a message") // TODO: report rpc status bad message
		}

		if message.Order != nil {
			if err := endpoint.VerifyOrder(ctx, limit, message.Order, largestOrder.Amount); err != nil {
				return err
			}
			largestOrder = *message.Order
		}

		if message.Chunk != nil {
			if message.Chunk.Offset != pieceWriter.Size() {
				return ErrProtocol.New("chunk out of order") // TODO: report rpc status bad message
			}

			chunkSize := int64(len(message.Chunk.Data))
			if largestOrder.Amount < pieceWriter.Size()+chunkSize {
				// TODO: should we write currently and give a chance for uplink to remedy the situation?
				return ErrProtocol.New("not enough allocated, allocated=%v writing=%v", largestOrder.Amount, pieceWriter.Size()+int64(len(message.Chunk.Data))) // TODO: report rpc status ?
			}

			availableBandwidth -= chunkSize
			if availableBandwidth < 0 {
				return ErrProtocol.New("out of bandwidth")
			}
			availableSpace -= chunkSize
			if availableSpace < 0 {
				return ErrProtocol.New("out of space")
			}

			if _, err := pieceWriter.Write(message.Chunk.Data); err != nil {
				return ErrInternal.Wrap(err) // TODO: report rpc status internal server error
			}
		}

		if message.Done != nil {
			calculatedHash := pieceWriter.Hash()
			if err := endpoint.VerifyPieceHash(ctx, limit, message.Done, calculatedHash); err != nil {
				return err // TODO: report rpc status internal server error
			}
			if message.Done.PieceSize != pieceWriter.Size() {
				return ErrProtocol.New("Size of finished piece does not match size declared by uplink! %d != %d",
					message.Done.PieceSize, pieceWriter.Size())
			}

			{
				info := &pb.PieceHeader{
					Hash:         calculatedHash,
					CreationTime: message.Done.Timestamp,
					Signature:    message.Done.GetSignature(),
					OrderLimit:   *limit,
				}
				if err := pieceWriter.Commit(ctx, info); err != nil {
					return ErrInternal.Wrap(err) // TODO: report rpc status internal server error
				}
				if !limit.PieceExpiration.IsZero() {
					err := endpoint.store.SetExpiration(ctx, limit.SatelliteId, limit.PieceId, limit.PieceExpiration)
					if err != nil {
						return ErrInternal.Wrap(err) // TODO: report rpc status internal server error
					}
				}
			}

			storageNodeHash, err := signing.SignPieceHash(ctx, endpoint.signer, &pb.PieceHash{
				PieceId:   limit.PieceId,
				Hash:      calculatedHash,
				PieceSize: pieceWriter.Size(),
				Timestamp: time.Now(),
			})
			if err != nil {
				return ErrInternal.Wrap(err)
			}

			// Save the order before completing the call. Set orderSaved so
			// that the defer above does not also save.
			orderSaved = true
			endpoint.saveOrder(ctx, limit, &largestOrder)

			closeErr := stream.SendAndClose(&pb.PieceUploadResponse{
				Done: storageNodeHash,
			})
			return ErrProtocol.Wrap(ignoreEOF(closeErr))
		}
	}
}

// Download handles Downloading a piece on piece store.
func (endpoint *Endpoint) Download(stream pb.Piecestore_DownloadServer) (err error) {
	return endpoint.doDownload(stream)
}

// Download handles Downloading a piece on piece store.
func (endpoint *drpcEndpoint) Download(stream pb.DRPCPiecestore_DownloadStream) (err error) {
	return endpoint.doDownload(stream)
}

// downloadStream is the minimum interface required to perform settlements.
type downloadStream interface {
	Context() context.Context
	Recv() (*pb.PieceDownloadRequest, error)
	Send(*pb.PieceDownloadResponse) error
}

// Download implements downloading a piece from piece store.
func (endpoint *Endpoint) doDownload(stream downloadStream) (err error) {
	ctx := stream.Context()
	defer monLiveRequests(&ctx)(&err)
	defer mon.Task()(&ctx)(&err)

	atomic.AddInt32(&endpoint.liveRequests, 1)
	defer atomic.AddInt32(&endpoint.liveRequests, -1)

	startTime := time.Now().UTC()

	endpoint.pingStats.WasPinged(time.Now())

	// TODO: set connection timeouts
	// TODO: set maximum message size

	var message *pb.PieceDownloadRequest

	// receive limit and chunk from uplink
	message, err = stream.Recv()
	if err != nil {
		return ErrProtocol.Wrap(err)
	}
	if message.Limit == nil || message.Chunk == nil {
		return ErrProtocol.New("expected order limit and chunk as the first message")
	}
	limit, chunk := message.Limit, message.Chunk

	endpoint.log.Info("download started", zap.Stringer("Piece ID", limit.PieceId), zap.Stringer("Satellite ID", limit.SatelliteId), zap.Stringer("Action", limit.Action))

	if limit.Action != pb.PieceAction_GET && limit.Action != pb.PieceAction_GET_REPAIR && limit.Action != pb.PieceAction_GET_AUDIT {
		return ErrProtocol.New("expected get or get repair or audit action got %v", limit.Action) // TODO: report rpc status unauthorized or bad request
	}

	if chunk.ChunkSize > limit.Limit {
		return ErrProtocol.New("requested more that order limit allows, limit=%v requested=%v", limit.Limit, chunk.ChunkSize)
	}

	if err := endpoint.verifyOrderLimit(ctx, limit); err != nil {
		return Error.Wrap(err) // TODO: report rpc status unauthorized or bad request
	}

	var pieceReader *pieces.Reader
	defer func() {
		endTime := time.Now().UTC()
		dt := endTime.Sub(startTime)
		downloadSize := int64(0)
		if pieceReader != nil {
			downloadSize = pieceReader.Size()
		}
		downloadRate := float64(0)
		if dt.Seconds() > 0 {
			downloadRate = float64(downloadSize) / dt.Seconds()
		}
		downloadDuration := dt.Nanoseconds()
		if err != nil {
			mon.Meter("download_failure_byte_meter").Mark64(downloadSize)
			mon.IntVal("download_failure_size_bytes").Observe(downloadSize)
			mon.IntVal("download_failure_duration_ns").Observe(downloadDuration)
			mon.FloatVal("download_failure_rate_bytes_per_sec").Observe(downloadRate)
			endpoint.log.Info("download failed", zap.Stringer("Piece ID", limit.PieceId), zap.Stringer("Satellite ID", limit.SatelliteId), zap.Stringer("Action", limit.Action), zap.Error(err))
		} else {
			mon.Meter("download_success_byte_meter").Mark64(downloadSize)
			mon.IntVal("download_success_size_bytes").Observe(downloadSize)
			mon.IntVal("download_success_duration_ns").Observe(downloadDuration)
			mon.FloatVal("download_success_rate_bytes_per_sec").Observe(downloadRate)
			endpoint.log.Info("downloaded", zap.Stringer("Piece ID", limit.PieceId), zap.Stringer("Satellite ID", limit.SatelliteId), zap.Stringer("Action", limit.Action))
		}
	}()

	pieceReader, err = endpoint.store.Reader(ctx, limit.SatelliteId, limit.PieceId)
	if err != nil {
		if os.IsNotExist(err) {
			return rpcstatus.Error(rpcstatus.NotFound, err.Error())
		}
		return rpcstatus.Error(rpcstatus.Internal, err.Error())
	}
	defer func() {
		err := pieceReader.Close() // similarly how transcation Rollback works
		if err != nil {
			// no reason to report this error to the uplink
			endpoint.log.Error("failed to close piece reader", zap.Error(err))
		}
	}()

	// for repair traffic, send along the PieceHash and original OrderLimit for validation
	// before sending the piece itself
	if message.Limit.Action == pb.PieceAction_GET_REPAIR {
		pieceHash, orderLimit, err := endpoint.store.GetHashAndLimit(ctx, limit.SatelliteId, limit.PieceId, pieceReader)
		if err != nil {
			endpoint.log.Error("could not get hash and order limit", zap.Error(err))
			return rpcstatus.Error(rpcstatus.Internal, err.Error())
		}

		err = stream.Send(&pb.PieceDownloadResponse{Hash: &pieceHash, Limit: &orderLimit})
		if err != nil {
			endpoint.log.Error("error sending hash and order limit", zap.Error(err))
			return rpcstatus.Error(rpcstatus.Internal, err.Error())
		}
	}

	// TODO: verify chunk.Size behavior logic with regards to reading all
	if chunk.Offset+chunk.ChunkSize > pieceReader.Size() {
		return Error.New("requested more data than available, requesting=%v available=%v", chunk.Offset+chunk.ChunkSize, pieceReader.Size())
	}

	availableBandwidth, err := endpoint.monitor.AvailableBandwidth(ctx)
	if err != nil {
		endpoint.log.Error("error getting available bandwidth", zap.Error(err))
		return rpcstatus.Error(rpcstatus.Internal, err.Error())
	}

	throttle := sync2.NewThrottle()
	// TODO: see whether this can be implemented without a goroutine

	group, ctx := errgroup.WithContext(ctx)
	group.Go(func() (err error) {
		var maximumChunkSize = 1 * memory.MiB.Int64()

		currentOffset := chunk.Offset
		unsentAmount := chunk.ChunkSize
		for unsentAmount > 0 {
			tryToSend := min(unsentAmount, maximumChunkSize)

			// TODO: add timeout here
			chunkSize, err := throttle.ConsumeOrWait(tryToSend)
			if err != nil {
				// this can happen only because uplink decided to close the connection
				return nil
			}

			chunkData := make([]byte, chunkSize)
			_, err = pieceReader.Seek(currentOffset, io.SeekStart)
			if err != nil {
				endpoint.log.Error("error seeking on piecereader", zap.Error(err))
				return rpcstatus.Error(rpcstatus.Internal, err.Error())
			}

			// ReadFull is required to ensure we are sending the right amount of data.
			_, err = io.ReadFull(pieceReader, chunkData)
			if err != nil {
				endpoint.log.Error("error reading from piecereader", zap.Error(err))
				return rpcstatus.Error(rpcstatus.Internal, err.Error())
			}

			err = stream.Send(&pb.PieceDownloadResponse{
				Chunk: &pb.PieceDownloadResponse_Chunk{
					Offset: currentOffset,
					Data:   chunkData,
				},
			})
			if err != nil {
				// err is io.EOF when uplink asked for a piece, but decided not to retrieve it,
				// no need to propagate it
				return ErrProtocol.Wrap(ignoreEOF(err))
			}

			currentOffset += chunkSize
			unsentAmount -= chunkSize
		}
		return nil
	})

	recvErr := func() (err error) {
		largestOrder := pb.Order{}
		defer endpoint.saveOrder(ctx, limit, &largestOrder)

		// ensure that we always terminate sending goroutine
		defer throttle.Fail(io.EOF)

		for {
			// TODO: check errors
			// TODO: add timeout here
			message, err = stream.Recv()
			if err != nil {
				// err is io.EOF or canceled when uplink closed the connection, no need to return error
				if errs2.IsCanceled(err) {
					endpoint.log.Debug("client canceled connection")
					return nil
				}
				return ErrProtocol.Wrap(ignoreEOF(err))
			}

			if message == nil || message.Order == nil {
				return ErrProtocol.New("expected order as the message")
			}

			if err := endpoint.VerifyOrder(ctx, limit, message.Order, largestOrder.Amount); err != nil {
				return err
			}

			chunkSize := message.Order.Amount - largestOrder.Amount
			availableBandwidth -= chunkSize
			if availableBandwidth < 0 {
				return ErrProtocol.New("out of bandwidth")
			}

			if err := throttle.Produce(chunkSize); err != nil {
				// shouldn't happen since only receiving side is calling Fail
				return ErrInternal.Wrap(err)
			}
			largestOrder = *message.Order
		}
	}()

	// ensure we wait for sender to complete
	sendErr := group.Wait()
	return Error.Wrap(errs.Combine(sendErr, recvErr))
}

// saveOrder saves the order with all necessary information. It assumes it has been already verified.
func (endpoint *Endpoint) saveOrder(ctx context.Context, limit *pb.OrderLimit, order *pb.Order) {
	var err error
	defer mon.Task()(&ctx)(&err)

	// TODO: do this in a goroutine
	if order == nil || order.Amount <= 0 {
		return
	}
	err = endpoint.orders.Enqueue(ctx, &orders.Info{
		Limit: limit,
		Order: order,
	})
	if err != nil {
		endpoint.log.Error("failed to add order", zap.Error(err))
	} else {
		err = endpoint.usage.Add(ctx, limit.SatelliteId, limit.Action, order.Amount, time.Now())
		if err != nil {
			endpoint.log.Error("failed to add bandwidth usage", zap.Error(err))
		}
	}
}

// RestoreTrash restores all trashed items for the satellite issuing the call
func (endpoint *Endpoint) RestoreTrash(ctx context.Context, restoreTrashReq *pb.RestoreTrashRequest) (res *pb.RestoreTrashResponse, err error) {
	defer mon.Task()(&ctx)(&err)

	peer, err := identity.PeerIdentityFromContext(ctx)
	if err != nil {
		return nil, rpcstatus.Error(rpcstatus.Unauthenticated, Error.Wrap(err).Error())
	}

	err = endpoint.trust.VerifySatelliteID(ctx, peer.ID)
	if err != nil {
		return nil, rpcstatus.Error(rpcstatus.PermissionDenied, Error.New("RestoreTrash called with untrusted ID").Error())
	}

	err = endpoint.store.RestoreTrash(ctx, peer.ID)
	if err != nil {
		return nil, ErrInternal.Wrap(err)
	}

	return &pb.RestoreTrashResponse{}, nil
}

// Retain keeps only piece ids specified in the request
func (endpoint *Endpoint) Retain(ctx context.Context, retainReq *pb.RetainRequest) (res *pb.RetainResponse, err error) {
	defer mon.Task()(&ctx)(&err)

	// if retain status is disabled, quit immediately
	if endpoint.retain.Status() == retain.Disabled {
		return &pb.RetainResponse{}, nil
	}

	peer, err := identity.PeerIdentityFromContext(ctx)
	if err != nil {
		return nil, rpcstatus.Error(rpcstatus.Unauthenticated, Error.Wrap(err).Error())
	}

	err = endpoint.trust.VerifySatelliteID(ctx, peer.ID)
	if err != nil {
		return nil, rpcstatus.Error(rpcstatus.PermissionDenied, Error.New("retain called with untrusted ID").Error())
	}

	filter, err := bloomfilter.NewFromBytes(retainReq.GetFilter())
	if err != nil {
		return nil, rpcstatus.Error(rpcstatus.InvalidArgument, Error.Wrap(err).Error())
	}

	// the queue function will update the created before time based on the configurable retain buffer
	queued := endpoint.retain.Queue(retain.Request{
		SatelliteID:   peer.ID,
		CreatedBefore: retainReq.GetCreationDate(),
		Filter:        filter,
	})
	if !queued {
		endpoint.log.Debug("Retain job not queued for satellite", zap.Stringer("Satellite ID", peer.ID))
	}

	return &pb.RetainResponse{}, nil
}

// TestLiveRequestCount returns the current number of live requests.
func (endpoint *Endpoint) TestLiveRequestCount() int32 {
	return atomic.LoadInt32(&endpoint.liveRequests)
}

// min finds the min of two values
func min(a, b int64) int64 {
	if a < b {
		return a
	}
	return b
}

// ignoreEOF ignores io.EOF error.
func ignoreEOF(err error) error {
	// gRPC gives us an io.EOF but dRPC gives us a wrapped io.EOF
	if errs.Is(err, io.EOF) {
		return nil
	}
	return err
}
