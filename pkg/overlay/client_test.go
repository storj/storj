// Copyright (C) 2018 Storj Labs, Inc.
// See LICENSE for copying information.

package overlay_test

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"storj.io/storj/internal/identity"
	"storj.io/storj/internal/testcontext"
	"storj.io/storj/internal/testplanet"
	"storj.io/storj/internal/teststorj"
	"storj.io/storj/pkg/overlay"
	"storj.io/storj/pkg/pb"
	"storj.io/storj/pkg/storj"
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

	planet, err := testplanet.New(t, 1, 4, 1)
	if err != nil {
		t.Fatal(err)
	}
	defer ctx.Check(planet.Shutdown)

	planet.Start(ctx)
	// we wait a second for all the nodes to complete bootstrapping off the satellite
	time.Sleep(2 * time.Second)

	oc, err := planet.Uplinks[0].DialOverlay(planet.Satellites[0])
	if err != nil {
		t.Fatal(err)
	}

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
			uptime:       0,
			uptimeCount:  0,
			auditSuccess: 0,
			auditCount:   0,
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

		excludedNodes := make(map[storj.NodeID]bool)
		for _, e := range v.excluded {
			excludedNodes[e] = true
		}
		assert.Len(t, newNodes, v.limit)
		for _, n := range newNodes {
			assert.NotContains(t, excludedNodes, n.Id)
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

	planet, err := testplanet.New(t, 1, 4, 1)
	if err != nil {
		t.Fatal(err)
	}
	defer ctx.Check(planet.Shutdown)

	planet.Start(ctx)
	// we wait a second for all the nodes to complete bootstrapping off the satellite
	time.Sleep(2 * time.Second)

	oc, err := planet.Uplinks[0].DialOverlay(planet.Satellites[0])
	if err != nil {
		t.Fatal(err)
	}
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

	planet, err := testplanet.New(t, 1, 4, 1)
	if err != nil {
		t.Fatal(err)
	}
	defer ctx.Check(planet.Shutdown)

	planet.Start(ctx)
	// we wait a second for all the nodes to complete bootstrapping off the satellite
	time.Sleep(2 * time.Second)

	oc, err := planet.Uplinks[0].DialOverlay(planet.Satellites[0])
	if err != nil {
		t.Fatal(err)
	}
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
		resNodes, err := oc.BulkLookup(ctx, v.nodeIDs)
		assert.NoError(t, err)
		for i, n := range resNodes {
			assert.Equal(t, n.Id, v.nodeIDs[i])
		}
		assert.Equal(t, len(resNodes), len(v.nodeIDs))
	}
}

func TestBulkLookupV2(t *testing.T) {
	ctx := testcontext.New(t)
	defer ctx.Cleanup()

	planet, err := testplanet.New(t, 1, 4, 1)
	if err != nil {
		t.Fatal(err)
	}
	defer ctx.Check(planet.Shutdown)

	planet.Start(ctx)
	// we wait a second for all the nodes to complete bootstrapping off the satellite
	time.Sleep(2 * time.Second)

	oc, err := planet.Uplinks[0].DialOverlay(planet.Satellites[0])
	if err != nil {
		t.Fatal(err)
	}
	cache := planet.Satellites[0].Overlay

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
		idList := storj.NodeIDList{nid1, nid2, nid3}
		ns, err := oc.BulkLookup(ctx, idList)
		assert.NoError(t, err)

		for i, n := range ns {
			assert.Equal(t, n.Id, idList[i])
		}
	}

	{ // missing ids
		idList := storj.NodeIDList{nid4, nid5}
		ns, err := oc.BulkLookup(ctx, idList)
		assert.NoError(t, err)

		assert.Equal(t, []*pb.Node{nil, nil}, ns)
	}

	{ // different order and missing
		idList := storj.NodeIDList{nid3, nid4, nid1, nid2, nid5}
		ns, err := oc.BulkLookup(ctx, idList)
		assert.NoError(t, err)

		expectedNodes := []*pb.Node{n3, nil, n1, n2, nil}
		for i, n := range ns {
			if n == nil {
				assert.Nil(t, expectedNodes[i])
			} else {
				assert.Equal(t, n.Id, expectedNodes[i].Id)
			}
		}
	}
}
