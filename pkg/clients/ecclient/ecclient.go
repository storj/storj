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

var mon = monkit.Package()

// ECClient defines an interface for storing erasure coded data to piece store nodes
type ECClient interface {
	Put(ctx context.Context, nodes []proto.Node, rs eestream.RedundancyStrategy,
		pieceID PieceID, data io.Reader, expiration time.Time) error
	Get(ctx context.Context, nodes []proto.Node, es eestream.ErasureScheme,
		pieceID PieceID, size int64) (ranger.RangeCloser, error)
	Delete(ctx context.Context, nodes []proto.Node, pieceID PieceID) error
}

type dialer interface {
	dial(ctx context.Context, node proto.Node) (ps PSClient, err error)
}

type defaultDialer struct {
	t TransportClient
}

func (d *defaultDialer) dial(ctx context.Context, node proto.Node) (ps PSClient, err error) {
	defer mon.Task()(&ctx)(&err)
	c, err := d.t.DialNode(ctx, node)
	if err != nil {
		return nil, err
	}
	return NewPSClient(c), nil
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
	pieceID PieceID, data io.Reader, expiration time.Time) (err error) {
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
	var errbucket []error
	for range readers {
		err := <-errs
		if err != nil {
			errbucket = append(errbucket, err)
			// TODO log error?
		}
	}
	sc := len(readers) - len(errbucket)
	if sc < rs.MinimumThreshold() {
		return Error.New(
			"successful puts (%d) less than minimum threshold (%d)",
			sc, rs.MinimumThreshold())
	}
	return nil
}

func (ec *ecClient) Get(ctx context.Context, nodes []proto.Node, es eestream.ErasureScheme,
	pieceID PieceID, size int64) (rr ranger.RangeCloser, err error) {
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
			// no defer ps.CloseConn() here, the connection will be closed by
			// the caller using RangeCloser.Close
			rr, err := ps.Get(ctx, pieceID, size)
			rrch <- rangerInfo{i: i, rr: rr, err: err}
		}(i, n)
	}
	var first error
	for range nodes {
		rri := <-rrch
		if rri.err != nil {
			// TODO better error for the failed node
			zap.S().Error(rri.err)
			first = rri.err
			continue
		}
		rrs[rri.i] = rri.rr
	}
	if len(rrs) == 0 {
		return nil, Error.New("could not get from any node: %v", first)
	}
	return eestream.Decode(rrs, es, ec.mbm)
}

func (ec *ecClient) Delete(ctx context.Context, nodes []proto.Node, pieceID PieceID) (err error) {
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
	var errbucket []error
	for range nodes {
		err := <-errs
		if err != nil {
			errbucket = append(errbucket, err)
		}
	}
	if len(errbucket) > 0 && len(errbucket) == len(nodes) {
		return errbucket[0]
	}
	return nil
}
