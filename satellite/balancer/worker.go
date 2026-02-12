// Copyright (C) 2026 Storj Labs, Inc.
// See LICENSE for copying information.

package balancer

import (
	"bytes"
	"context"
	"hash"
	"io"
	"time"

	"github.com/zeebo/errs"
	"go.uber.org/zap"

	"storj.io/common/pb"
	"storj.io/common/rpc"
	"storj.io/common/rpc/rpcpool"
	"storj.io/common/storj"
	"storj.io/storj/satellite/metabase"
	"storj.io/storj/satellite/nodeselection"
	"storj.io/storj/satellite/orders"
	"storj.io/storj/satellite/overlay"
	"storj.io/storj/satellite/repair"
	"storj.io/storj/satellite/taskqueue"
	"storj.io/uplink/private/piecestore"
)

// WorkerConfig holds the configuration for the balancer worker.
type WorkerConfig struct {
	StreamID            string        `help:"Redis stream name for balancer jobs" default:"balancer"`
	DialTimeout         time.Duration `help:"timeout for dialing storage nodes" default:"5s"`
	DownloadTimeout     time.Duration `help:"timeout for downloading a piece" default:"5m"`
	UploadTimeout       time.Duration `help:"timeout for uploading a piece" default:"5m"`
	DeleteAfterTransfer bool          `help:"delete the source piece from the source node after a successful transfer" default:"false"`
}

// Worker consumes balancer jobs from the task queue and transfers pieces between nodes.
type Worker struct {
	log    *zap.Logger
	config WorkerConfig

	metabase   *metabase.DB
	orders     *orders.Service
	overlay    *overlay.Service
	dialer     rpc.Dialer
	placements nodeselection.PlacementDefinitions

	runner *taskqueue.Runner[Job]
}

// NewWorker creates a new balancer worker.
func NewWorker(
	log *zap.Logger,
	config WorkerConfig,
	runnerConfig taskqueue.RunnerConfig,
	client *taskqueue.Client,
	metabase *metabase.DB,
	orders *orders.Service,
	overlay *overlay.Service,
	dialer rpc.Dialer,
	placements nodeselection.PlacementDefinitions,
) *Worker {
	w := &Worker{
		log:        log,
		config:     config,
		metabase:   metabase,
		orders:     orders,
		overlay:    overlay,
		dialer:     dialer,
		placements: placements,
	}
	w.runner = taskqueue.NewRunner[Job](log, runnerConfig, client, config.StreamID, w)
	return w
}

// Run starts the worker loop.
func (w *Worker) Run(ctx context.Context) error {
	return w.runner.Run(ctx)
}

// Close stops the worker.
func (w *Worker) Close() error {
	return w.runner.Close()
}

// Process handles a single balancer job.
func (w *Worker) Process(ctx context.Context, job Job) {
	err := w.processJob(ctx, job)
	if err != nil {
		w.log.Error("failed to process balancer job",
			zap.Stringer("stream_id", job.StreamID),
			zap.Uint64("position", job.Position),
			zap.Stringer("source_node", job.SourceNode),
			zap.Stringer("dest_node", job.DestNode),
			zap.Error(err),
		)
		mon.Counter("balancer_worker_failed").Inc(1)
		return
	}
	mon.Counter("balancer_worker_success").Inc(1)
}

// TestingProcessJob exposes processJob for testing.
func (w *Worker) TestingProcessJob(ctx context.Context, job Job) error {
	return w.processJob(ctx, job)
}

