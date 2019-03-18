// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package ecclient

import (
	"context"
	"io"
	"io/ioutil"
	"sort"
	"sync/atomic"
	"time"

	"github.com/zeebo/errs"
	"go.uber.org/zap"
	monkit "gopkg.in/spacemonkeygo/monkit.v2"

	"storj.io/storj/internal/sync2"
	"storj.io/storj/pkg/auth/signing"
	"storj.io/storj/pkg/eestream"
	"storj.io/storj/pkg/pb"
	"storj.io/storj/pkg/ranger"
	"storj.io/storj/pkg/storj"
	"storj.io/storj/pkg/transport"
	"storj.io/storj/pkg/utils"
	"storj.io/storj/uplink/piecestore"
)

var mon = monkit.Package()

// Client defines an interface for storing erasure coded data to piece store nodes
type Client interface {
	Put(ctx context.Context, limits []*pb.AddressedOrderLimit, rs eestream.RedundancyStrategy, data io.Reader, expiration time.Time) (successfulNodes []*pb.Node, successfulHashes []*pb.PieceHash, err error)
	Get(ctx context.Context, limits []*pb.AddressedOrderLimit, es eestream.ErasureScheme, size int64) (ranger.Ranger, error)
	Delete(ctx context.Context, limits []*pb.AddressedOrderLimit) error
}

type psClientHelper func(context.Context, *pb.Node) (*piecestore.Client, error)

type ecClient struct {
	transport   transport.Client
	memoryLimit int
}

// NewClient from the given identity and max buffer memory
func NewClient(tc transport.Client, memoryLimit int) Client {
	return &ecClient{
		transport:   tc,
		memoryLimit: memoryLimit,
	}
}

func (ec *ecClient) newPSClient(ctx context.Context, n *pb.Node) (*piecestore.Client, error) {
	n.Type.DPanicOnInvalid("new ps client")
	conn, err := ec.transport.DialNode(ctx, n)
	if err != nil {
		return nil, err
	}
	return piecestore.NewClient(
		zap.L().Named(n.Id.String()),
		signing.SignerFromFullIdentity(ec.transport.Identity()),
		conn,
		piecestore.DefaultConfig,
	), nil
}

func (ec *ecClient) Put(ctx context.Context, limits []*pb.AddressedOrderLimit, rs eestream.RedundancyStrategy, data io.Reader, expiration time.Time) (successfulNodes []*pb.Node, successfulHashes []*pb.PieceHash, err error) {
	defer mon.Task()(&ctx)(&err)
	if len(limits) != rs.TotalCount() {
		return nil, nil, Error.New("size of limits slice (%d) does not match total count (%d) of erasure scheme", len(limits), rs.TotalCount())
	}

	if nonNilCount(limits) < rs.RepairThreshold() {
		return nil, nil, Error.New("number of non-nil limits (%d) is less than repair threshold (%d) of erasure scheme", nonNilCount(limits), rs.RepairThreshold())
	}

	if !unique(limits) {
		return nil, nil, Error.New("duplicated nodes are not allowed")
	}

	padded := eestream.PadReader(ioutil.NopCloser(data), rs.StripeSize())
	readers, err := eestream.EncodeReader(ctx, padded, rs)
	if err != nil {
		return nil, nil, err
	}

	type info struct {
		i    int
		err  error
		hash *pb.PieceHash
	}
	infos := make(chan info, len(limits))

	psCtx, cancel := context.WithCancel(ctx)
	defer cancel()

	start := time.Now()

	for i, addressedLimit := range limits {
		go func(i int, addressedLimit *pb.AddressedOrderLimit) {
			hash, err := ec.putPiece(psCtx, ctx, addressedLimit, readers[i], expiration)
			infos <- info{i: i, err: err, hash: hash}
		}(i, addressedLimit)
	}

	successfulNodes = make([]*pb.Node, len(limits))
	successfulHashes = make([]*pb.PieceHash, len(limits))
	var successfulCount int32
	var timer *time.Timer

	for range limits {
		info := <-infos
		if info.err != nil {
			zap.S().Debugf("Upload to storage node %s failed: %v", limits[info.i].GetLimit().StorageNodeId, info.err)
			continue
		}

		successfulNodes[info.i] = &pb.Node{
			Id:      limits[info.i].GetLimit().StorageNodeId,
			Address: limits[info.i].GetStorageNodeAddress(),
			Type:    pb.NodeType_STORAGE,
		}
		successfulHashes[info.i] = info.hash

		switch int(atomic.AddInt32(&successfulCount, 1)) {
		case rs.RepairThreshold():
			elapsed := time.Since(start)
			more := elapsed * 3 / 2

			zap.S().Infof("Repair threshold (%d nodes) reached in %.2f s. Starting a timer for %.2f s for reaching the success threshold (%d nodes)...",
				rs.RepairThreshold(), elapsed.Seconds(), more.Seconds(), rs.OptimalThreshold())

			timer = time.AfterFunc(more, func() {
				if ctx.Err() != context.Canceled {
					zap.S().Infof("Timer expired. Successfully uploaded to %d nodes. Canceling the long tail...", atomic.LoadInt32(&successfulCount))
					cancel()
				}
			})
		case rs.OptimalThreshold():
			zap.S().Infof("Success threshold (%d nodes) reached. Canceling the long tail...", rs.OptimalThreshold())
			timer.Stop()
			cancel()
		}
	}

	// Ensure timer is stopped in the case of repair threshold is reached, but
	// not the success threshold due to errors instead of slowness.
	if timer != nil {
		timer.Stop()
	}

	/* clean up the partially uploaded segment's pieces */
	defer func() {
		select {
		case <-ctx.Done():
			err = utils.CombineErrors(
				Error.New("upload cancelled by user"),
				// ec.Delete(context.Background(), nodes, pieceID, pba.SatelliteId), //TODO
			)
		default:
		}
	}()

	if int(atomic.LoadInt32(&successfulCount)) < rs.RepairThreshold() {
		return nil, nil, Error.New("successful puts (%d) less than repair threshold (%d)", successfulCount, rs.RepairThreshold())
	}

	return successfulNodes, successfulHashes, nil
}

