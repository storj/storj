// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package overlay_test

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"storj.io/storj/internal/testcontext"
	"storj.io/storj/internal/testplanet"
	"storj.io/storj/pkg/overlay"
	"storj.io/storj/pkg/pb"
	"storj.io/storj/pkg/storj"
)

func TestChoose(t *testing.T) {
	testplanet.Run(t, testplanet.Config{
		SatelliteCount: 1, StorageNodeCount: 8, UplinkCount: 1,
	}, func(t *testing.T, ctx *testcontext.Context, planet *testplanet.Planet) {
		// we wait a second for all the nodes to complete bootstrapping off the satellite
		time.Sleep(2 * time.Second)

		oc, err := planet.Uplinks[0].DialOverlay(planet.Satellites[0])
		if err != nil {
			t.Fatal(err)
		}

		cases := []struct {
			limit     int
			space     int64
			bandwidth int64
		}{
			{
				limit:     4,
				space:     0,
				bandwidth: 0,
			},
		}

		for _, v := range cases {
			newNodes, err := oc.Choose(ctx, overlay.Options{
				Amount: v.limit,
				Space:  v.space,
			})
			assert.NoError(t, err)

			assert.Len(t, newNodes, v.limit)
			for _, n := range newNodes {
				assert.True(t, n.GetRestrictions().GetFreeDisk() >= v.space)
				assert.True(t, n.GetRestrictions().GetFreeBandwidth() >= v.bandwidth)
			}
		}
	})
}

func TestLookup(t *testing.T) {
	testplanet.Run(t, testplanet.Config{
		SatelliteCount: 1, StorageNodeCount: 4, UplinkCount: 1,
	}, func(t *testing.T, ctx *testcontext.Context, planet *testplanet.Planet) {
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
				nodeID:    storj.NodeID{1},
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
	})
}

func TestBulkLookup(t *testing.T) {
	testplanet.Run(t, testplanet.Config{
		SatelliteCount: 1, StorageNodeCount: 4, UplinkCount: 1,
	}, func(t *testing.T, ctx *testcontext.Context, planet *testplanet.Planet) {
		// we wait a second for all the nodes to complete bootstrapping off the satellite
		time.Sleep(2 * time.Second)
		defer ctx.Check(planet.Shutdown)

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
	})
}

func TestBulkLookupV2(t *testing.T) {
	testplanet.Run(t, testplanet.Config{
		SatelliteCount: 1, StorageNodeCount: 4, UplinkCount: 1,
	}, func(t *testing.T, ctx *testcontext.Context, planet *testplanet.Planet) {
		// we wait a second for all the nodes to complete bootstrapping off the satellite
		time.Sleep(2 * time.Second)
		defer ctx.Check(planet.Shutdown)

		oc, err := planet.Uplinks[0].DialOverlay(planet.Satellites[0])
		if err != nil {
			t.Fatal(err)
		}

		cache := planet.Satellites[0].Overlay.Service

		nid1 := storj.NodeID{1}
		nid2 := storj.NodeID{2}
		nid3 := storj.NodeID{3}
		nid4 := storj.NodeID{4}
		nid5 := storj.NodeID{5}

		n1 := &pb.Node{Id: storj.NodeID{1}}
		n2 := &pb.Node{Id: storj.NodeID{2}}
		n3 := &pb.Node{Id: storj.NodeID{3}}

		nodes := []*pb.Node{n1, n2, n3}
		for _, n := range nodes {
			assert.NoError(t, cache.Put(ctx, n.Id, *n))
		}

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
	})
}
