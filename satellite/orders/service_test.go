// Copyright (C) 2023 Storj Labs, Inc.
// See LICENSE for copying information.

package orders

import (
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
	"go.uber.org/zap"
	"go.uber.org/zap/zaptest"

	"storj.io/common/identity/testidentity"
	"storj.io/common/pb"
	"storj.io/common/signing"
	"storj.io/common/storj"
	"storj.io/common/testcontext"
	"storj.io/common/testrand"
	"storj.io/storj/satellite/metabase"
	"storj.io/storj/satellite/nodeselection"
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

	uplinkIdentity, err := testidentity.PregeneratedIdentity(0, storj.LatestIDVersion())
	require.NoError(t, err)

	overlayService := NewMockOverlayForOrders(ctrl)
	overlayService.
		EXPECT().
		CachedGetOnlineNodesForGet(gomock.Any(), gomock.Any()).
		Return(nodes, nil).AnyTimes()

	service, err := NewService(zaptest.NewLogger(t), k, overlayService, NewNoopDB(),
		func(constraint storj.PlacementConstraint) (nodeselection.NodeFilter, nodeselection.DownloadSelector) {
			return nodeselection.AnyFilter{}, nodeselection.DefaultDownloadSelector
		},
		Config{
			EncryptionKeys: EncryptionKeys{
				Default: EncryptionKey{
					ID:  EncryptionKeyID{1, 2, 3, 4, 5, 6, 7, 8},
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
		limits, _, err := service.CreateGetOrderLimits(ctx, uplinkIdentity.PeerIdentity(), bucket, segment, requested, 0)
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

func TestDownloadNodes(t *testing.T) {
	key, err := storj.NewKey([]byte("test-key"))
	require.NoError(t, err)
	encryptionKeys := EncryptionKeys{
		Default: EncryptionKey{
			ID:  EncryptionKeyID{0, 1, 2, 3, 4, 5, 6, 7},
			Key: *key,
		},
	}

	service, err := NewService(zap.L(), nil, nil, nil, nil, Config{EncryptionKeys: encryptionKeys})
	require.NoError(t, err)

	for i, tt := range []struct {
		k, m, o, n int16
		needed     int32
	}{
		{k: 0, m: 0, o: 0, n: 0, needed: 0},
		{k: 1, m: 1, o: 1, n: 1, needed: 1},
		{k: 1, m: 1, o: 2, n: 2, needed: 2},
		{k: 1, m: 2, o: 2, n: 2, needed: 2},
		{k: 2, m: 3, o: 4, n: 4, needed: 3},
		{k: 2, m: 4, o: 6, n: 8, needed: 3},
		{k: 20, m: 30, o: 40, n: 50, needed: 25},
		{k: 29, m: 35, o: 80, n: 95, needed: 34},
	} {
		tag := fmt.Sprintf("#%d. %+v", i, tt)

		rs := storj.RedundancyScheme{
			RequiredShares: tt.k,
			RepairShares:   tt.m,
			OptimalShares:  tt.o,
			TotalShares:    tt.n,
		}

		require.Equal(t, tt.needed, service.DownloadNodes(rs), tag)
	}

	service, err = NewService(zap.L(), nil, nil, nil, nil, Config{
		EncryptionKeys:                 encryptionKeys,
		DownloadTailToleranceOverrides: "1-4,20-21",
	})
	require.NoError(t, err)

	for i, tt := range []struct {
		k, m, o, n int16
		needed     int32
	}{
		{k: 0, m: 0, o: 0, n: 0, needed: 0},
		{k: 1, m: 1, o: 1, n: 1, needed: 4},
		{k: 1, m: 1, o: 2, n: 2, needed: 4},
		{k: 1, m: 2, o: 2, n: 2, needed: 4},
		{k: 2, m: 3, o: 4, n: 4, needed: 3},
		{k: 2, m: 4, o: 6, n: 8, needed: 3},
		{k: 20, m: 30, o: 40, n: 50, needed: 21},
		{k: 29, m: 35, o: 80, n: 95, needed: 34},
	} {
		tag := fmt.Sprintf("#%d. %+v", i, tt)

		rs := storj.RedundancyScheme{
			RequiredShares: tt.k,
			RepairShares:   tt.m,
			OptimalShares:  tt.o,
			TotalShares:    tt.n,
		}

		require.Equal(t, tt.needed, service.DownloadNodes(rs), tag)
	}
}

func TestParseDownloadOverrides(t *testing.T) {
	for i, tt := range []struct {
		unparsed string
		parsed   map[int16]int32
		success  bool
	}{
		{"", map[int16]int32{}, true},
		{" \n", map[int16]int32{}, true},
		{"29-28", nil, false},
		{"29-29", map[int16]int32{29: 35}, true},
		{"29-35", map[int16]int32{29: 35}, true},
		{"29-35,29-36", nil, false},
		{"29-35,2-4", map[int16]int32{2: 4, 29: 35}, true},
		{"29-35,2-4,7-9", map[int16]int32{2: 4, 29: 35, 7: 9}, true},
		{"29-35,", nil, false},
		{",29-35", nil, false},
	} {
		tag := fmt.Sprintf("#%d. %+v", i, tt)

		actual, err := parseDownloadOverrides(tt.unparsed)
		if !tt.success {
			require.Error(t, err, tag)
			continue
		}
		require.Equal(t, len(actual), len(tt.parsed), tag)
		for k, v := range tt.parsed {
			require.Equal(t, v, tt.parsed[k], tag)
		}
	}
}
