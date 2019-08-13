// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package audit_test

import (
	"context"
	"fmt"
	"testing"

	"storj.io/storj/pkg/storj"

	"github.com/gogo/protobuf/proto"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap/zaptest"

	"storj.io/storj/internal/memory"
	"storj.io/storj/internal/testcontext"
	"storj.io/storj/internal/testplanet"
	"storj.io/storj/internal/testrand"
	"storj.io/storj/pkg/pb"
	"storj.io/storj/satellite/audit"
	"storj.io/storj/storage"
)

// For every node in testplanet:
//    - expect that there is a reservoir for that node
//    - that the reservoir size is <= 3
//    - that every item in the reservoir is unique
//    - that looking up each pieceID in allPieces results in the same node ID
func TestAuditObserver(t *testing.T) {
	testplanet.Run(t, testplanet.Config{
		SatelliteCount: 1, StorageNodeCount: 5, UplinkCount: 1,
	}, func(t *testing.T, ctx *testcontext.Context, planet *testplanet.Planet) {
		audits := planet.Satellites[0].Audit.Service
		satellite := planet.Satellites[0]
		err := audits.Close()
		require.NoError(t, err)

		uplink := planet.Uplinks[0]

		// upload 5 remote files with 1 segment
		for i := 0; i < 5; i++ {
			testData := testrand.Bytes(8 * memory.KiB)
			path := "/some/remote/path/" + string(i)
			err := uplink.Upload(ctx, satellite, "bucket", path, testData)
			require.NoError(t, err)
		}

		observer := audit.NewObserver(zaptest.NewLogger(t), satellite.Overlay.Service, audit.ReservoirConfig{3, 3})

		allPieces := make(map[storj.PieceID]storj.NodeID)

		srv := satellite.Metainfo.Service
		fmt.Println("srv", srv)

		err = satellite.Metainfo.Service.Iterate(ctx, "", "", true, false,
			func(ctx context.Context, it storage.Iterator) error {
				var item storage.ListItem

				// iterate over every segment in metainfo
				for it.Next(ctx, &item) {
					pointer := &pb.Pointer{}

					err = proto.Unmarshal(item.Value, pointer)
					require.NoError(t, err)

					// todo: why is piece.GetHash() nil?
					for _, piece := range pointer.GetRemote().GetRemotePieces() {
						fmt.Println("piece hash", piece.GetHash())
						fmt.Println("piece id", piece.GetHash().PieceId)
						allPieces[piece.GetHash().PieceId] = piece.NodeId
					}

					// if context has been canceled exit. Otherwise, continue
					select {
					case <-ctx.Done():
						return ctx.Err()
					default:
					}
				}
				return nil
			})
		require.NoError(t, err)

		err = audits.MetainfoLoop.Join(ctx, observer)
		require.NoError(t, err)

		for _, node := range planet.StorageNodes {
			// expect a reservoir for every node
			require.NotNil(t, observer.Reservoirs[node.ID()])
			if len(observer.Reservoirs[node.ID()].Paths) <= 3 {
				t.Error("expected all reservoir sizes to be <= 3")
			}
			repeats := make(map[storj.Path]bool)
			for _, path := range observer.Reservoirs[node.ID()].Paths {
				require.False(t, repeats[path], "expected every item in reservoir to be unique")
				repeats[path] = true
			}
		}
	})
}
