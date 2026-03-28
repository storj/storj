// Copyright (C) 2026 Storj Labs, Inc.
// See LICENSE for copying information.

package nodeaudit

import (
	"bytes"
	"context"
	"errors"
	"io"
	"time"

	"github.com/spacemonkeygo/monkit/v3"
	"github.com/zeebo/errs"
	"go.uber.org/zap"

	"storj.io/common/errs2"
	"storj.io/common/pb"
	"storj.io/common/rpc"
	"storj.io/common/rpc/rpcpool"
	"storj.io/common/rpc/rpcstatus"
	"storj.io/common/signing"
	"storj.io/common/storj"
	"storj.io/storj/satellite/metabase"
	"storj.io/storj/satellite/orders"
	"storj.io/storj/satellite/taskqueue"
)

var (
	mon = monkit.Package()

	// Error is the error class for nodeaudit service.
	Error = errs.Class("nodeaudit")
)

// PieceAuditConfig contains configuration for the nodeaudit service.
type PieceAuditConfig struct {
	TargetNodeURL   storj.NodeURL `help:"node URL (id@host:port) to check pieces on"`
	DialTimeout     time.Duration `help:"timeout for connecting to storage node" default:"5s"`
	DownloadTimeout time.Duration `help:"timeout for piece download" default:"30s"`
}

// PieceAudit checks segment downloadability from a specific storage node.
type PieceAudit struct {
	log    *zap.Logger
	config PieceAuditConfig

	runner   *taskqueue.Runner[Job]
	metabase *metabase.DB
	orders   *orders.Service
	dialer   rpc.Dialer
	pool     *rpcpool.Pool
	signer   signing.Signer
}

// NewChecker creates a new nodeaudit service.
func NewChecker(
	log *zap.Logger,
	config PieceAuditConfig,
	runnerConfig taskqueue.RunnerConfig,
	client *taskqueue.Client,
	metabaseDB *metabase.DB,
	ordersService *orders.Service,
	dialer rpc.Dialer,
	signer signing.Signer,
) *PieceAudit {
	pool := rpcpool.New(rpcpool.Options{
		Capacity:       runnerConfig.WorkerCount,
		KeyCapacity:    runnerConfig.WorkerCount,
		IdleExpiration: 2 * time.Minute,
		Name:           "nodeaudit",
	})
	dialer.Pool = pool

	service := &PieceAudit{
		log:      log,
		config:   config,
		metabase: metabaseDB,
		orders:   ordersService,
		dialer:   dialer,
		pool:     pool,
		signer:   signer,
	}

	service.runner = taskqueue.NewRunner[Job](
		log,
		runnerConfig,
		client,
		streamID,
		service,
	)

	return service
}

// Process implements taskqueue.Processor[Job].
func (service *PieceAudit) Process(ctx context.Context, job Job) {
	service.processJob(ctx, job)
}

// Run starts the nodeaudit service loop.
func (service *PieceAudit) Run(ctx context.Context) (err error) {
	defer mon.Task()(&ctx)(&err)

	return service.runner.Run(ctx)
}

// Close closes the service.
func (service *PieceAudit) Close() error {
	var errlist errs.Group
	errlist.Add(service.runner.Close())
	if service.pool != nil {
		errlist.Add(service.pool.Close())
	}
	return errlist.Err()
}

func (service *PieceAudit) processJob(ctx context.Context, job Job) {
	defer mon.Task()(&ctx)(nil)

	log := service.log.With(
		zap.Stringer("stream_id", job.StreamID),
		zap.Uint64("position", job.Position),
	)

	log = log.With(zap.Uint16("piece_num", job.PieceNo))

	// Create order limit for piece download
	limit, piecePrivateKey, err := service.orders.CreateAuditPieceOrderLimitForNode(ctx, service.config.TargetNodeURL, job.PieceNo, job.RootPieceID, int32(1))
	if err != nil {
		log.Error("failed to create order limit", zap.Error(err))
		return
	}

	// Download the piece using the configured node URL
	err = service.downloadPiece(ctx, limit, piecePrivateKey, service.config.TargetNodeURL.Address, 1)
	if err != nil {
		if !errs2.IsRPC(err, rpcstatus.NotFound) {
			log.Info("piece download failed", zap.Error(err))
			mon.Counter("nodeaudit_failed").Inc(1)
			return
		}

		// Piece not found on the node; check if the segment still exists
		_, merr := service.metabase.GetSegmentByPosition(ctx, metabase.GetSegmentByPosition{
			StreamID: job.StreamID,
			Position: metabase.SegmentPositionFromEncoded(job.Position),
		})
		if merr != nil {
			if metabase.ErrSegmentNotFound.Has(merr) {
				log.Debug("segment no longer exists")
				return
			}
			log.Error("failed to get segment", zap.Error(merr))
			return
		}

		log.Info("piece not found on node", zap.Error(err))
		mon.Counter("nodeaudit_failed").Inc(1)
		return
	}

	log.Debug("piece download successful")
	mon.Counter("nodeaudit_success").Inc(1)
}

func (service *PieceAudit) downloadPiece(ctx context.Context, limit *pb.AddressedOrderLimit, piecePrivateKey storj.PiecePrivateKey, nodeAddress string, pieceSize int64) (err error) {
	defer mon.Task()(&ctx)(&err)

	// Set up timeout for dial
	dialCtx, dialCancel := context.WithTimeout(ctx, service.config.DialTimeout)
	defer dialCancel()

	// Connect to storage node
	nodeURL := storj.NodeURL{
		ID:      limit.Limit.StorageNodeId,
		Address: nodeAddress,
	}

	conn, err := service.dialer.DialNodeURL(dialCtx, nodeURL)
	if err != nil {
		return Error.Wrap(err)
	}
	defer func() { err = errs.Combine(err, conn.Close()) }()

	// Set up timeout for download
	downloadCtx, downloadCancel := context.WithTimeout(ctx, service.config.DownloadTimeout)
	defer downloadCancel()

	// Create piecestore client
	client := pb.NewDRPCReplaySafePiecestoreClient(conn)

	stream, err := client.Download(downloadCtx)
	if err != nil {
		return Error.Wrap(err)
	}
	defer func() { err = errs.Combine(err, stream.Close()) }()

	// Send download request
	order := &pb.Order{
		SerialNumber: limit.Limit.SerialNumber,
		Amount:       pieceSize,
	}
	order, err = signing.SignUplinkOrder(ctx, piecePrivateKey, order)
	if err != nil {
		return Error.Wrap(err)
	}

	err = stream.Send(&pb.PieceDownloadRequest{
		Limit: limit.Limit,
		Chunk: &pb.PieceDownloadRequest_Chunk{
			Offset:    0,
			ChunkSize: pieceSize,
		},
		Order: order,
	})
	if err != nil {
		return Error.Wrap(err)
	}

	var downloaded int64
	buf := bytes.NewBuffer(make([]byte, 0, pieceSize))

	for downloaded < pieceSize {
		resp, err := stream.Recv()
		if err != nil {
			if errors.Is(err, io.EOF) {
				break
			}
			return Error.Wrap(err)
		}

		if resp.Chunk != nil && len(resp.Chunk.Data) > 0 {
			buf.Write(resp.Chunk.Data)
			downloaded += int64(len(resp.Chunk.Data))
		}
	}

	if downloaded != pieceSize {
		return Error.New("incomplete download: got %d bytes, expected %d", downloaded, pieceSize)
	}

	return nil
}
