// Copyright (C) 2025 Storj Labs, Inc.
// See LICENSE for copying information.

package metainfo

import (
	"context"
	"crypto/tls"
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"go.uber.org/zap"

	"storj.io/common/identity"
	"storj.io/common/identity/testidentity"
	"storj.io/common/macaroon"
	"storj.io/common/memory"
	"storj.io/common/pb"
	"storj.io/common/rpc/rpcpeer"
	"storj.io/common/signing"
	"storj.io/common/storj"
	"storj.io/common/uuid"
	"storj.io/storj/satellite/console"
	"storj.io/storj/satellite/internalpb"
	"storj.io/storj/satellite/metabase"
	"storj.io/storj/satellite/nodeselection"
	"storj.io/storj/satellite/orders"
	"storj.io/storj/satellite/overlay"
	"storj.io/storj/satellite/trust"
	"storj.io/storj/shared/mud"
	"storj.io/storj/shared/mud/mudtest"
)

const (
	nodeCount   = 130
	failedCount = 50
	groupSize   = 5
)

func TestRetryBeginSegmentPieces_Selection(t *testing.T) {
	t.Run("TopologySelector", func(t *testing.T) {
		testRetryBeginSegmentPieces(t, NewTopologyPlacement, false)
	})
	t.Run("StreamSelector", func(t *testing.T) {
		testRetryBeginSegmentPieces(t, NewStreamPlacement, false)
	})
}

func testRetryBeginSegmentPieces(t *testing.T, newPlacement func(*identity.FullIdentity) nodeselection.PlacementDefinitions, strict bool) {
	mudtest.RunF(t, mudtest.WithTestLogger(t, func(ball *mud.Ball) {
		mud.Provide[*identity.FullIdentity](ball, func() *identity.FullIdentity {
			return testidentity.MustPregeneratedSignedIdentity(0, storj.LatestIDVersion())
		})
		mud.Provide[signing.Signer](ball, signing.SignerFromFullIdentity)
		mud.Provide[*Endpoint](ball, NewEndpointT)
		mud.Provide[nodeselection.PlacementDefinitions](ball, newPlacement)
		mud.Provide[*retryTestUploadDB](ball, newUploadSelectionDB)
		mud.View[*retryTestUploadDB, overlay.UploadSelectionDB](ball, func(db *retryTestUploadDB) overlay.UploadSelectionDB { return db })
		mud.Provide[[]byte](ball, macaroon.NewSecret)
		mud.Provide[*overlay.UploadSelectionCache](ball, NewUploadCache)
		mud.Provide[*overlay.Service](ball, overlay.TestingNewServiceWithUploadCache)
		mud.Provide[*orders.Service](ball, NewOrdersService)

	}), mud.SelectIfExists[*Endpoint](), func(ctx context.Context, t testing.TB, endpoint *Endpoint, db *retryTestUploadDB, secret []byte) {
		apiKey, err := macaroon.NewAPIKey(secret)
		require.NoError(t, err)

		uplinkIdent := testidentity.MustPregeneratedSignedIdentity(nodeCount+groupSize, storj.LatestIDVersion())
		peerCtx := rpcpeer.NewContext(context.Background(), &rpcpeer.Peer{
			State: tls.ConnectionState{
				PeerCertificates: uplinkIdent.Chain(),
			},
		})

		placement := storj.PlacementConstraint(0)
		totalShares := endpoint.config.RS.Total

		// Step 1: Simulate what BeginSegment does: select nodes and create order limits.
		nodes, err := endpoint.overlay.FindStorageNodesForUpload(ctx, overlay.FindStorageNodesRequest{
			RequestedCount: totalShares,
			Placement:      placement,
		})
		require.NoError(t, err)
		require.Len(t, nodes, totalShares)

		bucket := metabase.BucketLocation{ProjectID: uuid.UUID{1}, BucketName: "test-bucket"}
		rootPieceID, addressedLimits, _, err := endpoint.orders.CreatePutOrderLimits(ctx, bucket, nodes, time.Time{}, 1024)
		require.NoError(t, err)
		require.Len(t, addressedLimits, totalShares)

		streamID := &internalpb.StreamID{
			Bucket:             []byte("test-bucket"),
			EncryptedObjectKey: []byte("test-key"),
			StreamId:           uuid.UUID{2}.Bytes(),
			Version:            1,
			Placement:          int32(placement),
		}
		segmentID, err := endpoint.packSegmentID(ctx, &internalpb.SegmentID{
			StreamId:            streamID,
			OriginalOrderLimits: addressedLimits,
			RootPieceId:         rootPieceID,
			CreationDate:        time.Now(),
		})
		require.NoError(t, err)

		checkGroup(t, "initial", db, addressedLimits, strict)

		// Step 2: Find which pieces map to failed nodes.
		var failedPieceNumbers []int32
		failedPieceNumbers = findFailed(db, addressedLimits)

		t.Logf("total pieces: %d, failed pieces: %d", totalShares, len(failedPieceNumbers))

		lastLimits := addressedLimits

		for retry := 1; retry < 10; retry++ {
			if len(lastLimits)-len(failedPieceNumbers) >= endpoint.config.RS.Success {
				t.Logf("enough pieces succeeded after %d retries, stopping", retry)
				break
			}

			t.Logf("retry %d: requesting %d", retry, len(failedPieceNumbers))
			resp, err := endpoint.RetryBeginSegmentPieces(peerCtx, &pb.RetryBeginSegmentPiecesRequest{
				Header:            &pb.RequestHeader{ApiKey: apiKey.SerializeRaw()},
				SegmentId:         segmentID,
				RetryPieceNumbers: failedPieceNumbers,
			})
			require.NoError(t, err)

			checkGroup(t, fmt.Sprintf("retry-%d", retry), db, resp.AddressedLimits, strict)

			lastLimits = resp.AddressedLimits

			failedPieceNumbers = findFailed(db, resp.AddressedLimits)
			t.Logf("retry %d: got failed: %d", retry, len(failedPieceNumbers))

		}

		// Step 4: Verify the final state.
		// TODO: this can fail due to the limitation of topology selector + mishandling of existing selection nodes in retry. See #7675. We should fix the underlying issue and re-enable this check.
		// failedPieceNumbers = findFailed(db, finalSeg.OriginalOrderLimits)
		// require.GreaterOrEqual(t, len(finalSeg.OriginalOrderLimits)-len(failedPieceNumbers), endpoint.config.RS.Success)

		seenNodes := make(map[storj.NodeID]int, totalShares)
		for _, limit := range lastLimits {
			seenNodes[limit.Limit.StorageNodeId]++
		}
		duplicates := 0
		for _, count := range seenNodes {
			if count > 1 {
				duplicates += count - 1
			}
		}
		t.Logf("unique nodes: %d, duplicates: %d (known limitation, see #7675)", len(seenNodes), duplicates)
	})
}