func (w *Worker) processJob(ctx context.Context, job Job) (err error) {
	defer mon.Task()(&ctx)(&err)

	// 1. Get segment from metabase.
	segment, err := w.metabase.GetSegmentByPosition(ctx, metabase.GetSegmentByPosition{
		StreamID: job.StreamID,
		Position: metabase.SegmentPositionFromEncoded(job.Position),
	})
	if err != nil {
		if metabase.ErrSegmentNotFound.Has(err) {
			w.log.Debug("segment no longer exists, skipping",
				zap.Stringer("stream_id", job.StreamID),
				zap.Uint64("position", job.Position))
			return nil
		}
		return Error.Wrap(err)
	}

	// 2. Find source node piece.
	pieceNum := -1
	for _, piece := range segment.Pieces {
		if piece.StorageNode == job.SourceNode {
			pieceNum = int(piece.Number)
			break
		}
	}
	if pieceNum < 0 {
		w.log.Debug("source node not in segment pieces, skipping",
			zap.Stringer("stream_id", job.StreamID),
			zap.Uint64("position", job.Position),
			zap.Stringer("source_node", job.SourceNode))
		return nil
	}

	// 3. Check if destination node is already in the segment.
	for _, piece := range segment.Pieces {
		if piece.StorageNode == job.DestNode {
			w.log.Debug("destination node already in segment, skipping",
				zap.Stringer("stream_id", job.StreamID),
				zap.Uint64("position", job.Position),
				zap.Stringer("dest_node", job.DestNode))
			return nil
		}
	}

	// 4. Classify segment health.
	nodeIDs := make(storj.NodeIDList, len(segment.Pieces))
	for i, piece := range segment.Pieces {
		nodeIDs[i] = piece.StorageNode
	}

	selectedNodes, err := w.overlay.GetParticipatingNodes(ctx, nodeIDs)
	if err != nil {
		return Error.Wrap(err)
	}

	nodeMap := make(map[storj.NodeID]nodeselection.SelectedNode, len(selectedNodes))
	for _, node := range selectedNodes {
		nodeMap[node.ID] = node
	}

	orderedNodes := make([]nodeselection.SelectedNode, len(segment.Pieces))
	for i, piece := range segment.Pieces {
		if node, ok := nodeMap[piece.StorageNode]; ok {
			orderedNodes[i] = node
		}
	}

	placement, ok := w.placements[segment.Placement]
	if !ok {
		return Error.New("unknown placement %d", segment.Placement)
	}

	piecesCheck := repair.ClassifySegmentPieces(
		segment.Pieces,
		orderedNodes,
		nil,   // excludedCountryCodes
		false, // doPlacementCheck
		false, // doDeclumping
		placement,
	)

	healthyCount := piecesCheck.Healthy.Count()
	if healthyCount < int(segment.Redundancy.RequiredShares) {
		w.log.Debug("not enough healthy pieces, skipping",
			zap.Stringer("stream_id", job.StreamID),
			zap.Uint64("position", job.Position),
			zap.Int("healthy", healthyCount),
			zap.Int16("required", segment.Redundancy.RequiredShares))
		return nil
	}

	// 5. Resolve source and destination node addresses.
	sourceNode, ok := nodeMap[job.SourceNode]
	if !ok || sourceNode.Address == nil {
		return Error.New("source node %s not found or has no address", job.SourceNode)
	}

	// Load upload-eligible nodes from cache to validate destination.
	uploadEligible, err := w.overlay.UploadSelectionCache.GetAllNodes(ctx)
	if err != nil {
		return Error.Wrap(err)
	}

	var destNode nodeselection.SelectedNode
	found := false
	for _, node := range uploadEligible {
		if node.ID == job.DestNode {
			destNode = *node
			found = true
			break
		}
	}
	if !found || destNode.Address == nil {
		w.log.Debug("destination node is not eligible for upload, skipping",
			zap.Stringer("stream_id", job.StreamID),
			zap.Uint64("position", job.Position),
			zap.Stringer("dest_node", job.DestNode))
		return nil
	}

	pieceSize := segment.PieceSize()

	// 6. Download piece from source node.
	pieceData, downloadHash, err := w.downloadPiece(ctx, segment, sourceNode, uint16(pieceNum), pieceSize)
	if err != nil {
		return Error.New("download from source %s failed: %w", job.SourceNode, err)
	}

	// 7. Upload piece to destination node, using the same hash algorithm as the source.
	var hashAlgo pb.PieceHashAlgorithm
	if downloadHash != nil {
		hashAlgo = downloadHash.HashAlgorithm
	}
	uploadHash, err := w.uploadPiece(ctx, segment, destNode, uint16(pieceNum), pieceSize, pieceData, hashAlgo)
	if err != nil {
		return Error.New("upload to destination %s failed: %w", job.DestNode, err)
	}

	// 8. Verify that download and upload hashes match.
	if downloadHash != nil && uploadHash != nil && !bytes.Equal(downloadHash.Hash, uploadHash.Hash) {
		return Error.New("piece hash mismatch: download %x != upload %x", downloadHash.Hash, uploadHash.Hash)
	}

	// 9. Update segment pieces atomically (CAS).
	newPieces, err := segment.Pieces.Update(
		metabase.Pieces{{Number: uint16(pieceNum), StorageNode: job.DestNode}},
		metabase.Pieces{{Number: uint16(pieceNum), StorageNode: job.SourceNode}},
	)
	if err != nil {
		return Error.Wrap(err)
	}

	err = w.metabase.UpdateSegmentPieces(ctx, metabase.UpdateSegmentPieces{
		StreamID:      segment.StreamID,
		Position:      segment.Position,
		OldPieces:     segment.Pieces,
		NewRedundancy: segment.Redundancy,
		NewPieces:     newPieces,
	})
	if err != nil {
		if metabase.ErrValueChanged.Has(err) {
			w.log.Debug("segment pieces changed concurrently, skipping",
				zap.Stringer("stream_id", job.StreamID),
				zap.Uint64("position", job.Position))
			return nil
		}
		return Error.Wrap(err)
	}

	// 10. Optionally delete the piece from the source node.
	if w.config.DeleteAfterTransfer {
		pieceID := segment.RootPieceID.Derive(job.SourceNode, int32(pieceNum))
		deleteErr := w.deletePiece(ctx, sourceNode, pieceID)
		if deleteErr != nil {
			// Log but don't fail the job — the metabase update succeeded,
			// and GC will eventually clean up the orphaned piece.
			w.log.Warn("failed to delete source piece after transfer",
				zap.Stringer("stream_id", job.StreamID),
				zap.Uint64("position", job.Position),
				zap.Stringer("source_node", job.SourceNode),
				zap.Stringer("piece_id", pieceID),
				zap.Error(deleteErr))
		} else {
			w.log.Debug("source piece deleted",
				zap.Stringer("stream_id", job.StreamID),
				zap.Uint64("position", job.Position),
				zap.Stringer("source_node", job.SourceNode),
				zap.Stringer("piece_id", pieceID))
		}
	}

	w.log.Debug("piece transferred successfully",
		zap.Stringer("stream_id", job.StreamID),
		zap.Uint64("position", job.Position),
		zap.Int("piece_num", pieceNum),
		zap.Stringer("source", job.SourceNode),
		zap.Stringer("dest", job.DestNode))

	return nil
}

