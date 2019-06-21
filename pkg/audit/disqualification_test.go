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
	"storj.io/storj/pkg/storj"
	"storj.io/storj/satellite"
	"storj.io/storj/uplink"
)

func TestDisqualifiedNodesGetNoDownload(t *testing.T) {

	// - uploads random data
	// - mark a node as disqualified
	// - check we don't get it when we require order limit

	testplanet.Run(t, testplanet.Config{
		SatelliteCount: 1, StorageNodeCount: 4, UplinkCount: 1,
	}, func(t *testing.T, ctx *testcontext.Context, planet *testplanet.Planet) {
		satellite := planet.Satellites[0]
		disqualifiedNode := planet.StorageNodes[0]
		uplinkNode := planet.Uplinks[0]

		err := satellite.Audit.Service.Close()
		require.NoError(t, err)

		testData := make([]byte, 8*memory.KiB)
		_, err = rand.Read(testData)
		require.NoError(t, err)

		err = uplinkNode.UploadWithConfig(ctx, satellite, &uplink.RSConfig{
			MinThreshold:     2,
			RepairThreshold:  3,
			SuccessThreshold: 4,
			MaxThreshold:     4,
		}, "testbucket", "test/path", testData)
		require.NoError(t, err)

		projects, err := satellite.DB.Console().Projects().GetAll(ctx)
		require.NoError(t, err)

		bucketID := []byte(storj.JoinPaths(projects[0].ID.String(), "testbucket"))

		disqualifyNode(t, ctx, satellite, disqualifiedNode.ID())

		listResponse, _, err := satellite.Metainfo.Service.List(ctx, "", "", "", true, 0, 0)
		require.NoError(t, err)

		var path string
		var pointer *pb.Pointer
		for _, v := range listResponse {
			path = v.GetPath()
			pointer, err = satellite.Metainfo.Service.Get(ctx, path)
			require.NoError(t, err)
			if pointer.GetType() == pb.Pointer_REMOTE {
				break
			}
		}
		limits, err := satellite.Orders.Service.CreateGetOrderLimits(ctx, uplinkNode.Identity.PeerIdentity(), bucketID, pointer)
		require.NoError(t, err)

		for _, orderLimit := range limits {
			require.NotEqual(t, orderLimit.StorageNodeAddress.Address, disqualifiedNode.Addr())
		}

	})
}

func TestDisqualifiedNodesGetNoUpload(t *testing.T) {

	// - uploads random data
	// - mark a node as disqualified
	// - check we don't get it when we want to upload a segment (this should raise an error)

	testplanet.Run(t, testplanet.Config{
		SatelliteCount: 1, StorageNodeCount: 4, UplinkCount: 1,
	}, func(t *testing.T, ctx *testcontext.Context, planet *testplanet.Planet) {
		satellite := planet.Satellites[0]
		disqualifiedNode := planet.StorageNodes[0]
		uplinkNode := planet.Uplinks[0]

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

		apiKey := uplinkNode.APIKey[satellite.ID()]

		metainfo, err := uplinkNode.DialMetainfo(ctx, satellite, apiKey)
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
