// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package contact_test

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"storj.io/storj/internal/testcontext"
	"storj.io/storj/pkg/pb"
	"storj.io/storj/pkg/storj"
	"storj.io/storj/satellite"
	"storj.io/storj/satellite/overlay"
	"storj.io/storj/satellite/satellitedb/satellitedbtest"
)

func TestUpdateCheckIn(t *testing.T) {
	satellitedbtest.Run(t, func(t *testing.T, db satellite.DB) {
		ctx := testcontext.New(t)
		defer ctx.Cleanup()

		// setup
		nodeID := storj.NodeID{1, 2, 3}
		expectedEmail := "test@email.com"
		expectedAddress := "1.2.4.4"
		info := overlay.NodeCheckInInfo{
			NodeID: nodeID,
			Address: &pb.NodeAddress{
				Address: expectedAddress,
			},
			IsUp: true,
			Capacity: &pb.NodeCapacity{
				FreeBandwidth: int64(1234),
				FreeDisk:      int64(5678),
			},
			Operator: &pb.NodeOperator{
				Email:  expectedEmail,
				Wallet: "0x123",
			},
			Version: &pb.NodeVersion{
				Version:    "v0.0.0",
				CommitHash: "",
				Timestamp:  time.Time{},
				Release:    false,
			},
		}
		expectedNode := &overlay.NodeDossier{
			Node: pb.Node{
				Id:     nodeID,
				LastIp: info.LastIP,
				Address: &pb.NodeAddress{
					Address:   info.Address.GetAddress(),
					Transport: pb.NodeTransport_TCP_TLS_GRPC,
				},
			},
			Type: pb.NodeType_STORAGE,
			Operator: pb.NodeOperator{
				Email:  info.Operator.GetEmail(),
				Wallet: info.Operator.GetWallet(),
			},
			Capacity: pb.NodeCapacity{
				FreeBandwidth: info.Capacity.GetFreeBandwidth(),
				FreeDisk:      info.Capacity.GetFreeDisk(),
			},
			Reputation: overlay.NodeStats{
				UptimeCount:           1,
				UptimeSuccessCount:    1,
				UptimeReputationAlpha: 1,
			},
			Version: pb.NodeVersion{
				Version:    "v0.0.0",
				CommitHash: "",
				Timestamp:  time.Time{},
				Release:    false,
			},
			Contained:    false,
			Disqualified: nil,
			PieceCount:   0,
		}
		config := overlay.NodeSelectionConfig{
			UptimeReputationLambda: 0.99,
			UptimeReputationWeight: 1.0,
			UptimeReputationDQ:     0,
		}

		// confirm the node doesn't exist in nodes table yet
		_, err := db.OverlayCache().Get(ctx, nodeID)
		require.Error(t, err)
		require.Contains(t, err.Error(), "node not found")

		// check-in for that node id, which should add the node
		// to the nodes tables in the database
		startOfTest := time.Now().UTC()
		err = db.OverlayCache().UpdateCheckIn(ctx, info, config)
		require.NoError(t, err)

		// confirm that the node is now in the nodes table with the
		// correct fields set
		actualNode, err := db.OverlayCache().Get(ctx, nodeID)
		require.NoError(t, err)
		require.True(t, actualNode.Reputation.LastContactSuccess.After(startOfTest))
		require.True(t, actualNode.Reputation.LastContactFailure.Equal(time.Time{}.UTC()))

		// we need to overwrite the times so that the deep equal considers them the same
		expectedNode.Reputation.LastContactSuccess = actualNode.Reputation.LastContactSuccess
		expectedNode.Reputation.LastContactFailure = actualNode.Reputation.LastContactFailure
		expectedNode.Version.Timestamp = actualNode.Version.Timestamp
		require.Equal(t, actualNode, expectedNode)

		// confirm that we can update the address field
		startOfUpdateTest := time.Now().UTC()
		expectedAddress = "9.8.7.6"
		updatedInfo := overlay.NodeCheckInInfo{
			NodeID: nodeID,
			Address: &pb.NodeAddress{
				Address: expectedAddress,
			},
			IsUp: true,
			Capacity: &pb.NodeCapacity{
				FreeBandwidth: int64(12355),
			},
			Version: &pb.NodeVersion{
				Version:    "v0.0.0",
				CommitHash: "",
				Timestamp:  time.Time{},
				Release:    false,
			},
		}
		// confirm that the updated node is in the nodes table with the
		// correct updated fields set
		err = db.OverlayCache().UpdateCheckIn(ctx, updatedInfo, config)
		require.NoError(t, err)
		updatedNode, err := db.OverlayCache().Get(ctx, nodeID)
		require.NoError(t, err)
		require.True(t, updatedNode.Reputation.LastContactSuccess.After(startOfUpdateTest))
		require.True(t, updatedNode.Reputation.LastContactFailure.Equal(time.Time{}.UTC()))
		require.Equal(t, updatedNode.Address.GetAddress(), expectedAddress)
		require.Equal(t, updatedNode.Reputation.UptimeSuccessCount, actualNode.Reputation.UptimeSuccessCount+1)
		require.Equal(t, updatedNode.Capacity.GetFreeBandwidth(), int64(12355))

		// confirm we can udpate IsUp field
		startOfUpdateTest2 := time.Now().UTC()
		updatedInfo2 := overlay.NodeCheckInInfo{
			NodeID: nodeID,
			Address: &pb.NodeAddress{
				Address: "9.8.7.6",
			},
			IsUp: false,
			Capacity: &pb.NodeCapacity{
				FreeBandwidth: int64(12355),
			},
			Version: &pb.NodeVersion{
				Version:    "v0.0.0",
				CommitHash: "",
				Timestamp:  time.Time{},
				Release:    false,
			},
		}
		err = db.OverlayCache().UpdateCheckIn(ctx, updatedInfo2, config)
		require.NoError(t, err)
		updated2Node, err := db.OverlayCache().Get(ctx, nodeID)
		require.NoError(t, err)
		require.True(t, updated2Node.Reputation.LastContactSuccess.Equal(updatedNode.Reputation.LastContactSuccess))
		require.Equal(t, updated2Node.Reputation.UptimeSuccessCount, updatedNode.Reputation.UptimeSuccessCount)
		require.True(t, updated2Node.Reputation.LastContactFailure.After(startOfUpdateTest2))
	})
}
