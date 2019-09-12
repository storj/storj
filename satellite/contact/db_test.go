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

		// confirm the node doesn't exist in nodes table yet
		_, err := db.OverlayCache().Get(ctx, nodeID)
		require.Error(t, err)
		require.Contains(t, err.Error(), "node not found")

		config := overlay.NodeSelectionConfig{
			UptimeReputationLambda: 0.99,
			UptimeReputationWeight: 1.0,
			UptimeReputationDQ:     0,
		}

		// check-in for that node id, which should add the node
		// to the nodes tables in the database
		startOfTest := time.Now()
		err = db.OverlayCache().UpdateCheckIn(ctx, info, config)
		require.NoError(t, err)

		// confirm that the node is now in the nodes table with the
		// correct fields set
		actualNode, err := db.OverlayCache().Get(ctx, nodeID)
		require.NoError(t, err)
		require.True(t, actualNode.Reputation.LastContactSuccess.After(startOfTest))
		require.True(t, actualNode.Reputation.LastContactFailure.Equal(time.Time{}.UTC()))

		// we need to overwrite the times so that the deep equal considers then the same
		expectedNode.Reputation.LastContactSuccess = actualNode.Reputation.LastContactSuccess
		expectedNode.Reputation.LastContactFailure = actualNode.Reputation.LastContactFailure
		expectedNode.Version.Timestamp = actualNode.Version.Timestamp
		require.Equal(t, actualNode, expectedNode)
	})
}
