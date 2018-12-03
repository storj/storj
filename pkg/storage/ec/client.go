// Copyright (C) 2018 Storj Labs, Inc.
// See LICENSE for copying information.

package ecclient

import (
	"context"
	"io"
	"io/ioutil"
	"sort"
	"time"

	"go.uber.org/zap"
	"gopkg.in/spacemonkeygo/monkit.v2"

	"storj.io/storj/pkg/eestream"
	"storj.io/storj/pkg/pb"
	"storj.io/storj/pkg/piecestore/psclient"
	"storj.io/storj/pkg/provider"
	"storj.io/storj/pkg/ranger"
	"storj.io/storj/pkg/storj"
	"storj.io/storj/pkg/transport"
	"storj.io/storj/pkg/utils"
)

var mon = monkit.Package()

// Client defines an interface for storing erasure coded data to piece store nodes
type Client interface {
	Put(ctx context.Context, nodes []*pb.Node, rs eestream.RedundancyStrategy,
		pieceID psclient.PieceID, data io.Reader, expiration time.Time, pba *pb.PayerBandwidthAllocation, authorization *pb.SignedMessage) (successfulNodes []*pb.Node, err error)
	Get(ctx context.Context, nodes []*pb.Node, es eestream.ErasureScheme,
		pieceID psclient.PieceID, size int64, pba *pb.PayerBandwidthAllocation, authorization *pb.SignedMessage) (ranger.Ranger, error)
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
func NewClient(identity *provider.FullIdentity, memoryLimit int) Client {
	tc := transport.NewClient(identity)
	return &ecClient{
		transport:       tc,
		memoryLimit:     memoryLimit,
		newPSClientFunc: psclient.NewPSClient,
	}
}

func (ec *ecClient) newPSClient(ctx context.Context, n *pb.Node) (psclient.Client, error) {
	return ec.newPSClientFunc(ctx, ec.transport, n, 0)
}

func (ec *ecClient) Put(ctx context.Context, nodes []*pb.Node, rs eestream.RedundancyStrategy,
	pieceID psclient.PieceID, data io.Reader, expiration time.Time, pba *pb.PayerBandwidthAllocation, authorization *pb.SignedMessage) (successfulNodes []*pb.Node, err error) {
	defer mon.Task()(&ctx)(&err)

	if len(nodes) != rs.TotalCount() {
		return nil, Error.New("number of nodes (%d) do not match total count (%d) of erasure scheme", len(nodes), rs.TotalCount())
	}
	if !unique(nodes) {
		return nil, Error.New("duplicated nodes are not allowed")
	}

	padded := eestream.PadReader(ioutil.NopCloser(data), rs.StripeSize())
	readers, err := eestream.EncodeReader(ctx, padded, rs, ec.memoryLimit)
	if err != nil {
		return nil, err
	}

	type info struct {
		i   int
		err error
	}
	infos := make(chan info, len(nodes))

	for i, n := range nodes {

		go func(i int, n *pb.Node) {
			if n == nil {
				_, err := io.Copy(ioutil.Discard, readers[i])
				infos <- info{i: i, err: err}
				return
			}
			derivedPieceID, err := pieceID.Derive(n.Id.Bytes())

			if err != nil {
				zap.S().Errorf("Failed deriving piece id for %s: %v", pieceID, err)
				infos <- info{i: i, err: err}
				return
			}
			ps, err := ec.newPSClient(ctx, n)
			if err != nil {
				zap.S().Errorf("Failed dialing for putting piece %s -> %s to node %s: %v",
					pieceID, derivedPieceID, n.Id, err)
				infos <- info{i: i, err: err}
				return
			}
			err = ps.Put(ctx, derivedPieceID, readers[i], expiration, pba, authorization)
			// normally the bellow call should be deferred, but doing so fails
			// randomly the unit tests
			utils.LogClose(ps)
			// io.ErrUnexpectedEOF means the piece upload was interrupted due to slow connection.
			// No error logging for this case.
			if err != nil && err != io.ErrUnexpectedEOF {
				zap.S().Errorf("Failed putting piece %s -> %s to node %s: %v",
					pieceID, derivedPieceID, n.Id, err)
			}
			infos <- info{i: i, err: err}
		}(i, n)
	}

	successfulNodes = make([]*pb.Node, len(nodes))
	var successfulCount int
	for range nodes {
		info := <-infos
		if info.err == nil {
			successfulNodes[info.i] = nodes[info.i]
			successfulCount++
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

	if successfulCount < rs.RepairThreshold() {
		return nil, Error.New("successful puts (%d) less than repair threshold (%d)", successfulCount, rs.RepairThreshold())
	}

	return successfulNodes, nil
}

func (ec *ecClient) Get(ctx context.Context, nodes []*pb.Node, es eestream.ErasureScheme,
	pieceID psclient.PieceID, size int64, pba *pb.PayerBandwidthAllocation, authorization *pb.SignedMessage) (rr ranger.Ranger, err error) {
	defer mon.Task()(&ctx)(&err)

	validNodeCount := validCount(nodes)
	if validNodeCount < es.RequiredCount() {
		return nil, Error.New("number of nodes (%v) do not match minimum required count (%v) of erasure scheme", len(nodes), es.RequiredCount())
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

	errs := make(chan error, len(nodes))

	for _, n := range nodes {
		if n == nil {
			errs <- nil
			continue
		}

		go func(n *pb.Node) {
			derivedPieceID, err := pieceID.Derive(n.Id.Bytes())
			if err != nil {
				zap.S().Errorf("Failed deriving piece id for %s: %v", pieceID, err)
				errs <- err
				return
			}
			ps, err := ec.newPSClient(ctx, n)
			if err != nil {
				zap.S().Errorf("Failed dialing for deleting piece %s -> %s from node %s: %v",
					pieceID, derivedPieceID, n.Id, err)
				errs <- err
				return
			}
			err = ps.Delete(ctx, derivedPieceID, authorization)
			// normally the bellow call should be deferred, but doing so fails
			// randomly the unit tests
			utils.LogClose(ps)
			if err != nil {
				zap.S().Errorf("Failed deleting piece %s -> %s from node %s: %v",
					pieceID, derivedPieceID, n.Id, err)
			}
			errs <- err
		}(n)
	}

	allerrs := collectErrors(errs, len(nodes))

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

func validCount(nodes []*pb.Node) int {
	total := 0
	for _, node := range nodes {
		if node != nil {
			total++
		}
	}
	return total
}