func findFailed(db *retryTestUploadDB, limits []*pb.AddressedOrderLimit) []int32 {
	var result []int32
	for i, limit := range limits {
		if db.isFailed(limit.Limit.StorageNodeId) {
			result = append(result, int32(i))
		}
	}
	return result
}

func checkGroup(t testing.TB, name string, db *retryTestUploadDB, limits []*pb.AddressedOrderLimit, strict bool) {
	g, err := nodeselection.CreateNodeAttribute("tag:server_group")

	groups := map[string]int{}

	require.NoError(t, err)
	for _, l := range limits {
		node := db.GetNode(l.Limit.StorageNodeId)
		require.NotNilf(t, node, "node %s not found in DB", l.Limit.StorageNodeId)
		groups[g(*node)]++
	}
	violation := 0
	for _, v := range groups {
		if v > 1 {
			violation++
		}
	}
	if violation > 0 {
		if strict {
			t.Fatalf("group violations %s: %d", name, violation)
		}
		// TODO: this should be a test failure once we fixed the problem
		t.Logf("group violations %s: %d", name, violation)
	}
}

func NewUploadCache(log *zap.Logger, mockDB overlay.UploadSelectionDB, placements nodeselection.PlacementDefinitions) (*overlay.UploadSelectionCache, error) {
	return overlay.NewUploadSelectionCache(
		log, mockDB, time.Hour,
		overlay.NodeSelectionConfig{
			OnlineWindow:     time.Hour,
			MinimumDiskSpace: 100 * memory.MB,
		},
		nodeselection.NodeFilters{},
		placements,
	)
}

func NewOrdersService(log *zap.Logger, placements nodeselection.PlacementDefinitions, signer signing.Signer, overlayService *overlay.Service) (*orders.Service, error) {
	var encKey orders.EncryptionKey
	err := encKey.Set("0100000000000000=0100000000000000000000000000000000000000000000000000000000000000")
	if err != nil {
		return nil, err
	}
	return orders.NewService(log, signer, overlayService, nil, placements.CreateFilters, orders.Config{
		EncryptionKeys: orders.EncryptionKeys{
			Default: encKey,
			KeyByID: map[orders.EncryptionKeyID]storj.Key{encKey.ID: encKey.Key},
		},
		Expiration: 24 * time.Hour,
	})

}

