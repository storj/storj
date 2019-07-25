// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package piecestore

import (
	"context"
	"io"
	"os"
	"runtime"
	"sync/atomic"
	"time"

	"github.com/zeebo/errs"
	"go.uber.org/zap"
	"golang.org/x/sync/errgroup"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"gopkg.in/spacemonkeygo/monkit.v2"

	"storj.io/storj/internal/memory"
	"storj.io/storj/internal/sync2"
	"storj.io/storj/pkg/auth/signing"
	"storj.io/storj/pkg/bloomfilter"
	"storj.io/storj/pkg/identity"
	"storj.io/storj/pkg/pb"
	"storj.io/storj/pkg/storj"
	"storj.io/storj/storagenode/bandwidth"
	"storj.io/storj/storagenode/monitor"
	"storj.io/storj/storagenode/orders"
	"storj.io/storj/storagenode/pieces"
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
	WhitelistedSatellites  storj.NodeURLs `help:"a comma-separated list of approved satellite node urls" devDefault:"" releaseDefault:"12EayRS2V1kEsWESU9QMRseFhdxYxKicsiFmxrsLZHeLUtdps3S@mars.tardigrade.io:7777,118UWpMCHzs6CvSgWd9BfFVjw5K9pZbJjkfZJexMtSkmKxvvAW@satellite.stefan-benten.de:7777,121RTSDpyNZVcEU84Ticf2L1ntiuUimbWgfATz21tuvgk3vzoA6@saturn.tardigrade.io:7777,12L9ZFwhzVpuEKMUNUqkaTLGzwY9G24tbiigLiXpmZWKwmcNDDs@jupiter.tardigrade.io:7777"`
	AllocatedDiskSpace     memory.Size    `user:"true" help:"total allocated disk space in bytes" default:"1TB"`
	AllocatedBandwidth     memory.Size    `user:"true" help:"total allocated bandwidth in bytes" default:"2TB"`
	KBucketRefreshInterval time.Duration  `help:"how frequently Kademlia bucket should be refreshed with node stats" default:"1h0m0s"`
}

// Config defines parameters for piecestore endpoint.
type Config struct {
	ExpirationGracePeriod time.Duration `help:"how soon before expiration date should things be considered expired" default:"48h0m0s"`
	MaxConcurrentRequests int           `help:"how many concurrent requests are allowed, before uploads are rejected." default:"6"`
	OrderLimitGracePeriod time.Duration `help:"how long after OrderLimit creation date are OrderLimits no longer accepted" default:"1h0m0s"`
	RetainTimeBuffer      time.Duration `help:"allows for small differences in the satellite and storagenode clocks" default:"1h0m0s"`

	Monitor monitor.Config
	Sender  orders.SenderConfig
}

// Endpoint implements uploading, downloading and deleting for a storage node.
type Endpoint struct {
	log    *zap.Logger
	config Config

	signer  signing.Signer
	trust   *trust.Pool
	monitor *monitor.Service

	store       *pieces.Store
	pieceinfo   pieces.DB
	orders      orders.DB
	usage       bandwidth.DB
	usedSerials UsedSerials

	liveRequests int32
}

// NewEndpoint creates a new piecestore endpoint.
func NewEndpoint(log *zap.Logger, signer signing.Signer, trust *trust.Pool, monitor *monitor.Service, store *pieces.Store, pieceinfo pieces.DB, orders orders.DB, usage bandwidth.DB, usedSerials UsedSerials, config Config) (*Endpoint, error) {
	return &Endpoint{
		log:    log,
		config: config,

		signer:  signer,
		trust:   trust,
		monitor: monitor,

		store:       store,
		pieceinfo:   pieceinfo,
		orders:      orders,
		usage:       usage,
		usedSerials: usedSerials,

		liveRequests: 0,
	}, nil
}