func (w *Worker) downloadPiece(ctx context.Context, segment metabase.Segment, node nodeselection.SelectedNode, pieceNum uint16, pieceSize int64) (_ []byte, _ *pb.PieceHash, err error) {
	defer mon.Task()(&ctx)(&err)

	signer, err := orders.NewSignerRepairGet(w.orders, segment.RootPieceID, time.Now(), pieceSize, metabase.BucketLocation{})
	if err != nil {
		return nil, nil, err
	}

	addressedLimit, err := signer.Sign(ctx, &pb.Node{
		Id:      node.ID,
		Address: node.Address,
	}, int32(pieceNum))
	if err != nil {
		return nil, nil, err
	}

	dialCtx, dialCancel := context.WithTimeout(ctx, w.config.DialTimeout)
	defer dialCancel()

	ps, err := w.dialPiecestore(dialCtx, storj.NodeURL{
		ID:      node.ID,
		Address: node.Address.Address,
	})
	if err != nil {
		return nil, nil, err
	}
	defer func() { err = errs.Combine(err, ps.Close()) }()

	downloadCtx, downloadCancel := context.WithTimeout(ctx, w.config.DownloadTimeout)
	defer downloadCancel()

	downloader, err := ps.Download(downloadCtx, addressedLimit.GetLimit(), signer.PrivateKey, 0, pieceSize)
	if err != nil {
		return nil, nil, err
	}
	defer func() { err = errs.Combine(err, downloader.Close()) }()

	// Read the data while computing a local hash to verify integrity.
	// The hash algorithm must match what the source node used, which is
	// only available after the first message is received from the node.
	var hasher lazyHashWriter
	hasher.downloader = downloader

	buf := make([]byte, pieceSize)
	n, err := io.ReadFull(io.TeeReader(downloader, &hasher), buf)
	if err != nil {
		return nil, nil, err
	}

	hash, _ := downloader.GetHashAndLimit()
	if hash != nil {
		calculatedHash := hasher.Sum(nil)
		if !bytes.Equal(hash.Hash, calculatedHash) {
			return nil, nil, Error.New("downloaded piece hash mismatch: expected %x, calculated %x", hash.Hash, calculatedHash)
		}
	}

	mon.IntVal("balancer_bytes_downloaded").Observe(int64(n))
	return buf[:n], hash, nil
}

