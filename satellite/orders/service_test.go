// Copyright (C) 2023 Storj Labs, Inc.
// See LICENSE for copying information.

package orders_test

import (
	"fmt"
	"testing"
	"time"

	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap/zaptest"

	"storj.io/common/identity/testidentity"
	"storj.io/common/pb"
	"storj.io/common/signing"
	"storj.io/common/storj"
	"storj.io/common/testcontext"
	"storj.io/common/testrand"
	"storj.io/storj/satellite/metabase"
	"storj.io/storj/satellite/nodeselection"
	"storj.io/storj/satellite/orders"
)

func TestGetOrderLimits(t *testing.T) {
	ctx := testcontext.New(t)
	ctrl := gomock.NewController(t)

	bucket := metabase.BucketLocation{ProjectID: testrand.UUID(), BucketName: "bucket1"}

	pieces := metabase.Pieces{}
	nodes := map[storj.NodeID]*nodeselection.SelectedNode{}
	for i := 0; i < 8; i++ {
		nodeID := testrand.NodeID()
		nodes[nodeID] = &nodeselection.SelectedNode{
			ID: nodeID,
			Address: &pb.NodeAddress{
				Address: fmt.Sprintf("host%d.com", i),
			},
		}

		pieces = append(pieces, metabase.Piece{
			Number:      uint16(i),
			StorageNode: nodeID,
		})
	}
	testIdentity, err := testidentity.PregeneratedIdentity(0, storj.LatestIDVersion())
	require.NoError(t, err)
	k := signing.SignerFromFullIdentity(testIdentity)

	overlayService := orders.NewMockOverlayForOrders(ctrl)
	overlayService.
		EXPECT().
		CachedGetOnlineNodesForGet(gomock.Any(), gomock.Any()).
		Return(nodes, nil).AnyTimes()

	service, err := orders.NewService(zaptest.NewLogger(t), k, overlayService, orders.NewNoopDB(),
		func(constraint storj.PlacementConstraint) (filter nodeselection.NodeFilter) {
			return nodeselection.AnyFilter{}
		},
		orders.Config{
			EncryptionKeys: orders.EncryptionKeys{
				Default: orders.EncryptionKey{
					ID:  orders.EncryptionKeyID{1, 2, 3, 4, 5, 6, 7, 8},
					Key: testrand.Key(),
				},
			},
		})
	require.NoError(t, err)

	segment := metabase.Segment{
		StreamID:  testrand.UUID(),
		CreatedAt: time.Now(),
		Redundancy: storj.RedundancyScheme{
			Algorithm:      storj.ReedSolomon,
			ShareSize:      256,
			RequiredShares: 4,
			RepairShares:   5,
			OptimalShares:  6,
			TotalShares:    10,
		},
		Pieces:       pieces,
		EncryptedKey: []byte{1, 2, 3, 4},
		RootPieceID:  testrand.PieceID(),
	}

	checkExpectedLimits := func(requested int32, received int) {
		limits, _, err := service.CreateGetOrderLimits(ctx, bucket, segment, requested, 0)
		require.NoError(t, err)
		realLimits := 0
		for _, limit := range limits {
			if limit.Limit != nil {
				realLimits++
			}
		}
		require.Equal(t, received, realLimits)
	}

	t.Run("Do not request any specific number", func(t *testing.T) {
		checkExpectedLimits(0, 6)
	})

	t.Run("Request less than the optimal", func(t *testing.T) {
		checkExpectedLimits(2, 6)
	})

	t.Run("Request more than the optimal", func(t *testing.T) {
		checkExpectedLimits(8, 8)
	})

	t.Run("Request more than the replication", func(t *testing.T) {
		checkExpectedLimits(1000, 8)
	})

}