// Delete handles deleting a piece on piece store.
func (endpoint *Endpoint) Delete(ctx context.Context, delete *pb.PieceDeleteRequest) (_ *pb.PieceDeleteResponse, err error) {
	defer mon.Task()(&ctx)(&err)

	atomic.AddInt32(&endpoint.liveRequests, 1)
	defer atomic.AddInt32(&endpoint.liveRequests, -1)

	if delete.Limit.Action != pb.PieceAction_DELETE {
		return nil, Error.New("expected delete action got %v", delete.Limit.Action) // TODO: report grpc status unauthorized or bad request
	}

	if err := endpoint.VerifyOrderLimit(ctx, delete.Limit); err != nil {
		// TODO: report grpc status unauthorized or bad request
		return nil, Error.Wrap(err)
	}

	// TODO: parallelize this and maybe return early
	pieceInfoErr := endpoint.pieceinfo.Delete(ctx, delete.Limit.SatelliteId, delete.Limit.PieceId)
	pieceErr := endpoint.store.Delete(ctx, delete.Limit.SatelliteId, delete.Limit.PieceId)

	if err := errs.Combine(pieceInfoErr, pieceErr); err != nil {
		// explicitly ignoring error because the errors
		// TODO: add more debug info
		endpoint.log.Error("delete failed", zap.Stringer("Piece ID", delete.Limit.PieceId), zap.Error(err))
		// TODO: report internal server internal or missing error using grpc status,
		// e.g. missing might happen when we get a deletion request after garbage collection has deleted it
	} else {
		endpoint.log.Info("deleted", zap.Stringer("Piece ID", delete.Limit.PieceId))
	}

	return &pb.PieceDeleteResponse{}, nil
}