func (ec *ecClient) putPiece(ctx, parent context.Context, limit *pb.AddressedOrderLimit, data io.ReadCloser, expiration time.Time) (hash *pb.PieceHash, err error) {
	defer func() { err = errs.Combine(err, data.Close()) }()

	if limit == nil {
		_, err = io.Copy(ioutil.Discard, data)
		return nil, err
	}

	storageNodeID := limit.GetLimit().StorageNodeId
	pieceID := limit.GetLimit().PieceId
	ps, err := ec.newPSClient(ctx, &pb.Node{
		Id:      storageNodeID,
		Address: limit.GetStorageNodeAddress(),
		Type:    pb.NodeType_STORAGE,
	})
	if err != nil {
		zap.S().Errorf("Failed dialing for putting piece %s to node %s: %v", pieceID, storageNodeID, err)
		return nil, err
	}
	defer func() { err = errs.Combine(err, ps.Close()) }()

	upload, err := ps.Upload(ctx, limit.GetLimit())
	if err != nil {
		zap.S().Errorf("Failed requesting upload of piece %s to node %s: %v", pieceID, storageNodeID, err)
		return nil, err
	}
	defer func() {
		if ctx.Err() != nil || err != nil {
			hash = nil
			err = errs.Combine(err, upload.Cancel())
			return
		}
		h, closeErr := upload.Commit()
		hash = h
		err = errs.Combine(err, closeErr)
	}()

	_, err = sync2.Copy(ctx, upload, data)
	// Canceled context means the piece upload was interrupted by user or due
	// to slow connection. No error logging for this case.
	if ctx.Err() == context.Canceled {
		if parent.Err() == context.Canceled {
			zap.S().Infof("Upload to node %s canceled by user.", storageNodeID)
		} else {
			zap.S().Infof("Node %s cut from upload due to slow connection.", storageNodeID)
		}
		err = context.Canceled
	} else if err != nil {
		nodeAddress := "nil"
		if limit.GetStorageNodeAddress() != nil {
			nodeAddress = limit.GetStorageNodeAddress().GetAddress()
		}
		zap.S().Errorf("Failed uploading piece %s to node %s (%+v): %v", pieceID, storageNodeID, nodeAddress, err)
	}

	return hash, err
}

func (ec *ecClient) Get(ctx context.Context, limits []*pb.AddressedOrderLimit, es eestream.ErasureScheme, size int64) (rr ranger.Ranger, err error) {
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
			newPSClientHelper: ec.newPSClient,
			limit:             addressedLimit,
			size:              pieceSize,
		}
	}

	rr, err = eestream.Decode(rrs, es, ec.memoryLimit)
	if err != nil {
		return nil, err
	}

	return eestream.Unpad(rr, int(paddedSize-size))
}

func (ec *ecClient) Delete(ctx context.Context, limits []*pb.AddressedOrderLimit) (err error) {
	defer mon.Task()(&ctx)(&err)

	errch := make(chan error, len(limits))
	for _, addressedLimit := range limits {
		if addressedLimit == nil {
			errch <- nil
			continue
		}

		go func(addressedLimit *pb.AddressedOrderLimit) {
			limit := addressedLimit.GetLimit()
			ps, err := ec.newPSClient(ctx, &pb.Node{
				Id:      limit.StorageNodeId,
				Address: addressedLimit.GetStorageNodeAddress(),
				Type:    pb.NodeType_STORAGE,
			})
			if err != nil {
				zap.S().Errorf("Failed dialing for deleting piece %s from node %s: %v", limit.PieceId, limit.StorageNodeId, err)
				errch <- err
				return
			}
			err = ps.Delete(ctx, limit)
			err = errs.Combine(err, ps.Close())
			if err != nil {
				zap.S().Errorf("Failed deleting piece %s from node %s: %v", limit.PieceId, limit.StorageNodeId, err)
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
	newPSClientHelper psClientHelper
	limit             *pb.AddressedOrderLimit
	size              int64
}

// Size implements Ranger.Size
func (lr *lazyPieceRanger) Size() int64 {
	return lr.size
}

// Range implements Ranger.Range to be lazily connected
func (lr *lazyPieceRanger) Range(ctx context.Context, offset, length int64) (io.ReadCloser, error) {
	ps, err := lr.newPSClientHelper(ctx, &pb.Node{
		Id:      lr.limit.GetLimit().StorageNodeId,
		Address: lr.limit.GetStorageNodeAddress(),
		Type:    pb.NodeType_STORAGE,
	})
	if err != nil {
		return nil, err
	}
	return ps.Download(ctx, lr.limit.GetLimit(), offset, length)
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
