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
	monkit "gopkg.in/spacemonkeygo/monkit.v2"

	"storj.io/storj/pkg/eestream"
	"storj.io/storj/pkg/piecestore/rpc/client"
	"storj.io/storj/pkg/ranger"
	"storj.io/storj/pkg/transport"
	"storj.io/storj/pkg/utils"
	proto "storj.io/storj/protos/overlay"
	pb "storj.io/storj/protos/piecestore"
)

var mon = monkit.Package()

// Client defines an interface for storing erasure coded data to piece store nodes
type Client interface {
	Put(ctx context.Context, nodes []*proto.Node, rs eestream.RedundancyStrategy,
		pieceID client.PieceID, data io.Reader, expiration time.Time) error
	Get(ctx context.Context, nodes []*proto.Node, es eestream.ErasureScheme,
		pieceID client.PieceID, size int64) (ranger.RangeCloser, error)
	Delete(ctx context.Context, nodes []*proto.Node, pieceID client.PieceID) error
}

type dialer interface {
	dial(ctx context.Context, node *proto.Node) (ps client.PSClient, err error)
}

type defaultDialer struct {
	t transport.Client
}

func (d *defaultDialer) dial(ctx context.Context, node *proto.Node) (ps client.PSClient, err error) {
	defer mon.Task()(&ctx)(&err)
	c, err := d.t.DialNode(ctx, node)
	if err != nil {
		return nil, err
	}

	return client.NewPSClient(c, 0)
}

type ecClient struct {
	d   dialer
	mbm int
}

// NewClient from the given TransportClient and max buffer memory
func NewClient(t transport.Client, mbm int) Client {
	d := defaultDialer{t: t}
	return &ecClient{d: &d, mbm: mbm}
}

func (ec *ecClient) Put(ctx context.Context, nodes []*proto.Node, rs eestream.RedundancyStrategy,
	pieceID client.PieceID, data io.Reader, expiration time.Time) (err error) {
	defer mon.Task()(&ctx)(&err)
	if len(nodes) != rs.TotalCount() {
		return Error.New("number of nodes (%d) do not match total count (%d) of erasure scheme",
			len(nodes), rs.TotalCount())
	}
	if !unique(nodes) {
		return Error.New("duplicated nodes are not allowed")
	}
	padded := eestream.PadReader(ioutil.NopCloser(data), rs.DecodedBlockSize())
	readers, err := eestream.EncodeReader(ctx, padded, rs, ec.mbm)
	if err != nil {
		return err
	}
	errs := make(chan error, len(readers))
	for i, n := range nodes {
		go func(i int, n *proto.Node) {
			derivedPieceID, err := pieceID.Derive([]byte(n.GetId()))
			if err != nil {
				zap.S().Errorf("Failed deriving piece id for %s: %v", pieceID, err)
				errs <- err
				return
			}
			ps, err := ec.d.dial(ctx, n)
			if err != nil {
				zap.S().Errorf("Failed putting piece %s -> %s to node %s: %v",
					pieceID, derivedPieceID, n.GetId(), err)
				errs <- err
				return
			}
			err = ps.Put(ctx, derivedPieceID, readers[i], expiration, &pb.PayerBandwidthAllocation{})
			// normally the bellow call should be deferred, but doing so fails
			// randomly the unit tests
			utils.LogClose(ps)
			if err != nil {
				zap.S().Errorf("Failed putting piece %s -> %s to node %s: %v",
					pieceID, derivedPieceID, n.GetId(), err)
			}
			errs <- err
		}(i, n)
	}
	allerrs := collectErrors(errs, len(readers))
	sc := len(readers) - len(allerrs)
	if sc < rs.MinimumThreshold() {
		return Error.New(
			"successful puts (%d) less than minimum threshold (%d)",
			sc, rs.MinimumThreshold())
	}
	return nil
}

func (ec *ecClient) Get(ctx context.Context, nodes []*proto.Node, es eestream.ErasureScheme,
	pieceID client.PieceID, size int64) (rr ranger.RangeCloser, err error) {
	defer mon.Task()(&ctx)(&err)
	if len(nodes) != es.TotalCount() {
		return nil, Error.New("number of nodes do not match total count of erasure scheme")
	}
	paddedSize := calcPadded(size, es.DecodedBlockSize())
	pieceSize := paddedSize / int64(es.RequiredCount())
	rrs := map[int]ranger.RangeCloser{}
	type rangerInfo struct {
		i   int
		rr  ranger.RangeCloser
		err error
	}
	ch := make(chan rangerInfo, len(nodes))
	for i, n := range nodes {
		go func(i int, n *proto.Node) {
			derivedPieceID, err := pieceID.Derive([]byte(n.GetId()))
			if err != nil {
				zap.S().Errorf("Failed deriving piece id for %s: %v", pieceID, err)
				ch <- rangerInfo{i: i, rr: nil, err: err}
				return
			}
			ps, err := ec.d.dial(ctx, n)
			if err != nil {
				zap.S().Errorf("Failed getting piece %s -> %s from node %s: %v",
					pieceID, derivedPieceID, n.GetId(), err)
				ch <- rangerInfo{i: i, rr: nil, err: err}
				return
			}
			rr, err := ps.Get(ctx, derivedPieceID, pieceSize, &pb.PayerBandwidthAllocation{})
			// no ps.CloseConn() here, the connection will be closed by
			// the caller using RangeCloser.Close
			if err != nil {
				zap.S().Errorf("Failed getting piece %s -> %s from node %s: %v",
					pieceID, derivedPieceID, n.GetId(), err)
			}
			ch <- rangerInfo{i: i, rr: rr, err: err}
		}(i, n)
	}
	for range nodes {
		rri := <-ch
		if rri.err == nil {
			rrs[rri.i] = rri.rr
		}
	}
	rr, err = eestream.Decode(rrs, es, ec.mbm)
	if err != nil {
		return nil, err
	}
	return eestream.Unpad(rr, int(paddedSize-size))
}

func (ec *ecClient) Delete(ctx context.Context, nodes []*proto.Node, pieceID client.PieceID) (err error) {
	defer mon.Task()(&ctx)(&err)
	errs := make(chan error, len(nodes))
	for _, n := range nodes {
		go func(n *proto.Node) {
			derivedPieceID, err := pieceID.Derive([]byte(n.GetId()))
			if err != nil {
				zap.S().Errorf("Failed deriving piece id for %s: %v", pieceID, err)
				errs <- err
				return
			}
			ps, err := ec.d.dial(ctx, n)
			if err != nil {
				zap.S().Errorf("Failed deleting piece %s -> %s from node %s: %v",
					pieceID, derivedPieceID, n.GetId(), err)
				errs <- err
				return
			}
			err = ps.Delete(ctx, derivedPieceID)
			// normally the bellow call should be deferred, but doing so fails
			// randomly the unit tests
			utils.LogClose(ps)
			if err != nil {
				zap.S().Errorf("Failed deleting piece %s -> %s from node %s: %v",
					pieceID, derivedPieceID, n.GetId(), err)
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

func unique(nodes []*proto.Node) bool {
	if len(nodes) < 2 {
		return true
	}

	ids := make([]string, len(nodes))
	for i, n := range nodes {
		ids[i] = n.GetId()
	}

	// sort the ids and check for identical neighbors
	sort.Strings(ids)
	for i := 1; i < len(ids); i++ {
		if ids[i] == ids[i-1] {
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
