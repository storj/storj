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
	"storj.io/storj/pkg/ranger"
	proto "storj.io/storj/protos/overlay"
)

var (
	mon = monkit.Package()
)

// ECClient defines an interface for storing erasure coded data to piece store nodes
type ECClient interface {
	Put(ctx context.Context, nodes []proto.Node, rs eestream.RedundancyStrategy,
		pieceID PieceID, data io.Reader, expiration time.Time) error
	Get(ctx context.Context, nodes []proto.Node, es eestream.ErasureScheme,
		pieceID PieceID, size int64) (ranger.RangeCloser, error)
	Delete(ctx context.Context, nodes []proto.Node, pieceID PieceID) error
}

type ecClient struct {
	t   TransportClient
	mbm int
}

// NewECClient from the given TransportClient and max buffer memory
func NewECClient(t TransportClient, mbm int) ECClient {
	return &ecClient{t: t, mbm: mbm}
}

func (ec *ecClient) Put(ctx context.Context, nodes []proto.Node, rs eestream.RedundancyStrategy,
	pieceID PieceID, data io.Reader, expiration time.Time) (err error) {
	defer mon.Task()(&ctx)(&err)
	readers, err := eestream.EncodeReader(ctx, data, rs, ec.mbm)
	if err != nil {
		return err
	}
	errs := make(chan error, len(readers))
	for i, n := range nodes {
		go func(i int, n proto.Node) {
			c, err := ec.t.DialNode(ctx, &n)
			if err != nil {
				errs <- err
				return
			}
			defer c.Close()
			ps := NewPSClient(c)
			errs <- ps.Put(ctx, pieceID, readers[i], expiration)
		}(i, n)
	}
	var errbucket []error
	for range readers {
		err := <-errs
		if err != nil {
			errbucket = append(errbucket, err)
			// TODO log error?
		}
	}
	if len(readers)-len(errbucket) < rs.MinimumThreshold() {
		// TODO return error
	}
	return nil
}

func (ec *ecClient) Get(ctx context.Context, nodes []proto.Node, es eestream.ErasureScheme,
	pieceID PieceID, size int64) (rr ranger.RangeCloser, err error) {
	defer mon.Task()(&ctx)(&err)
	rrs := map[int]ranger.RangeCloser{}
	type rangerInfo struct {
		i   int
		rr  ranger.RangeCloser
		err error
	}
	rrch := make(chan rangerInfo, len(nodes))
	for i, n := range nodes {
		go func(i int, n proto.Node) {
			c, err := ec.t.DialNode(ctx, &n)
			if err != nil {
				rrch <- rangerInfo{i: i, rr: nil, err: err}
				return
			}
			// no defer c.Close() here, the connection will be closed by the
			// caller using RangeCloser.Close
			ps := NewPSClient(c)
			rr, err := ps.Get(ctx, pieceID, size)
			rrch <- rangerInfo{i: i, rr: rr, err: err}
		}(i, n)
	}
	for range nodes {
		rri := <-rrch
		if rri.err != nil {
			// TODO better error for the failed node
			zap.S().Error(rri.err)
			continue
		}
		rrs[rri.i] = rri.rr
	}
	return eestream.Decode(rrs, es, ec.mbm)
}

func (ec *ecClient) Delete(ctx context.Context, nodes []proto.Node, pieceID PieceID) (err error) {
	defer mon.Task()(&ctx)(&err)
	errs := make(chan error, len(nodes))
	for _, n := range nodes {
		go func(n proto.Node) {
			c, err := ec.t.DialNode(ctx, &n)
			if err != nil {
				errs <- err
				return
			}
			defer c.Close()
			ps := NewPSClient(c)
			if err != nil {
				errs <- err
				return
			}
			errs <- ps.Delete(ctx, pieceID)
		}(n)
	}
	for range nodes {
		err := <-errs
		if err != nil {
			// TODO should we return an error with the list of all failed nodes?
			return err
		}
	}
	return nil
}
