// Copyright (C) 2018 Storj Labs, Inc.
// See LICENSE for copying information.

package ecclient

import (
	"context"
	"io"
	"time"

	"go.uber.org/zap"
	monkit "gopkg.in/spacemonkeygo/monkit.v2"

	"storj.io/storj/pkg/eestream"
	"storj.io/storj/pkg/piecestore/rpc/client"
	"storj.io/storj/pkg/ranger"
	proto "storj.io/storj/protos/overlay"
)

var mon = monkit.Package()

// ECClient defines an interface for storing erasure coded data to piece store nodes
type ECClient interface {
	Put(ctx context.Context, nodes []proto.Node, rs eestream.RedundancyStrategy,
		pieceID client.PieceID, data io.Reader, expiration time.Time) error
	Get(ctx context.Context, nodes []proto.Node, es eestream.ErasureScheme,
		pieceID client.PieceID, size int64) (ranger.RangeCloser, error)
	Delete(ctx context.Context, nodes []proto.Node, pieceID client.PieceID) error
}

type dialer interface {
	dial(ctx context.Context, node proto.Node) (ps client.PSClient, err error)
}

type defaultDialer struct {
	t TransportClient
}

func (d *defaultDialer) dial(ctx context.Context, node proto.Node) (ps client.PSClient, err error) {
	defer mon.Task()(&ctx)(&err)
	c, err := d.t.DialNode(ctx, node)
	if err != nil {
		return nil, err
	}
	return client.NewPSClient(c), nil
}

type ecClient struct {
	d   dialer
	mbm int
}

// NewECClient from the given TransportClient and max buffer memory
func NewECClient(t TransportClient, mbm int) ECClient {
	return &ecClient{d: &defaultDialer{t: t}, mbm: mbm}
}

func (ec *ecClient) Put(ctx context.Context, nodes []proto.Node, rs eestream.RedundancyStrategy,
	pieceID client.PieceID, data io.Reader, expiration time.Time) (err error) {
	defer mon.Task()(&ctx)(&err)
	if len(nodes) != rs.TotalCount() {
		return Error.New("number of nodes do not match total count of erasure scheme")
	}
	readers, err := eestream.EncodeReader(ctx, data, rs, ec.mbm)
	if err != nil {
		return err
	}
	errs := make(chan error, len(readers))
	for i, n := range nodes {
		go func(i int, n proto.Node) {
			ps, err := ec.d.dial(ctx, n)
			if err != nil {
				errs <- err
				return
			}
			err = ps.Put(ctx, pieceID, readers[i], expiration)
			ps.CloseConn()
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

func (ec *ecClient) Get(ctx context.Context, nodes []proto.Node, es eestream.ErasureScheme,
	pieceID client.PieceID, size int64) (rr ranger.RangeCloser, err error) {
	defer mon.Task()(&ctx)(&err)
	if len(nodes) != es.TotalCount() {
		return nil, Error.New("number of nodes do not match total count of erasure scheme")
	}
	rrs := map[int]ranger.RangeCloser{}
	type rangerInfo struct {
		i   int
		rr  ranger.RangeCloser
		err error
	}
	rrch := make(chan rangerInfo, len(nodes))
	for i, n := range nodes {
		go func(i int, n proto.Node) {
			ps, err := ec.d.dial(ctx, n)
			if err != nil {
				rrch <- rangerInfo{i: i, rr: nil, err: err}
				return
			}
			rr, err := ps.Get(ctx, pieceID, size)
			// no ps.CloseConn() here, the connection will be closed by
			// the caller using RangeCloser.Close
			rrch <- rangerInfo{i: i, rr: rr, err: err}
		}(i, n)
	}
	for range nodes {
		rri := <-rrch
		if rri.err != nil {
			zap.S().Error(rri.err)
			continue
		}
		rrs[rri.i] = rri.rr
	}
	return eestream.Decode(rrs, es, ec.mbm)
}

func (ec *ecClient) Delete(ctx context.Context, nodes []proto.Node, pieceID client.PieceID) (err error) {
	defer mon.Task()(&ctx)(&err)
	errs := make(chan error, len(nodes))
	for _, n := range nodes {
		go func(n proto.Node) {
			ps, err := ec.d.dial(ctx, n)
			if err != nil {
				errs <- err
				return
			}
			err = ps.Delete(ctx, pieceID)
			ps.CloseConn()
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
			zap.S().Error(err)
			result = append(result, err)
		}
	}
	return result
}
