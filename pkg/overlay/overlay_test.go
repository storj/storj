// Copyright (C) 2018 Storj Labs, Inc.
// See LICENSE for copying information.

package overlay

import (
	"context"
	"net"
	"testing"

	"github.com/gogo/protobuf/proto"
	"github.com/stretchr/testify/assert"
	"google.golang.org/grpc"

	"storj.io/storj/pkg/node"
	"storj.io/storj/pkg/pb"
	"storj.io/storj/storage"
)

func TestFindStorageNodes(t *testing.T) {
	lis, err := net.Listen("tcp", "127.0.0.1:0")
	assert.NoError(t, err)

	minRep := &pb.NodeRep{
		UptimeRatio:       0.95,
		AuditSuccessRatio: 0.95,
		AuditCount:        10,
	}
	restrictions := &pb.NodeRestrictions{
		FreeDisk: 10,
	}

	mockServerNodeList := []storage.ListItem{}
	goodNodeIds := [][]byte{}

	for _, tt := range []struct {
		addr string
		//freeBandwidth int64
		freeDisk        int64
		totalAuditCount int64
		auditRatio      float64
		uptimeRatio     float64
	}{
		{"127.0.0.1:9090", 10, 20, 1, 1},     // good stats, enough space
		{"127.0.0.1:9090", 10, 30, 1, 1},     // good stats, enough space, duplicate IP
		{"127.0.0.2:9090", 30, 30, 0.6, 0.5}, // bad stats, enough space
		{"127.0.0.4:9090", 5, 30, 1, 1},      // good stats, not enough space
		{"127.0.0.5:9090", 20, 30, 1, 1},     // good stats, enough space
	} {
		fid, err := node.NewFullIdentity(ctx, 12, 4)
		assert.NoError(t, err)

		nodeRes := &pb.NodeRestrictions{
			FreeDisk: tt.freeDisk,
		}
		nodeRep := &pb.NodeRep{
			AuditSuccessRatio: tt.auditRatio,
			UptimeRatio:       tt.uptimeRatio,
			AuditCount:        tt.totalAuditCount,
		}

		mockServerNodeList = append(mockServerNodeList, storage.ListItem{
			Key:   storage.Key(fid.ID.String()),
			Value: newNodeStorageValue(t, fid.ID.String(), tt.addr, nodeRes, nodeRep),
		})

		if tt.freeDisk >= restrictions.FreeDisk &&
			tt.totalAuditCount >= minRep.AuditCount &&
			tt.auditRatio >= minRep.AuditSuccessRatio &&
			tt.uptimeRatio >= minRep.UptimeRatio {
			goodNodeIds = append(goodNodeIds, fid.ID.Bytes())
		}
	}

	srv := NewMockServer(mockServerNodeList)
	assert.NotNil(t, srv)

	go func() { assert.NoError(t, srv.Serve(lis)) }()
	defer srv.Stop()

	address := lis.Addr().String()
	c, err := NewTestClient(address)
	assert.NoError(t, err)

	r, err := c.FindStorageNodes(ctx,
		&pb.FindStorageNodesRequest{
			Opts: &pb.OverlayOptions{
				Amount:        2,
				Restrictions:  restrictions,
				MinReputation: minRep,
			},
		},
	)

	assert.NoError(t, err)
	assert.NotNil(t, r)
	assert.Len(t, r.Nodes, 2)

	addrs := make(map[string]bool)
	for _, n := range r.Nodes {
		nodeAddr := n.Address.GetAddress()
		assert.EqualValues(t, addrs[nodeAddr], false)
		addrs[nodeAddr] = true

		nid := node.ID(n.Id)
		assert.Contains(t, goodNodeIds, nid.Bytes())
	}
}

func TestOverlayLookup(t *testing.T) {
	lis, err := net.Listen("tcp", "127.0.0.1:0")
	assert.NoError(t, err)

	fid, err := node.NewFullIdentity(ctx, 12, 4)

	assert.NoError(t, err)

	srv := NewMockServer([]storage.ListItem{
		{
			Key:   storage.Key(fid.ID.String()),
			Value: newNodeStorageValue(t, fid.ID.String(), "127.0.0.1:9090", nil, nil),
		},
	})
	go func() { assert.NoError(t, srv.Serve(lis)) }()
	defer srv.Stop()

	address := lis.Addr().String()
	c, err := NewTestClient(address)
	assert.NoError(t, err)

	r, err := c.Lookup(context.Background(), &pb.LookupRequest{NodeID: fid.ID.String()})
	assert.NoError(t, err)
	assert.NotNil(t, r)
}

func TestOverlayBulkLookup(t *testing.T) {
	lis, err := net.Listen("tcp", "127.0.0.1:0")
	assert.NoError(t, err)

	fid, err := node.NewFullIdentity(ctx, 12, 4)
	assert.NoError(t, err)
	fid2, err := node.NewFullIdentity(ctx, 12, 4)
	assert.NoError(t, err)

	srv := NewMockServer([]storage.ListItem{
		{
			Key:   storage.Key(fid.ID.String()),
			Value: newNodeStorageValue(t, fid.ID.String(), "127.0.0.1:9090", nil, nil),
		},
	})
	go func() { assert.NoError(t, srv.Serve(lis)) }()
	defer srv.Stop()

	address := lis.Addr().String()
	c, err := NewTestClient(address)
	assert.NoError(t, err)

	req1 := &pb.LookupRequest{NodeID: fid.ID.String()}
	req2 := &pb.LookupRequest{NodeID: fid2.ID.String()}
	rs := &pb.LookupRequests{Lookuprequest: []*pb.LookupRequest{req1, req2}}
	r, err := c.BulkLookup(context.Background(), rs)
	assert.NoError(t, err)
	assert.NotNil(t, r)
}

// newNodeStorageValue provides a convient way to create a node as a storage.Value for testing purposes
func newNodeStorageValue(t *testing.T, id, address string, restrictions *pb.NodeRestrictions, reputation *pb.NodeRep) storage.Value {
	na := &pb.Node{
		Id: id,
		Address: &pb.NodeAddress{
			Transport: pb.NodeTransport_TCP_TLS_GRPC,
			Address:   address,
		},
		Reputation:   reputation,
		Restrictions: restrictions,
	}
	d, err := proto.Marshal(na)
	assert.NoError(t, err)
	return d
}

func NewTestClient(address string) (pb.OverlayClient, error) {
	conn, err := grpc.Dial(address, grpc.WithInsecure())
	if err != nil {
		return nil, err
	}
	return pb.NewOverlayClient(conn), nil
}
