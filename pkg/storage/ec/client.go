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

	"storj.io/storj/pkg/eestream"
	"storj.io/storj/pkg/pb"
	"storj.io/storj/pkg/piecestore/psclient"
	"storj.io/storj/pkg/ranger"
	"storj.io/storj/pkg/storj"
	"storj.io/storj/pkg/transport"
	"storj.io/storj/pkg/utils"
)

var mon = monkit.Package()

// Client defines an interface for storing erasure coded data to piece store nodes
type Client interface {
	Put(ctx context.Context, nodes []*pb.Node, rs eestream.RedundancyStrategy, pieceID psclient.PieceID, data io.Reader, expiration time.Time, pba *pb.PayerBandwidthAllocation, authorization *pb.SignedMessage) (successfulNodes []*pb.Node, err error)
	Get(ctx context.Context, nodes []*pb.Node, es eestream.ErasureScheme, pieceID psclient.PieceID, size int64, pba *pb.PayerBandwidthAllocation, authorization *pb.SignedMessage) (ranger.Ranger, error)
	Delete(ctx context.Context, nodes []*pb.Node, pieceID psclient.PieceID, authorization *pb.SignedMessage) error
}

type psClientFunc func(context.Context, transport.Client, *pb.Node, int) (psclient.Client, error)
type psClientHelper func(context.Context, *pb.Node) (psclient.Client, error)

type ecClient struct {
	transport       transport.Client
	memoryLimit     int
	newPSClientFunc psClientFunc
}

// NewClient from the given identity and max buffer memory
func NewClient(tc transport.Client, memoryLimit int) Client {
	return &ecClient{
		transport:       tc,
		memoryLimit:     memoryLimit,
		newPSClientFunc: psclient.NewPSClient,
	}
}

func (ec *ecClient) newPSClient(ctx context.Context, n *pb.Node) (psclient.Client, error) {
	n.Type.DPanicOnInvalid("new ps client")
	return ec.newPSClientFunc(ctx, ec.transport, n, 0)
}

func (ec *ecClient) Put(ctx context.Context, nodes []*pb.Node, rs eestream.RedundancyStrategy, pieceID psclient.PieceID, data io.Reader, expiration time.Time, pba *pb.PayerBandwidthAllocation, authorization *pb.SignedMessage) (successfulNodes []*pb.Node, err error) {
	defer mon.Task()(&ctx)(&err)
	if len(nodes) != rs.TotalCount() {
		return nil, Error.New("size of nodes slice (%d) does not match total count (%d) of erasure scheme", len(nodes), rs.TotalCount())
	}

	if nonNilCount(nodes) < rs.RepairThreshold() {
		return nil, Error.New("number of non-nil nodes (%d) is less than repair threshold (%d) of erasure scheme", nonNilCount(nodes), rs.RepairThreshold())
	}

	if !unique(nodes) {
		return nil, Error.New("duplicated nodes are not allowed")
	}

	padded := eestream.PadReader(ioutil.NopCloser(data), rs.StripeSize())
	readers, err := eestream.EncodeReader(ctx, padded, rs)
	if err != nil {
		return nil, err
	}

	type info struct {
		i   int
		err error
	}
	infos := make(chan info, len(nodes))

	psCtx, cancel := context.WithCancel(ctx)
	defer cancel()

	start := time.Now()

	for i, node := range nodes {
		if node != nil {
			node.Type.DPanicOnInvalid("ec client Put")
		}

		go func(i int, node *pb.Node) {
			err := ec.putPiece(psCtx, ctx, node, pieceID, readers[i], expiration, pba, authorization)
			infos <- info{i: i, err: err}
		}(i, node)
	}

	successfulNodes = make([]*pb.Node, len(nodes))
	var successfulCount int32
	var timer *time.Timer

	for range nodes {
		info := <-infos
		if info.err == nil {
			successfulNodes[info.i] = nodes[info.i]

			switch int(atomic.AddInt32(&successfulCount, 1)) {
			case rs.RepairThreshold():
				elapsed := time.Since(start)
				more := elapsed * 3 / 2

				zap.S().Infof("Repair threshold (%d nodes) reached in %.2f s. Starting a timer for %.2f s for reaching the success threshold (%d nodes)...",
					rs.RepairThreshold(), elapsed.Seconds(), more.Seconds(), rs.OptimalThreshold())

				timer = time.AfterFunc(more, func() {
					zap.S().Infof("Timer expired. Successfully uploaded to %d nodes. Canceling the long tail...", atomic.LoadInt32(&successfulCount))
					cancel()
				})
			case rs.OptimalThreshold():
				zap.S().Infof("Success threshold (%d nodes) reached. Canceling the long tail...", rs.OptimalThreshold())
				timer.Stop()
				cancel()
			}
		}
	}

	/* clean up the partially uploaded segment's pieces */
	defer func() {
		select {
		case <-ctx.Done():
			err = utils.CombineErrors(
				Error.New("upload cancelled by user"),
				ec.Delete(context.Background(), nodes, pieceID, authorization),
			)
		default:
		}
	}()

	if int(atomic.LoadInt32(&successfulCount)) < rs.RepairThreshold() {
		return nil, Error.New("successful puts (%d) less than repair threshold (%d)", successfulCount, rs.RepairThreshold())
	}

	return successfulNodes, nil
}

