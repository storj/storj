// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package ecclient

import (
	"context"
	"io"
	"io/ioutil"
	"sort"
	"sync"
	"sync/atomic"
	"time"

	"github.com/zeebo/errs"
	"go.uber.org/zap"
	monkit "gopkg.in/spacemonkeygo/monkit.v2"

	"storj.io/storj/internal/errs2"
	"storj.io/storj/internal/sync2"
	"storj.io/storj/pkg/pb"
	"storj.io/storj/pkg/ranger"
	"storj.io/storj/pkg/storj"
	"storj.io/storj/pkg/transport"
	"storj.io/storj/uplink/eestream"
	"storj.io/storj/uplink/piecestore"
)

var mon = monkit.Package()

// Client defines an interface for storing erasure coded data to piece store nodes
type Client interface {
	Put(ctx context.Context, limits []*pb.AddressedOrderLimit, privateKey storj.PiecePrivateKey, rs eestream.RedundancyStrategy, data io.Reader, expiration time.Time) (successfulNodes []*pb.Node, successfulHashes []*pb.PieceHash, err error)
	Repair(ctx context.Context, limits []*pb.AddressedOrderLimit, privateKey storj.PiecePrivateKey, rs eestream.RedundancyStrategy, data io.Reader, expiration time.Time, timeout time.Duration, path storj.Path) (successfulNodes []*pb.Node, successfulHashes []*pb.PieceHash, err error)
	Get(ctx context.Context, limits []*pb.AddressedOrderLimit, privateKey storj.PiecePrivateKey, es eestream.ErasureScheme, size int64) (ranger.Ranger, error)
	Delete(ctx context.Context, limits []*pb.AddressedOrderLimit, privateKey storj.PiecePrivateKey) error
	WithForceErrorDetection(force bool) Client
}

type dialPiecestoreFunc func(context.Context, *pb.Node) (*piecestore.Client, error)

type ecClient struct {
	log                 *zap.Logger
	transport           transport.Client
	memoryLimit         int
	forceErrorDetection bool
}

// NewClient from the given identity and max buffer memory
func NewClient(log *zap.Logger, tc transport.Client, memoryLimit int) Client {
	return &ecClient{
		log:         log,
		transport:   tc,
		memoryLimit: memoryLimit,
	}
}

func (ec *ecClient) WithForceErrorDetection(force bool) Client {
	ec.forceErrorDetection = force
	return ec
}

func (ec *ecClient) dialPiecestore(ctx context.Context, n *pb.Node) (*piecestore.Client, error) {
	logger := ec.log.Named(n.Id.String())
	return piecestore.Dial(ctx, ec.transport, n, logger, piecestore.DefaultConfig)
}

