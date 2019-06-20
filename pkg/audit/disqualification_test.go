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
		err := planet.Satellites[0].Audit.Service.Close()
		require.NoError(t, err)

		testData := make([]byte, 8*memory.KiB)
		_, err = rand.Read(testData)
		require.NoError(t, err)

		err = planet.Uplinks[0].UploadWithConfig(ctx, planet.Satellites[0], &uplink.RSConfig{
			MinThreshold:     2,
			RepairThreshold:  3,
			SuccessThreshold: 4,
			MaxThreshold:     4,
		}, "testbucket", "test/path", testData)
		require.NoError(t, err)

		projects, err := planet.Satellites[0].DB.Console().Projects().GetAll(ctx)
		require.NoError(t, err)

		bucketID := []byte(storj.JoinPaths(projects[0].ID.String(), "testbucket"))

		disqualifyNode(t, ctx, planet.Satellites[0], planet.StorageNodes[0].ID())

		listResponse, _, err := planet.Satellites[0].Metainfo.Service.List(ctx, "", "", "", true, 0, 0)
		require.NoError(t, err)

		var path string
		var pointer *pb.Pointer
		for _, v := range listResponse {
			path = v.GetPath()
			pointer, err = planet.Satellites[0].Metainfo.Service.Get(ctx, path)
			require.NoError(t, err)
			if pointer.GetType() == pb.Pointer_REMOTE {
				break
			}
		}
		limits, err := planet.Satellites[0].Orders.Service.CreateGetOrderLimits(ctx, planet.Uplinks[0].Identity.PeerIdentity(), bucketID, pointer)
		require.NoError(t, err)

		for _, orderLimit := range limits {
			require.NotEqual(t, orderLimit.StorageNodeAddress.Address, planet.StorageNodes[0].Addr())
		}

	})
}

func TestDisqualifiedNodesGetNoUpload(t *testing.T) {

	// - uploads random data
	// - mark a node as disqualified
	// - check we don't get it when we want to upload a segment

	testplanet.Run(t, testplanet.Config{
		SatelliteCount: 1, StorageNodeCount: 4, UplinkCount: 1,
	}, func(t *testing.T, ctx *testcontext.Context, planet *testplanet.Planet) {
		err := planet.Satellites[0].Audit.Service.Close()
		require.NoError(t, err)

		disqualifyNode(t, ctx, planet.Satellites[0], planet.StorageNodes[0].ID())

		rs := &pb.RedundancyScheme{
			MinReq:           1,
			RepairThreshold:  1,
			SuccessThreshold: 3,
			Total:            3,
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

		apiKey := planet.Uplinks[0].APIKey[planet.Satellites[0].ID()]

		metainfo, err := planet.Uplinks[0].DialMetainfo(ctx, planet.Satellites[0], apiKey)
		require.NoError(t, err)

		limits, _, err := metainfo.CreateSegment(ctx, "myBucketName", "file/path", -1, pointer.Remote.Redundancy, memory.MiB.Int64(), expirationDate)
		require.NoError(t, err)

		for _, orderLimit := range limits {
			require.NotEqual(t, orderLimit.StorageNodeAddress.Address, planet.StorageNodes[0].Addr())
		}
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
