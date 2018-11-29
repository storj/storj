// Copyright (C) 2018 Storj Labs, Inc.
// See LICENSE for copying information.

package overlay_test

import (
	"testing"

	"github.com/gogo/protobuf/proto"
	"github.com/stretchr/testify/assert"

	"storj.io/storj/internal/identity"
	"storj.io/storj/internal/testcontext"
	"storj.io/storj/internal/testplanet"
	"storj.io/storj/internal/teststorj"
	"storj.io/storj/pkg/overlay"
	"storj.io/storj/pkg/pb"
	"storj.io/storj/pkg/storj"
	"storj.io/storj/storage"
)

func TestNewOverlayClient(t *testing.T) {
	ctx := testcontext.New(t)
	defer ctx.Cleanup()

	cases := []struct {
		address string
	}{
		{
			address: "127.0.0.1:8080",
		},
	}

	for _, v := range cases {
		ca, err := testidentity.NewTestCA(ctx)
		assert.NoError(t, err)
		identity, err := ca.NewIdentity()
		assert.NoError(t, err)

		oc, err := overlay.NewOverlayClient(identity, v.address)
		assert.NoError(t, err)

		assert.NotNil(t, oc)
		_, ok := oc.(*overlay.Overlay)
		assert.True(t, ok)
	}
}

func TestChoose(t *testing.T) {
	ctx := testcontext.New(t)
	defer ctx.Cleanup()

	planet, err := testplanet.New(t, 1, 4, 0)
	if err != nil {
		t.Fatal(err)
	}
	defer ctx.Check(planet.Shutdown)

	overlayAddr := planet.Satellites[0].Addr()

	cases := []struct {
		limit        int
		space        int64
		bandwidth    int64
		uptime       float64
		uptimeCount  int64
		auditSuccess float64
		auditCount   int64
		allNodes     []*pb.Node
		excluded     storj.NodeIDList
	}{
		{
			limit:        4,
			space:        0,
			bandwidth:    0,
			uptime:       1,
			uptimeCount:  10,
			auditSuccess: 1,
			auditCount:   10,
			allNodes: func() []*pb.Node {
				n1 := teststorj.MockNode("n1")
				n2 := teststorj.MockNode("n2")
				n3 := teststorj.MockNode("n3")
				n4 := teststorj.MockNode("n4")
				n5 := teststorj.MockNode("n5")
				n6 := teststorj.MockNode("n6")
				n7 := teststorj.MockNode("n7")
				n8 := teststorj.MockNode("n8")
				nodes := []*pb.Node{n1, n2, n3, n4, n5, n6, n7, n8}
				for _, n := range nodes {
					n.Type = pb.NodeType_STORAGE
				}
				return nodes
			}(),
			excluded: func() storj.NodeIDList {
				id1 := teststorj.NodeIDFromString("n1")
				id2 := teststorj.NodeIDFromString("n2")
				id3 := teststorj.NodeIDFromString("n3")
				id4 := teststorj.NodeIDFromString("n4")
				return storj.NodeIDList{id1, id2, id3, id4}
			}(),
		},
	}

	for _, v := range cases {
		var listItems []storage.ListItem
		for _, n := range v.allNodes {
			data, err := proto.Marshal(n)
			assert.NoError(t, err)
			listItems = append(listItems, storage.ListItem{
				Key:   n.Id.Bytes(),
				Value: data,
			})
		}

		ca, err := testidentity.NewTestCA(ctx)
		assert.NoError(t, err)
		identity, err := ca.NewIdentity()
		assert.NoError(t, err)

		oc, err := overlay.NewOverlayClient(identity, overlayAddr)
		assert.NoError(t, err)

		assert.NotNil(t, oc)
		_, ok := oc.(*overlay.Overlay)
		assert.True(t, ok)

		newNodes, err := oc.Choose(ctx, overlay.Options{
			Amount:       v.limit,
			Space:        v.space,
			Uptime:       v.uptime,
			UptimeCount:  v.uptimeCount,
			AuditSuccess: v.auditSuccess,
			AuditCount:   v.auditCount,
			Excluded:     v.excluded,
		})
		assert.NoError(t, err)

		excludedNodes := make(map[string]bool)
		for _, e := range v.excluded {
			excludedNodes[e.String()] = true
		}
		assert.Len(t, newNodes, v.limit)
		for _, n := range newNodes {
			assert.NotContains(t, excludedNodes, n.Id.String())
			assert.True(t, n.GetRestrictions().GetFreeDisk() >= v.space)
			assert.True(t, n.GetRestrictions().GetFreeBandwidth() >= v.bandwidth)
			assert.True(t, n.GetReputation().GetUptimeRatio() >= v.uptime)
			assert.True(t, n.GetReputation().GetUptimeCount() >= v.uptimeCount)
			assert.True(t, n.GetReputation().GetAuditSuccessRatio() >= v.auditSuccess)
			assert.True(t, n.GetReputation().GetAuditCount() >= v.auditCount)

		}
	}
}

