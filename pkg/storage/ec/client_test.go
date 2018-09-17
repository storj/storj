// Copyright (C) 2018 Storj Labs, Inc.
// See LICENSE for copying information.

package ecclient

import (
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"errors"
	"fmt"
	"io"
	"testing"
	"time"

	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/assert"
	"github.com/vivint/infectious"

	"storj.io/storj/pkg/eestream"
	"storj.io/storj/pkg/pb"
	"storj.io/storj/pkg/piecestore/rpc/client"
	"storj.io/storj/pkg/provider"
	"storj.io/storj/pkg/ranger"
)

const (
	dialFailed = "dial failed"
	opFailed   = "op failed"
)

var (
	ErrDialFailed = errors.New(dialFailed)
	ErrOpFailed   = errors.New(opFailed)
)

var (
	node0 = &pb.Node{Id: "node-0"}
	node1 = &pb.Node{Id: "node-1"}
	node2 = &pb.Node{Id: "node-2"}
	node3 = &pb.Node{Id: "node-3"}
)

type mockDialer struct {
	m map[*pb.Node]client.PSClient
}

func (d *mockDialer) dial(ctx context.Context, node *pb.Node) (
	ps client.PSClient, err error) {
	ps = d.m[node]
	if ps == nil {
		return nil, ErrDialFailed
	}
	return d.m[node], nil
}

func TestNewECClient(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	tc := NewMockClient(ctrl)
	mbm := 1234

	privKey, _ := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	identity := &provider.FullIdentity{Key: privKey}
	ec := NewClient(identity, tc, mbm)
	assert.NotNil(t, ec)

	ecc, ok := ec.(*ecClient)
	assert.True(t, ok)
	assert.NotNil(t, ecc.d)
	assert.Equal(t, mbm, ecc.mbm)

	dd, ok := ecc.d.(*defaultDialer)
	assert.True(t, ok)
	assert.NotNil(t, dd.t)
	assert.Equal(t, dd.t, tc)
}

func TestDefaultDialer(t *testing.T) {
	ctx := context.Background()
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	privKey, _ := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	identity := &provider.FullIdentity{Key: privKey}

	for i, tt := range []struct {
		err       error
		errString string
	}{
		{nil, ""},
		{ErrDialFailed, dialFailed},
	} {
		errTag := fmt.Sprintf("Test case #%d", i)

		tc := NewMockClient(ctrl)
		tc.EXPECT().DialNode(gomock.Any(), node0).Return(nil, tt.err)

		dd := defaultDialer{t: tc, identity: identity}
		_, err := dd.dial(ctx, node0)

		if tt.errString != "" {
			assert.EqualError(t, err, tt.errString, errTag)
		} else {
			assert.NoError(t, err, errTag)
		}
	}
}

