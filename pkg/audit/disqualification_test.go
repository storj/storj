// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package audit_test

import (
	"math/rand"
	"testing"

	"github.com/golang/protobuf/ptypes"
	"github.com/stretchr/testify/require"

	"storj.io/storj/internal/memory"
	"storj.io/storj/internal/testcontext"
	"storj.io/storj/internal/testplanet"
	"storj.io/storj/pkg/overlay"
	"storj.io/storj/pkg/pb"
	"storj.io/storj/pkg/storage/streams"
	"storj.io/storj/pkg/storj"
	"storj.io/storj/satellite"
)

func TestDisqualifiedNodesGetNoDownload(t *testing.T) {

	// - uploads random data
	// - mark a node as disqualified
	// - check we don't get it when we require order limit

	testplanet.Run(t, testplanet.Config{
		SatelliteCount: 1, StorageNodeCount: 4, UplinkCount: 1,
	}, func(t *testing.T, ctx *testcontext.Context, planet *testplanet.Planet) {
		satellite := planet.Satellites[0]
		upl := planet.Uplinks[0]

		err := satellite.Audit.Service.Close()
		require.NoError(t, err)

		testData := make([]byte, 8*memory.KiB)
		_, err = rand.Read(testData)
		require.NoError(t, err)

		err = upl.Upload(ctx, planet.Satellites[0], "testbucket", "test/path", testData)
		require.NoError(t, err)

		projects, err := satellite.DB.Console().Projects().GetAll(ctx)
		require.NoError(t, err)
		require.Len(t, projects, 1)

		encScheme := upl.GetConfig(satellite).GetEncryptionScheme()
		cipher := encScheme.Cipher
		encryptedAfterBucket, err := streams.EncryptAfterBucket(ctx, "testbucket/test/path", cipher, &storj.Key{})
		require.NoError(t, err)

		lastSegPath := storj.JoinPaths(projects[0].ID.String(), "l", encryptedAfterBucket)
		pointer, err := satellite.Metainfo.Service.Get(ctx, lastSegPath)
		require.NoError(t, err)

		disqualifiedNode := pointer.GetRemote().GetRemotePieces()[0].NodeId
		disqualifyNode(t, ctx, satellite, disqualifiedNode)

		request := overlay.FindStorageNodesRequest{
			MinimumRequiredNodes: 4,
			RequestedCount:       0,
			FreeBandwidth:        0,
			FreeDisk:             0,
			ExcludedNodes:        nil,
			MinimumVersion:       "", // semver or empty
		}
		nodes, err := satellite.Overlay.Service.FindStorageNodes(ctx, request)
		require.True(t, overlay.ErrNotEnoughNodes.Has(err))

		require.NotEmpty(t, nodes)
		for _, node := range nodes {
			reputable, err := satellite.Overlay.Service.IsVetted(ctx, node.Id)
			require.NoError(t, err)
			require.True(t, reputable)
		}

	})
}

func TestDisqualifiedNodesGetNoUpload(t *testing.T) {

	// - mark a node as disqualified
	// - check that we have an error if we try to create a segment using all storage nodes

	testplanet.Run(t, testplanet.Config{
		SatelliteCount: 1, StorageNodeCount: 4, UplinkCount: 1,
	}, func(t *testing.T, ctx *testcontext.Context, planet *testplanet.Planet) {
		satellite := planet.Satellites[0]
		disqualifiedNode := planet.StorageNodes[0]
		upl := planet.Uplinks[0]

		err := satellite.Audit.Service.Close()
		require.NoError(t, err)

		disqualifyNode(t, ctx, satellite, disqualifiedNode.ID())

		rs := &pb.RedundancyScheme{
			MinReq:           1,
			RepairThreshold:  1,
			SuccessThreshold: 3,
			Total:            4,
			ErasureShareSize: 1024,
			Type:             pb.RedundancyScheme_RS,
		}

		pointer := &pb.Pointer{
			Type: pb.Pointer_REMOTE,
			Remote: &pb.RemoteSegment{
				Redundancy: rs,
				RemotePieces: []*pb.RemotePiece{
					&pb.RemotePiece{
						PieceNum: 0,
					},
					&pb.RemotePiece{
						PieceNum: 1,
					},
				},
			},
			ExpirationDate: ptypes.TimestampNow(),
		}

		expirationDate, err := ptypes.Timestamp(pointer.ExpirationDate)
		require.NoError(t, err)

		apiKey := upl.APIKey[satellite.ID()]

		metainfo, err := upl.DialMetainfo(ctx, satellite, apiKey)
		require.NoError(t, err)

		_, _, err = metainfo.CreateSegment(ctx, "testbucket", "file/path", -1, pointer.Remote.Redundancy, memory.MiB.Int64(), expirationDate)
		require.Error(t, err)

	})
}

func disqualifyNode(t *testing.T, ctx *testcontext.Context, satellite *satellite.Peer, nodeID storj.NodeID) {
	_, err := satellite.DB.OverlayCache().UpdateStats(ctx, &overlay.UpdateRequest{
		NodeID:       nodeID,
		IsUp:         true,
		AuditSuccess: false,
		AuditLambda:  0,
		AuditWeight:  1,
		AuditDQ:      0.5,
		UptimeLambda: 1,
		UptimeWeight: 1,
		UptimeDQ:     0.5,
	})
	require.NoError(t, err)
	reputable, err := satellite.Overlay.Service.IsVetted(ctx, nodeID)
	require.NoError(t, err)
	require.False(t, reputable)
}