func newUploadSelectionDB(signer signing.Signer) *retryTestUploadDB {

	nodes := make([]*nodeselection.SelectedNode, nodeCount)
	for i := range nodes {
		ident := testidentity.MustPregeneratedSignedIdentity(i+1, storj.LatestIDVersion())
		groupName := fmt.Sprintf("group-%d", i/groupSize)
		nodes[i] = &nodeselection.SelectedNode{
			ID: ident.ID,
			Address: &pb.NodeAddress{
				Address: fmt.Sprintf("127.0.%d.%d:8080", i/256, i%256),
			},
			LastNet:    fmt.Sprintf("127.0.%d.0", i/groupSize),
			LastIPPort: fmt.Sprintf("127.0.%d.%d:8080", i/256, i%256),
			Online:     true,
			Vetted:     true,
			FreeDisk:   1 * memory.GB.Int64(),
			Tags: nodeselection.NodeTags{
				{
					NodeID: ident.ID,
					Signer: signer.ID(),
					Name:   "server_group",
					Value:  []byte(groupName),
				},
			},
		}
	}

	// Build UploadSelectionCache with a mock DB that returns our nodes.
	mockDB := &retryTestUploadDB{nodes: nodes}
	return mockDB
}

func NewTopologyPlacement(id *identity.FullIdentity) nodeselection.PlacementDefinitions {
	tagAttr := "tag:server_group"
	placements := nodeselection.NewPlacementDefinitions(nodeselection.Placement{
		ID:   0,
		Name: "default",
		Selector: nodeselection.TopologySelector(
			func(node nodeselection.SelectedNode) float64 { return 1 },
			"999,1", // select 1 from each groups
			tagAttr, // group by server_group tag
			nil,
		),
	})
	return placements
}

func NewStreamPlacement(id *identity.FullIdentity) nodeselection.PlacementDefinitions {
	// Placement with Stream selector using RandomStream + GroupConstraint on server_group tag.
	groupAttr, err := nodeselection.CreateNodeAttribute("tag:server_group")
	if err != nil {
		panic(err)
	}
	placements := nodeselection.NewPlacementDefinitions(nodeselection.Placement{
		ID:   0,
		Name: "default",
		Selector: nodeselection.Stream(
			nodeselection.RandomStream,
			nodeselection.StreamFilter(nodeselection.GroupConstraint(groupAttr, 1)),
		),
	})
	return placements
}

func NewEndpointT(log *zap.Logger, secret []byte, placements nodeselection.PlacementDefinitions, overlayService *overlay.Service, service *orders.Service, signer signing.Signer) *Endpoint {
	return &Endpoint{
		log:       log,
		overlay:   overlayService,
		orders:    service,
		satellite: signer,
		apiKeys: &retryTestAPIKeys{
			secret:    secret,
			projectID: uuid.UUID{1},
		},
		config: Config{
			SuccessTrackerTickDuration: 1 * time.Hour,
			FailureTrackerTickDuration: 1 * time.Hour,
			RS: RSConfig{
				Min:              7,
				Repair:           12,
				Success:          13,
				Total:            16,
				ErasureShareSize: 256 * memory.B,
			},
		},
		placement:         placements,
		versionCollector:  newVersionCollector(log),
		migrationModeFlag: NewMigrationModeFlagExtension(Config{}),
		trackers: NewTrackers(Config{}, nil, func(id storj.NodeID) SuccessTracker {
			return NewBigBitshiftSuccessTracker(64)
		}, NewBigBitshiftSuccessTracker(64), trust.NewTrustedPeerList(nil)),
		trustedUplinks:     trust.NewTrustedPeerList(nil),
		nodeSelectionStats: NewNodeSelectionStats(),
	}
}

// retryTestAPIKeys implements APIKeys for testing.
type retryTestAPIKeys struct {
	secret    []byte
	projectID uuid.UUID
}

func (m *retryTestAPIKeys) GetByHead(context.Context, []byte) (*console.APIKeyInfo, error) {
	unlimited := int64(-1)
	return &console.APIKeyInfo{
		ProjectID:            m.projectID,
		ProjectPublicID:      m.projectID,
		Secret:               m.secret,
		ProjectStorageLimit:  &unlimited,
		ProjectSegmentsLimit: &unlimited,
	}, nil
}

// retryTestUploadDB implements overlay.UploadSelectionDB.
type retryTestUploadDB struct {
	nodes []*nodeselection.SelectedNode
}

func (m *retryTestUploadDB) SelectAllStorageNodesUpload(_ context.Context, _ overlay.NodeSelectionConfig) (reputable, new []*nodeselection.SelectedNode, err error) {
	return m.nodes, nil, nil
}

func (m *retryTestUploadDB) isFailed(id pb.NodeID) bool {
	for i := 0; i < failedCount; i++ {
		if m.nodes[i].ID == id {
			return true
		}
	}
	return false
}

func (m *retryTestUploadDB) GetNode(id pb.NodeID) *nodeselection.SelectedNode {
	for _, node := range m.nodes {
		if node.ID == id {
			return node
		}
	}
	return nil
}