func TestPut(t *testing.T) {
	ctx := context.Background()
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	size := 32 * 1024
	k := 2
	n := 4
	fc, err := infectious.NewFEC(k, n)
	if !assert.NoError(t, err) {
		return
	}
	es := eestream.NewRSScheme(fc, size/n)

TestLoop:
	for i, tt := range []struct {
		nodes     []*pb.Node
		min       int
		mbm       int
		badInput  bool
		errs      []error
		errString string
	}{
		{[]*pb.Node{}, 0, 0, true, []error{},
			fmt.Sprintf("ecclient error: number of nodes (0) do not match total count (%v) of erasure scheme", n)},
		{[]*pb.Node{node0, node1, node2, node3}, 0, -1, true,
			[]error{nil, nil, nil, nil},
			"eestream error: negative max buffer memory"},
		{[]*pb.Node{node0, node1, node0, node3}, 0, 0, true,
			[]error{nil, nil, nil, nil},
			"ecclient error: duplicated nodes are not allowed"},
		{[]*pb.Node{node0, node1, node2, node3}, 0, 0, false,
			[]error{nil, nil, nil, nil}, ""},
		{[]*pb.Node{node0, node1, node2, node3}, 0, 0, false,
			[]error{nil, ErrDialFailed, nil, nil},
			"ecclient error: successful puts (3) less than minimum threshold (4)"},
		{[]*pb.Node{node0, node1, node2, node3}, 0, 0, false,
			[]error{nil, ErrOpFailed, nil, nil},
			"ecclient error: successful puts (3) less than minimum threshold (4)"},
		{[]*pb.Node{node0, node1, node2, node3}, 2, 0, false,
			[]error{nil, ErrDialFailed, nil, nil}, ""},
		{[]*pb.Node{node0, node1, node2, node3}, 2, 0, false,
			[]error{ErrOpFailed, ErrDialFailed, nil, ErrDialFailed},
			"ecclient error: successful puts (1) less than minimum threshold (2)"},
	} {
		errTag := fmt.Sprintf("Test case #%d", i)

		id := client.NewPieceID()
		ttl := time.Now()

		errs := make(map[*pb.Node]error, len(tt.nodes))
		for i, n := range tt.nodes {
			errs[n] = tt.errs[i]
		}

		m := make(map[*pb.Node]client.PSClient, len(tt.nodes))
		for _, n := range tt.nodes {
			if !tt.badInput {
				derivedID, err := id.Derive([]byte(n.GetId()))
				if !assert.NoError(t, err, errTag) {
					continue TestLoop
				}
				ps := NewMockPSClient(ctrl)
				gomock.InOrder(
					ps.EXPECT().Put(gomock.Any(), derivedID, gomock.Any(), ttl, gomock.Any()).Return(errs[n]),
					ps.EXPECT().Close().Return(nil),
				)
				m[n] = ps
			}
		}
		rs, err := eestream.NewRedundancyStrategy(es, tt.min, 0)
		if !assert.NoError(t, err, errTag) {
			continue
		}
		r := io.LimitReader(rand.Reader, int64(size))
		ec := ecClient{d: &mockDialer{m: m}, mbm: tt.mbm}
		err = ec.Put(ctx, tt.nodes, rs, id, r, ttl)

		if tt.errString != "" {
			assert.EqualError(t, err, tt.errString, errTag)
		} else {
			assert.NoError(t, err, errTag)
		}
	}
}

func TestGet(t *testing.T) {
	ctx := context.Background()
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	size := 32 * 1024
	k := 2
	n := 4
	fc, err := infectious.NewFEC(k, n)
	if !assert.NoError(t, err) {
		return
	}
	es := eestream.NewRSScheme(fc, size/n)

TestLoop:
	for i, tt := range []struct {
		nodes     []*pb.Node
		mbm       int
		errs      []error
		errString string
	}{
		{[]*pb.Node{}, 0, []error{}, "ecclient error: " +
			fmt.Sprintf("number of nodes (0) do not match total count (%v) of erasure scheme", n)},
		{[]*pb.Node{node0, node1, node2, node3}, -1,
			[]error{nil, nil, nil, nil},
			"eestream error: negative max buffer memory"},
		{[]*pb.Node{node0, node1, node2, node3}, 0,
			[]error{nil, nil, nil, nil}, ""},
		{[]*pb.Node{node0, node1, node2, node3}, 0,
			[]error{nil, ErrDialFailed, nil, nil}, ""},
		{[]*pb.Node{node0, node1, node2, node3}, 0,
			[]error{nil, ErrOpFailed, nil, nil}, ""},
		{[]*pb.Node{node0, node1, node2, node3}, 0,
			[]error{ErrOpFailed, ErrDialFailed, nil, ErrDialFailed}, ""},
		{[]*pb.Node{node0, node1, node2, node3}, 0,
			[]error{ErrDialFailed, ErrOpFailed, ErrOpFailed, ErrDialFailed}, ""},
	} {
		errTag := fmt.Sprintf("Test case #%d", i)

		id := client.NewPieceID()

		errs := make(map[*pb.Node]error, len(tt.nodes))
		for i, n := range tt.nodes {
			errs[n] = tt.errs[i]
		}

		m := make(map[*pb.Node]client.PSClient, len(tt.nodes))
		for _, n := range tt.nodes {
			if errs[n] == ErrOpFailed {
				derivedID, err := id.Derive([]byte(n.GetId()))
				if !assert.NoError(t, err, errTag) {
					continue TestLoop
				}
				ps := NewMockPSClient(ctrl)
				ps.EXPECT().Get(gomock.Any(), derivedID, int64(size/k), gomock.Any()).Return(ranger.ByteRanger(nil), errs[n])
				m[n] = ps
			}
		}
		ec := ecClient{d: &mockDialer{m: m}, mbm: tt.mbm}
		rr, err := ec.Get(ctx, tt.nodes, es, id, int64(size))
		if err == nil {
			_, err := rr.Range(ctx, 0, 0)
			assert.NoError(t, err, errTag)
		}
		if tt.errString != "" {
			assert.EqualError(t, err, tt.errString, errTag)
		} else {
			assert.NoError(t, err, errTag)
			assert.NotNil(t, rr, errTag)
		}
	}
}