func (ec *ecClient) Put(ctx context.Context, limits []*pb.AddressedOrderLimit, privateKey storj.PiecePrivateKey, rs eestream.RedundancyStrategy, data io.Reader, expiration time.Time) (successfulNodes []*pb.Node, successfulHashes []*pb.PieceHash, err error) {
	defer mon.Task()(&ctx)(&err)

	pieceCount := len(limits)
	if pieceCount != rs.TotalCount() {
		return nil, nil, Error.New("size of limits slice (%d) does not match total count (%d) of erasure scheme", pieceCount, rs.TotalCount())
	}

	nonNilLimits := nonNilCount(limits)
	if nonNilLimits <= rs.RepairThreshold() && nonNilLimits < rs.OptimalThreshold() {
		return nil, nil, Error.New("number of non-nil limits (%d) is less than or equal to the repair threshold (%d) of erasure scheme", nonNilLimits, rs.RepairThreshold())
	}

	if !unique(limits) {
		return nil, nil, Error.New("duplicated nodes are not allowed")
	}

	ec.log.Debug("Uploading to storage nodes",
		zap.Int("Erasure Share Size", rs.ErasureShareSize()),
		zap.Int("Stripe Size", rs.StripeSize()),
		zap.Int("Repair Threshold", rs.RepairThreshold()),
		zap.Int("Optimal Threshold", rs.OptimalThreshold()),
	)

	padded := eestream.PadReader(ioutil.NopCloser(data), rs.StripeSize())
	readers, err := eestream.EncodeReader(ctx, ec.log, padded, rs)
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

	successfulNodes = make([]*pb.Node, pieceCount)
	successfulHashes = make([]*pb.PieceHash, pieceCount)
	var successfulCount, failureCount, cancellationCount int32
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
			ec.log.Debug("Upload to storage node failed",
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

		atomic.AddInt32(&successfulCount, 1)

		if int(successfulCount) >= rs.OptimalThreshold() {
			ec.log.Info("Success threshold reached. Cancelling remaining uploads.",
				zap.Int("Optimal Threshold", rs.OptimalThreshold()),
			)
			cancel()
		}
	}

	defer func() {
		select {
		case <-ctx.Done():
			err = Error.New("upload cancelled by user")
			// TODO: clean up the partially uploaded segment's pieces
			// ec.Delete(context.Background(), nodes, pieceID, pba.SatelliteId),
		default:
		}
	}()

	successes := int(atomic.LoadInt32(&successfulCount))
	mon.IntVal("put_segment_pieces_total").Observe(int64(pieceCount))
	mon.IntVal("put_segment_pieces_optimal").Observe(int64(rs.OptimalThreshold()))
	mon.IntVal("put_segment_pieces_successful").Observe(int64(successes))
	mon.IntVal("put_segment_pieces_failed").Observe(int64(failureCount))
	mon.IntVal("put_segment_pieces_canceled").Observe(int64(cancellationCount))

	if successes <= rs.RepairThreshold() && successes < rs.OptimalThreshold() {
		return nil, nil, Error.New("successful puts (%d) less than or equal to repair threshold (%d)", successes, rs.RepairThreshold())
	}

	if successes < rs.OptimalThreshold() {
		return nil, nil, Error.New("successful puts (%d) less than success threshold (%d)", successes, rs.OptimalThreshold())
	}

	return successfulNodes, successfulHashes, nil
}