func (ec *ecClient) putPiece(ctx, parent context.Context, node *pb.Node, pieceID psclient.PieceID, data io.ReadCloser, expiration time.Time, pba *pb.PayerBandwidthAllocation, authorization *pb.SignedMessage) (err error) {
	defer func() { err = errs.Combine(err, data.Close()) }()

	if node == nil {
		_, err = io.Copy(ioutil.Discard, data)
		return err
	}
	derivedPieceID, err := pieceID.Derive(node.Id.Bytes())

	if err != nil {
		zap.S().Errorf("Failed deriving piece id for %s: %v", pieceID, err)
		return err
	}
	ps, err := ec.newPSClient(ctx, node)
	if err != nil {
		zap.S().Errorf("Failed dialing for putting piece %s -> %s to node %s: %v",
			pieceID, derivedPieceID, node.Id, err)
		return err
	}
	err = ps.Put(ctx, derivedPieceID, data, expiration, pba, authorization)
	defer func() { err = errs.Combine(err, ps.Close()) }()
	// Canceled context means the piece upload was interrupted by user or due
	// to slow connection. No error logging for this case.
	if ctx.Err() == context.Canceled {
		if parent.Err() == context.Canceled {
			zap.S().Infof("Upload to node %s canceled by user.", node.Id)
		} else {
			zap.S().Infof("Node %s cut from upload due to slow connection.", node.Id)
		}
		err = context.Canceled
	} else if err != nil {
		nodeAddress := "nil"
		if node.Address != nil {
			nodeAddress = node.Address.Address
		}
		zap.S().Errorf("Failed putting piece %s -> %s to node %s (%+v): %v",
			pieceID, derivedPieceID, node.Id, nodeAddress, err)
	}

	return err
}

func (ec *ecClient) Get(ctx context.Context, nodes []*pb.Node, es eestream.ErasureScheme,
	pieceID psclient.PieceID, size int64, pba *pb.PayerBandwidthAllocation, authorization *pb.SignedMessage) (rr ranger.Ranger, err error) {
	defer mon.Task()(&ctx)(&err)

	if len(nodes) != es.TotalCount() {
		return nil, Error.New("size of nodes slice (%d) does not match total count (%d) of erasure scheme", len(nodes), es.TotalCount())
	}

	if nonNilCount(nodes) < es.RequiredCount() {
		return nil, Error.New("number of non-nil nodes (%d) is less than required count (%d) of erasure scheme", nonNilCount(nodes), es.RequiredCount())
	}

	paddedSize := calcPadded(size, es.StripeSize())
	pieceSize := paddedSize / int64(es.RequiredCount())
	rrs := map[int]ranger.Ranger{}

	type rangerInfo struct {
		i   int
		rr  ranger.Ranger
		err error
	}
	ch := make(chan rangerInfo, len(nodes))

	for i, n := range nodes {

		if n != nil {
			n.Type.DPanicOnInvalid("ec client Get")
		}

		if n == nil {
			ch <- rangerInfo{i: i, rr: nil, err: nil}
			continue
		}

		go func(i int, n *pb.Node) {
			derivedPieceID, err := pieceID.Derive(n.Id.Bytes())
			if err != nil {
				zap.S().Errorf("Failed deriving piece id for %s: %v", pieceID, err)
				ch <- rangerInfo{i: i, rr: nil, err: err}
				return
			}

			rr := &lazyPieceRanger{
				newPSClientHelper: ec.newPSClient,
				node:              n,
				id:                derivedPieceID,
				size:              pieceSize,
				pba:               pba,
				authorization:     authorization,
			}

			ch <- rangerInfo{i: i, rr: rr, err: nil}
		}(i, n)
	}

	for range nodes {
		rri := <-ch
		if rri.err == nil && rri.rr != nil {
			rrs[rri.i] = rri.rr
		}
	}

	rr, err = eestream.Decode(rrs, es, ec.memoryLimit)
	if err != nil {
		return nil, err
	}

	return eestream.Unpad(rr, int(paddedSize-size))
}