// lazyHashWriter computes a hash using the algorithm from the download response.
// The hash algorithm is only available after the first message from the storage node.
type lazyHashWriter struct {
	hasher     hash.Hash
	downloader *piecestore.Download
}

func (l *lazyHashWriter) Write(p []byte) (n int, err error) {
	if l.hasher == nil {
		h, _ := l.downloader.GetHashAndLimit()
		if h == nil {
			return len(p), nil
		}
		l.hasher = pb.NewHashFromAlgorithm(h.HashAlgorithm)
	}
	return l.hasher.Write(p)
}

func (l *lazyHashWriter) Sum(b []byte) []byte {
	if l.hasher == nil {
		return []byte{}
	}
	return l.hasher.Sum(b)
}

func (w *Worker) uploadPiece(ctx context.Context, segment metabase.Segment, node nodeselection.SelectedNode, pieceNum uint16, pieceSize int64, data []byte, hashAlgo pb.PieceHashAlgorithm) (_ *pb.PieceHash, err error) {
	defer mon.Task()(&ctx)(&err)

	var expiration time.Time
	if segment.ExpiresAt != nil {
		expiration = *segment.ExpiresAt
	}

	signer, err := orders.NewSignerRepairPut(w.orders, segment.RootPieceID, expiration, time.Now(), pieceSize, metabase.BucketLocation{})
	if err != nil {
		return nil, err
	}

	addressedLimit, err := signer.Sign(ctx, &pb.Node{
		Id:      node.ID,
		Address: node.Address,
	}, int32(pieceNum))
	if err != nil {
		return nil, err
	}

	dialCtx, dialCancel := context.WithTimeout(ctx, w.config.DialTimeout)
	defer dialCancel()

	ps, err := w.dialPiecestore(dialCtx, storj.NodeURL{
		ID:      node.ID,
		Address: node.Address.Address,
	})
	if err != nil {
		return nil, err
	}
	defer func() { err = errs.Combine(err, ps.Close()) }()

	ps.UploadHashAlgo = hashAlgo

	uploadCtx, uploadCancel := context.WithTimeout(ctx, w.config.UploadTimeout)
	defer uploadCancel()

	hash, err := ps.UploadReader(uploadCtx, addressedLimit.GetLimit(), signer.PrivateKey, io.NopCloser(bytes.NewReader(data)))
	if err != nil {
		return nil, err
	}

	mon.IntVal("balancer_bytes_uploaded").Observe(int64(len(data)))
	return hash, nil
}

func (w *Worker) deletePiece(ctx context.Context, node nodeselection.SelectedNode, pieceID storj.PieceID) (err error) {
	defer mon.Task()(&ctx)(&err)

	dialCtx, dialCancel := context.WithTimeout(ctx, w.config.DialTimeout)
	defer dialCancel()

	conn, err := w.dialer.DialNodeURL(dialCtx, storj.NodeURL{
		ID:      node.ID,
		Address: node.Address.Address,
	})
	if err != nil {
		return Error.Wrap(err)
	}
	defer func() { err = errs.Combine(err, conn.Close()) }()

	client := pb.NewDRPCPiecestoreClient(conn)
	_, err = client.DeletePieces(ctx, &pb.DeletePiecesRequest{
		PieceIds: []storj.PieceID{pieceID},
	})
	return Error.Wrap(err)
}

func (w *Worker) dialPiecestore(ctx context.Context, target storj.NodeURL) (*piecestore.Client, error) {
	ctx = rpcpool.WithForceDial(ctx)
	return piecestore.Dial(ctx, w.dialer, target, piecestore.DefaultConfig)
}