func (ec *ecClient) Repair(ctx context.Context, limits []*pb.AddressedOrderLimit, privateKey storj.PiecePrivateKey, rs eestream.RedundancyStrategy, data io.Reader, expiration time.Time, timeout time.Duration, path storj.Path) (successfulNodes []*pb.Node, successfulHashes []*pb.PieceHash, err error) {
	defer mon.Task()(&ctx)(&err)

	pieceCount := len(limits)
	if pieceCount != rs.TotalCount() {
		return nil, nil, Error.New("size of limits slice (%d) does not match total count (%d) of erasure scheme", pieceCount, rs.TotalCount())
	}

	if !unique(limits) {
		return nil, nil, Error.New("duplicated nodes are not allowed")
	}

	padded := eestream.PadReader(ioutil.NopCloser(data), rs.StripeSize())
	readers, err := eestream.EncodeReader(ctx, ec.log, padded, rs)
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

func (ec *ecClient) putPiece(ctx, parent context.Context, limit *pb.AddressedOrderLimit, privateKey storj.PiecePrivateKey, data io.ReadCloser, expiration time.Time) (hash *pb.PieceHash, err error) {
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

func (ec *ecClient) Get(ctx context.Context, limits []*pb.AddressedOrderLimit, privateKey storj.PiecePrivateKey, es eestream.ErasureScheme, size int64) (rr ranger.Ranger, err error) {
	defer mon.Task()(&ctx)(&err)

	if len(limits) != es.TotalCount() {
		return nil, Error.New("size of limits slice (%d) does not match total count (%d) of erasure scheme", len(limits), es.TotalCount())
	}

	if nonNilCount(limits) < es.RequiredCount() {
		return nil, Error.New("number of non-nil limits (%d) is less than required count (%d) of erasure scheme", nonNilCount(limits), es.RequiredCount())
	}

	paddedSize := calcPadded(size, es.StripeSize())
	pieceSize := paddedSize / int64(es.RequiredCount())

	rrs := map[int]ranger.Ranger{}
	for i, addressedLimit := range limits {
		if addressedLimit == nil {
			continue
		}

		rrs[i] = &lazyPieceRanger{
			dialPiecestore: ec.dialPiecestore,
			limit:          addressedLimit,
			privateKey:     privateKey,
			size:           pieceSize,
		}
	}

	rr, err = eestream.Decode(ec.log, rrs, es, ec.memoryLimit, ec.forceErrorDetection)
	if err != nil {
		return nil, Error.Wrap(err)
	}

	ranger, err := eestream.Unpad(rr, int(paddedSize-size))
	return ranger, Error.Wrap(err)
}

func (ec *ecClient) Delete(ctx context.Context, limits []*pb.AddressedOrderLimit, privateKey storj.PiecePrivateKey) (err error) {
	defer mon.Task()(&ctx)(&err)

	errch := make(chan error, len(limits))
	for _, addressedLimit := range limits {
		if addressedLimit == nil {
			errch <- nil
			continue
		}

		go func(addressedLimit *pb.AddressedOrderLimit) {
			limit := addressedLimit.GetLimit()
			ps, err := ec.dialPiecestore(ctx, &pb.Node{
				Id:      limit.StorageNodeId,
				Address: addressedLimit.GetStorageNodeAddress(),
			})
			if err != nil {
				ec.log.Debug("Failed dialing for deleting piece from node",
					zap.String("PieceID", limit.PieceId.String()),
					zap.String("NodeID", limit.StorageNodeId.String()),
					zap.Error(err),
				)
				errch <- err
				return
			}
			err = ps.Delete(ctx, limit, privateKey)
			err = errs.Combine(err, ps.Close())
			if err != nil {
				ec.log.Debug("Failed deleting piece from node",
					zap.String("PieceID", limit.PieceId.String()),
					zap.String("NodeID", limit.StorageNodeId.String()),
					zap.Error(err),
				)
			}
			errch <- err
		}(addressedLimit)
	}

	allerrs := collectErrors(errch, len(limits))
	if len(allerrs) > 0 && len(allerrs) == len(limits) {
		return allerrs[0]
	}

	return nil
}

func collectErrors(errs <-chan error, size int) []error {
	var result []error
	for i := 0; i < size; i++ {
		err := <-errs
		if err != nil {
			result = append(result, err)
		}
	}
	return result
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

func calcPadded(size int64, blockSize int) int64 {
	mod := size % int64(blockSize)
	if mod == 0 {
		return size
	}
	return size + int64(blockSize) - mod
}

type lazyPieceRanger struct {
	dialPiecestore dialPiecestoreFunc
	limit          *pb.AddressedOrderLimit
	privateKey     storj.PiecePrivateKey
	size           int64
}

// Size implements Ranger.Size
func (lr *lazyPieceRanger) Size() int64 {
	return lr.size
}

// Range implements Ranger.Range to be lazily connected
func (lr *lazyPieceRanger) Range(ctx context.Context, offset, length int64) (_ io.ReadCloser, err error) {
	defer mon.Task()(&ctx)(&err)

	return &lazyPieceReader{
		ranger: lr,
		ctx:    ctx,
		offset: offset,
		length: length,
	}, nil
}

type lazyPieceReader struct {
	ranger *lazyPieceRanger
	ctx    context.Context
	offset int64
	length int64

	mu sync.Mutex

	isClosed bool
	piecestore.Downloader
	client *piecestore.Client
}

func (lr *lazyPieceReader) Read(data []byte) (_ int, err error) {
	lr.mu.Lock()
	defer lr.mu.Unlock()

	if lr.isClosed {
		return 0, io.EOF
	}
	if lr.Downloader == nil {
		client, downloader, err := lr.ranger.dial(lr.ctx, lr.offset, lr.length)
		if err != nil {
			return 0, err
		}
		lr.Downloader = downloader
		lr.client = client
	}

	return lr.Downloader.Read(data)
}

func (lr *lazyPieceRanger) dial(ctx context.Context, offset, length int64) (_ *piecestore.Client, _ piecestore.Downloader, err error) {
	defer mon.Task()(&ctx)(&err)
	ps, err := lr.dialPiecestore(ctx, &pb.Node{
		Id:      lr.limit.GetLimit().StorageNodeId,
		Address: lr.limit.GetStorageNodeAddress(),
	})
	if err != nil {
		return nil, nil, err
	}

	download, err := ps.Download(ctx, lr.limit.GetLimit(), lr.privateKey, offset, length)
	if err != nil {
		return nil, nil, errs.Combine(err, ps.Close())
	}
	return ps, download, nil
}

func (lr *lazyPieceReader) Close() (err error) {
	lr.mu.Lock()
	defer lr.mu.Unlock()

	if lr.isClosed {
		return nil
	}
	lr.isClosed = true

	if lr.Downloader != nil {
		err = errs.Combine(err, lr.Downloader.Close())
	}
	if lr.client != nil {
		err = errs.Combine(err, lr.client.Close())
	}
	return err
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