func TestLookup(t *testing.T) {
	ctx := testcontext.New(t)
	defer ctx.Cleanup()

	planet, err := testplanet.New(t, 1, 4, 0)
	if err != nil {
		t.Fatal(err)
	}
	defer ctx.Check(planet.Shutdown)

	overlayAddr := planet.Satellites[0].Addr()
	nid1 := planet.StorageNodes[0].ID()

	cases := []struct {
		nodeID    storj.NodeID
		expectErr bool
	}{
		{
			nodeID:    nid1,
			expectErr: false,
		},
		{
			nodeID:    teststorj.NodeIDFromString("n1"),
			expectErr: true,
		},
	}

	for _, v := range cases {
		ca, err := testidentity.NewTestCA(ctx)
		assert.NoError(t, err)
		identity, err := ca.NewIdentity()
		assert.NoError(t, err)

		oc, err := overlay.NewOverlayClient(identity, overlayAddr)
		assert.NoError(t, err)

		assert.NotNil(t, oc)
		_, ok := oc.(*overlay.Overlay)
		assert.True(t, ok)

		n, err := oc.Lookup(ctx, v.nodeID)
		if v.expectErr {
			assert.Error(t, err)
		} else {
			assert.NoError(t, err)
			assert.Equal(t, n.Id.String(), v.nodeID.String())
		}
	}

}

func TestBulkLookup(t *testing.T) {
	ctx := testcontext.New(t)
	defer ctx.Cleanup()

	planet, err := testplanet.New(t, 1, 4, 0)
	if err != nil {
		t.Fatal(err)
	}
	defer ctx.Check(planet.Shutdown)

	overlayAddr := planet.Satellites[0].Addr()
	nid1 := planet.StorageNodes[0].ID()
	nid2 := planet.StorageNodes[1].ID()
	nid3 := planet.StorageNodes[2].ID()

	cases := []struct {
		nodeIDs       storj.NodeIDList
		expectedCalls int
	}{
		{
			nodeIDs:       storj.NodeIDList{nid1, nid2, nid3},
			expectedCalls: 1,
		},
	}
	for _, v := range cases {
		ca, err := testidentity.NewTestCA(ctx)
		assert.NoError(t, err)
		identity, err := ca.NewIdentity()
		assert.NoError(t, err)

		oc, err := overlay.NewOverlayClient(identity, overlayAddr)
		assert.NoError(t, err)

		assert.NotNil(t, oc)
		_, ok := oc.(*overlay.Overlay)
		assert.True(t, ok)

		resNodes, err := oc.BulkLookup(ctx, v.nodeIDs)
		assert.NoError(t, err)

		nodesFound := make(map[string]bool)
		for _, n := range resNodes {
			nodesFound[n.Id.String()] = true
		}
		for _, nid := range v.nodeIDs {
			assert.Contains(t, nodesFound, nid.String())
		}
		assert.Equal(t, len(resNodes), len(v.nodeIDs))
	}
}

func TestBulkLookupV2(t *testing.T) {
	ctx := testcontext.New(t)
	defer ctx.Cleanup()

	planet, err := testplanet.New(t, 1, 4, 0)
	if err != nil {
		t.Fatal(err)
	}
	defer ctx.Check(planet.Shutdown)

	overlayAddr := planet.Satellites[0].Addr()
	cache := planet.Satellites[0].Overlay

	ca, err := testidentity.NewTestCA(ctx)
	assert.NoError(t, err)
	identity, err := ca.NewIdentity()
	assert.NoError(t, err)

	oc, err := overlay.NewOverlayClient(identity, overlayAddr)
	assert.NoError(t, err)

	assert.NotNil(t, oc)
	_, ok := oc.(*overlay.Overlay)
	assert.True(t, ok)

	n1 := teststorj.MockNode("n1")
	n2 := teststorj.MockNode("n2")
	n3 := teststorj.MockNode("n3")
	nodes := []*pb.Node{n1, n2, n3}
	for _, n := range nodes {
		assert.NoError(t, cache.Put(n.Id, *n))
	}

	nid1 := teststorj.NodeIDFromString("n1")
	nid2 := teststorj.NodeIDFromString("n2")
	nid3 := teststorj.NodeIDFromString("n3")
	nid4 := teststorj.NodeIDFromString("n4")
	nid5 := teststorj.NodeIDFromString("n5")

	{ // empty id
		_, err := oc.BulkLookup(ctx, storj.NodeIDList{})
		assert.Error(t, err)
	}

	{ // valid ids
		ns, err := oc.BulkLookup(ctx, storj.NodeIDList{nid1, nid2, nid3})
		assert.NoError(t, err)
		assert.Equal(t, nodes, ns)
	}

	{ // missing ids
		ns, err := oc.BulkLookup(ctx, storj.NodeIDList{nid4, nid5})
		assert.NoError(t, err)
		assert.Equal(t, []*pb.Node{nil, nil}, ns)
	}

	{ // different order and missing
		ns, err := oc.BulkLookup(ctx, storj.NodeIDList{nid3, nid4, nid1, nid2, nid5})
		assert.NoError(t, err)
		assert.Equal(t, []*pb.Node{n3, nil, n1, n2, nil}, ns)
	}
}