func (ec *ecClient) Delete(ctx context.Context, nodes []*pb.Node, pieceID psclient.PieceID, authorization *pb.SignedMessage) (err error) {
	defer mon.Task()(&ctx)(&err)

	errch := make(chan error, len(nodes))
	for _, v := range nodes {
		if v != nil {
			v.Type.DPanicOnInvalid("ec client delete")
		}
	}
	for _, n := range nodes {
		if n == nil {
			errch <- nil
			continue
		}

		go func(n *pb.Node) {
			derivedPieceID, err := pieceID.Derive(n.Id.Bytes())
			if err != nil {
				zap.S().Errorf("Failed deriving piece id for %s: %v", pieceID, err)
				errch <- err
				return
			}
			ps, err := ec.newPSClient(ctx, n)
			if err != nil {
				zap.S().Errorf("Failed dialing for deleting piece %s -> %s from node %s: %v",
					pieceID, derivedPieceID, n.Id, err)
				errch <- err
				return
			}
			err = ps.Delete(ctx, derivedPieceID, authorization)
			// normally the bellow call should be deferred, but doing so fails
			// randomly the unit tests
			err = errs.Combine(err, ps.Close())
			if err != nil {
				zap.S().Errorf("Failed deleting piece %s -> %s from node %s: %v",
					pieceID, derivedPieceID, n.Id, err)
			}
			errch <- err
		}(n)
	}

	allerrs := collectErrors(errch, len(nodes))
	for _, v := range nodes {
		if v != nil {
			v.Type.DPanicOnInvalid("ec client delete 2")
		}
	}
	if len(allerrs) > 0 && len(allerrs) == len(nodes) {
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

func unique(nodes []*pb.Node) bool {
	if len(nodes) < 2 {
		return true
	}
	ids := make(storj.NodeIDList, len(nodes))
	for i, n := range nodes {
		if n != nil {
			ids[i] = n.Id
			n.Type.DPanicOnInvalid("ec client unique")
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
	ranger            ranger.Ranger
	newPSClientHelper psClientHelper
	node              *pb.Node
	id                psclient.PieceID
	size              int64
	pba               *pb.PayerBandwidthAllocation
	authorization     *pb.SignedMessage
}

// Size implements Ranger.Size
func (lr *lazyPieceRanger) Size() int64 {
	return lr.size
}

// Range implements Ranger.Range to be lazily connected
func (lr *lazyPieceRanger) Range(ctx context.Context, offset, length int64) (io.ReadCloser, error) {
	lr.node.Type.DPanicOnInvalid("Range")
	if lr.ranger == nil {
		ps, err := lr.newPSClientHelper(ctx, lr.node)
		if err != nil {
			return nil, err
		}
		ranger, err := ps.Get(ctx, lr.id, lr.size, lr.pba, lr.authorization)
		if err != nil {
			return nil, err
		}
		lr.ranger = ranger
	}
	return lr.ranger.Range(ctx, offset, length)
}

func nonNilCount(nodes []*pb.Node) int {
	total := 0
	for _, node := range nodes {
		if node != nil {
			total++
			node.Type.DPanicOnInvalid("nonNilCount")
		}
	}
	return total
}