func TestDelete(t *testing.T) {
	ctx := context.Background()
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

TestLoop:
	for i, tt := range []struct {
		nodes     []*pb.Node
		errs      []error
		errString string
	}{
		{[]*pb.Node{}, []error{}, ""},
		{[]*pb.Node{node0}, []error{nil}, ""},
		{[]*pb.Node{node0}, []error{ErrDialFailed}, dialFailed},
		{[]*pb.Node{node0}, []error{ErrOpFailed}, opFailed},
		{[]*pb.Node{node0, node1}, []error{nil, nil}, ""},
		{[]*pb.Node{node0, node1}, []error{ErrDialFailed, nil}, ""},
		{[]*pb.Node{node0, node1}, []error{nil, ErrOpFailed}, ""},
		{[]*pb.Node{node0, node1}, []error{ErrDialFailed, ErrDialFailed}, dialFailed},
		{[]*pb.Node{node0, node1}, []error{ErrOpFailed, ErrOpFailed}, opFailed},
	} {
		errTag := fmt.Sprintf("Test case #%d", i)

		id := client.NewPieceID()

		errs := make(map[*pb.Node]error, len(tt.nodes))
		for i, n := range tt.nodes {
			errs[n] = tt.errs[i]
		}

		m := make(map[*pb.Node]client.PSClient, len(tt.nodes))
		for _, n := range tt.nodes {
			if errs[n] != ErrDialFailed {
				derivedID, err := id.Derive([]byte(n.GetId()))
				if !assert.NoError(t, err, errTag) {
					continue TestLoop
				}
				ps := NewMockPSClient(ctrl)
				gomock.InOrder(
					ps.EXPECT().Delete(gomock.Any(), derivedID).Return(errs[n]),
					ps.EXPECT().Close().Return(nil),
				)
				m[n] = ps
			}
		}

		ec := ecClient{d: &mockDialer{m: m}}
		err := ec.Delete(ctx, tt.nodes, id)

		if tt.errString != "" {
			assert.EqualError(t, err, tt.errString, errTag)
		} else {
			assert.NoError(t, err, errTag)
		}
	}
}

func TestUnique(t *testing.T) {
	for i, tt := range []struct {
		nodes  []*pb.Node
		unique bool
	}{
		{nil, true},
		{[]*pb.Node{}, true},
		{[]*pb.Node{node0}, true},
		{[]*pb.Node{node0, node1}, true},
		{[]*pb.Node{node0, node0}, false},
		{[]*pb.Node{node0, node1, node0}, false},
		{[]*pb.Node{node1, node0, node0}, false},
		{[]*pb.Node{node0, node0, node1}, false},
		{[]*pb.Node{node2, node0, node1}, true},
		{[]*pb.Node{node2, node0, node3, node1}, true},
		{[]*pb.Node{node2, node0, node2, node1}, false},
		{[]*pb.Node{node1, node0, node3, node1}, false},
	} {
		errTag := fmt.Sprintf("Test case #%d", i)
		assert.Equal(t, tt.unique, unique(tt.nodes), errTag)
	}
}