// Upload handles uploading a piece on piece store.
func (endpoint *Endpoint) Upload(stream pb.Piecestore_UploadServer) (err error) {
	ctx := stream.Context()
	defer mon.Task()(&ctx)(&err)

	liveRequests := atomic.AddInt32(&endpoint.liveRequests, 1)
	defer atomic.AddInt32(&endpoint.liveRequests, -1)

	if int(liveRequests) > endpoint.config.MaxConcurrentRequests {
		endpoint.log.Error("upload rejected, too many requests", zap.Int32("live requests", liveRequests))
		return status.Error(codes.Unavailable, "storage node overloaded")
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
	endpoint.log.Info("upload started", zap.Stringer("Piece ID", limit.PieceId), zap.Stringer("SatelliteID", limit.SatelliteId), zap.Stringer("Action", limit.Action))

	// TODO: verify that we have have expected amount of storage before continuing

	if limit.Action != pb.PieceAction_PUT && limit.Action != pb.PieceAction_PUT_REPAIR {
		return ErrProtocol.New("expected put or put repair action got %v", limit.Action) // TODO: report grpc status unauthorized or bad request
	}

	if err := endpoint.VerifyOrderLimit(ctx, limit); err != nil {
		return err // TODO: report grpc status unauthorized or bad request
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
			endpoint.log.Info("upload failed", zap.Stringer("Piece ID", limit.PieceId), zap.Stringer("SatelliteID", limit.SatelliteId), zap.Stringer("Action", limit.Action), zap.Error(err))
		} else {
			mon.Meter("upload_success_byte_meter").Mark64(uploadSize)
			mon.IntVal("upload_success_size_bytes").Observe(uploadSize)
			mon.IntVal("upload_success_duration_ns").Observe(uploadDuration)
			mon.FloatVal("upload_success_rate_bytes_per_sec").Observe(uploadRate)
			endpoint.log.Info("uploaded", zap.Stringer("Piece ID", limit.PieceId), zap.Stringer("SatelliteID", limit.SatelliteId), zap.Stringer("Action", limit.Action))
		}
	}()

	pieceWriter, err = endpoint.store.Writer(ctx, limit.SatelliteId, limit.PieceId)
	if err != nil {
		return ErrInternal.Wrap(err) // TODO: report grpc status internal server error
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

	largestOrder := pb.Order{}
	defer endpoint.SaveOrder(ctx, limit, &largestOrder)

	for {
		message, err = stream.Recv() // TODO: reuse messages to avoid allocations
		if err == io.EOF {
			return ErrProtocol.New("unexpected EOF")
		} else if err != nil {
			return ErrProtocol.Wrap(err) // TODO: report grpc status bad message
		}
		if message == nil {
			return ErrProtocol.New("expected a message") // TODO: report grpc status bad message
		}
		if message.Order == nil && message.Chunk == nil && message.Done == nil {
			return ErrProtocol.New("expected a message") // TODO: report grpc status bad message
		}

		if message.Order != nil {
			if err := endpoint.VerifyOrder(ctx, limit, message.Order, largestOrder.Amount); err != nil {
				return err
			}
			largestOrder = *message.Order
		}

		if message.Chunk != nil {
			if message.Chunk.Offset != pieceWriter.Size() {
				return ErrProtocol.New("chunk out of order") // TODO: report grpc status bad message
			}

			chunkSize := int64(len(message.Chunk.Data))
			if largestOrder.Amount < pieceWriter.Size()+chunkSize {
				// TODO: should we write currently and give a chance for uplink to remedy the situation?
				return ErrProtocol.New("not enough allocated, allocated=%v writing=%v", largestOrder.Amount, pieceWriter.Size()+int64(len(message.Chunk.Data))) // TODO: report grpc status ?
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
				return ErrInternal.Wrap(err) // TODO: report grpc status internal server error
			}
		}

		if message.Done != nil {
			expectedHash := pieceWriter.Hash()
			if err := endpoint.VerifyPieceHash(ctx, limit, message.Done, expectedHash); err != nil {
				return err // TODO: report grpc status internal server error
			}
			if message.Done.PieceSize != pieceWriter.Size() {
				return ErrProtocol.New("Size of finished piece does not match size declared by uplink! %d != %d",
					message.Done.GetPieceSize(), pieceWriter.Size())
			}

			if err := pieceWriter.Commit(ctx); err != nil {
				return ErrInternal.Wrap(err) // TODO: report grpc status internal server error
			}

			// TODO: do this in a goroutine
			{
				// TODO: maybe this should be as a pieceWriter.Commit(ctx, info)
				info := &pieces.Info{
					SatelliteID: limit.SatelliteId,

					PieceID:         limit.PieceId,
					PieceSize:       pieceWriter.Size(),
					PieceCreation:   limit.OrderCreation,
					PieceExpiration: limit.PieceExpiration,

					OrderLimit:      limit,
					UplinkPieceHash: message.Done,
				}

				if err := endpoint.pieceinfo.Add(ctx, info); err != nil {
					ignoreCancelContext := context.Background()
					deleteErr := endpoint.store.Delete(ignoreCancelContext, limit.SatelliteId, limit.PieceId)
					return ErrInternal.Wrap(errs.Combine(err, deleteErr))
				}
			}

			storageNodeHash, err := signing.SignPieceHash(ctx, endpoint.signer, &pb.PieceHash{
				PieceId:   limit.PieceId,
				Hash:      expectedHash,
				PieceSize: pieceWriter.Size(),
				Timestamp: time.Now(),
			})
			if err != nil {
				return ErrInternal.Wrap(err)
			}

			closeErr := stream.SendAndClose(&pb.PieceUploadResponse{
				Done: storageNodeHash,
			})
			return ErrProtocol.Wrap(ignoreEOF(closeErr))
		}
	}
}

// Download implements downloading a piece from piece store.
func (endpoint *Endpoint) Download(stream pb.Piecestore_DownloadServer) (err error) {
	ctx := stream.Context()
	defer mon.Task()(&ctx)(&err)

	atomic.AddInt32(&endpoint.liveRequests, 1)
	defer atomic.AddInt32(&endpoint.liveRequests, -1)

	startTime := time.Now().UTC()

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

	endpoint.log.Info("download started", zap.Stringer("Piece ID", limit.PieceId), zap.Stringer("SatelliteID", limit.SatelliteId), zap.Stringer("Action", limit.Action))

	if limit.Action != pb.PieceAction_GET && limit.Action != pb.PieceAction_GET_REPAIR && limit.Action != pb.PieceAction_GET_AUDIT {
		return ErrProtocol.New("expected get or get repair or audit action got %v", limit.Action) // TODO: report grpc status unauthorized or bad request
	}

	if chunk.ChunkSize > limit.Limit {
		return ErrProtocol.New("requested more that order limit allows, limit=%v requested=%v", limit.Limit, chunk.ChunkSize)
	}

	if err := endpoint.VerifyOrderLimit(ctx, limit); err != nil {
		return Error.Wrap(err) // TODO: report grpc status unauthorized or bad request
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
			endpoint.log.Info("download failed", zap.Stringer("Piece ID", limit.PieceId), zap.Stringer("SatelliteID", limit.SatelliteId), zap.Stringer("Action", limit.Action), zap.Error(err))
		} else {
			mon.Meter("download_success_byte_meter").Mark64(downloadSize)
			mon.IntVal("download_success_size_bytes").Observe(downloadSize)
			mon.IntVal("download_success_duration_ns").Observe(downloadDuration)
			mon.FloatVal("download_success_rate_bytes_per_sec").Observe(downloadRate)
			endpoint.log.Info("downloaded", zap.Stringer("Piece ID", limit.PieceId), zap.Stringer("SatelliteID", limit.SatelliteId), zap.Stringer("Action", limit.Action))
		}
	}()

	pieceReader, err = endpoint.store.Reader(ctx, limit.SatelliteId, limit.PieceId)
	if err != nil {
		if os.IsNotExist(err) {
			return status.Error(codes.NotFound, err.Error())
		}
		return status.Error(codes.Internal, err.Error())
	}
	defer func() {
		err := pieceReader.Close() // similarly how transcation Rollback works
		if err != nil {
			// no reason to report this error to the uplink
			endpoint.log.Error("failed to close piece reader", zap.Error(err))
		}
	}()

	// TODO: verify chunk.Size behavior logic with regards to reading all
	if chunk.Offset+chunk.ChunkSize > pieceReader.Size() {
		return Error.New("requested more data than available, requesting=%v available=%v", chunk.Offset+chunk.ChunkSize, pieceReader.Size())
	}

	availableBandwidth, err := endpoint.monitor.AvailableBandwidth(ctx)
	if err != nil {
		return ErrInternal.Wrap(err)
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
				return ErrInternal.Wrap(err)
			}

			// ReadFull is required to ensure we are sending the right amount of data.
			_, err = io.ReadFull(pieceReader, chunkData)
			if err != nil {
				return ErrInternal.Wrap(err)
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
		defer endpoint.SaveOrder(ctx, limit, &largestOrder)

		// ensure that we always terminate sending goroutine
		defer throttle.Fail(io.EOF)

		for {
			// TODO: check errors
			// TODO: add timeout here
			message, err = stream.Recv()
			if err != nil {
				// err is io.EOF when uplink closed the connection, no need to return error
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

// SaveOrder saves the order with all necessary information. It assumes it has been already verified.
func (endpoint *Endpoint) SaveOrder(ctx context.Context, limit *pb.OrderLimit, order *pb.Order) {
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

// Retain keeps only piece ids specified in the request
func (endpoint *Endpoint) Retain(ctx context.Context, retainReq *pb.RetainRequest) (res *pb.RetainResponse, err error) {
	defer mon.Task()(&ctx)(&err)

	peer, err := identity.PeerIdentityFromContext(ctx)
	if err != nil {
		return nil, status.Error(codes.Unauthenticated, Error.Wrap(err).Error())
	}

	err = endpoint.trust.VerifySatelliteID(ctx, peer.ID)
	if err != nil {
		return nil, status.Error(codes.PermissionDenied, Error.New("retain called with untrusted ID").Error())
	}

	filter, err := bloomfilter.NewFromBytes(retainReq.GetFilter())
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, Error.Wrap(err).Error())
	}

	const limit = 1000
	offset := 0
	numDeleted := 0
	hasMorePieces := true

	for hasMorePieces {
		// subtract some time to leave room for clock difference between the satellite and storage node
		createdBefore := retainReq.GetCreationDate().Add(-endpoint.config.RetainTimeBuffer)

		pieceIDs, err := endpoint.pieceinfo.GetPieceIDs(ctx, peer.ID, createdBefore, limit, offset)
		if err != nil {
			return nil, status.Error(codes.Internal, Error.Wrap(err).Error())
		}
		for _, pieceID := range pieceIDs {
			if !filter.Contains(pieceID) {
				if err = endpoint.store.Delete(ctx, peer.ID, pieceID); err != nil {
					endpoint.log.Error("failed to delete a piece", zap.Error(Error.Wrap(err)))
					// continue because if we fail to delete from file system,
					// we need to keep the pieceinfo so we can delete next time
					continue
				}
				if err = endpoint.pieceinfo.Delete(ctx, peer.ID, pieceID); err != nil {
					endpoint.log.Error("failed to delete piece info", zap.Error(Error.Wrap(err)))
				}
				numDeleted++
			}
		}
		hasMorePieces = (len(pieceIDs) == limit)
		offset += len(pieceIDs)
		offset -= numDeleted
		// We call Gosched() here because the GC process is expected to be long and we want to keep it at low priority,
		// so other goroutines can continue serving requests.
		runtime.Gosched()
	}
	return &pb.RetainResponse{}, nil
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
	if err == io.EOF {
		return nil
	}
	return err
}
