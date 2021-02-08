// Copyright (C) 2021 Storj Labs, Inc.
// See LICENSE for copying information.

package metabase_test

import (
	"sort"
	"testing"

	"github.com/stretchr/testify/require"
	"go.uber.org/zap/zaptest"

	"storj.io/common/testcontext"
	"storj.io/common/testrand"
	"storj.io/common/uuid"
	"storj.io/storj/satellite/metainfo/metabase"
	"storj.io/storj/satellite/satellitedb/satellitedbtest"
)

func TestMigrateToAliases(t *testing.T) {
	for _, info := range databaseInfos() {
		info := info
		t.Run(info.name, func(t *testing.T) {
			t.Parallel()

			ctx := testcontext.New(t)
			defer ctx.Cleanup()

			db, err := satellitedbtest.CreateMetabaseDB(ctx, zaptest.NewLogger(t), t.Name(), "M", 0, satellitedbtest.Database{
				Name:    info.name,
				URL:     info.connstr,
				Message: "",
			})
			require.NoError(t, err)
			defer ctx.Check(db.Close)

			mdb := db.InternalImplementation().(*metabase.DB)

			allMigrations := mdb.PostgresMigration()

			beforeAliases := allMigrations.TargetVersion(2)
			err = beforeAliases.Run(ctx, zaptest.NewLogger(t))
			require.NoError(t, err)

			rawdb := mdb.UnderlyingTagSQL()
			require.NotNil(t, rawdb)

			type segmentEntry struct {
				StreamID uuid.UUID
				Position metabase.SegmentPosition
				Pieces   metabase.Pieces
			}

			s1, s2 := testrand.UUID(), testrand.UUID()
			n1, n2, n3 := testrand.NodeID(), testrand.NodeID(), testrand.NodeID()

			entries := []segmentEntry{
				{
					StreamID: s1,
					Position: metabase.SegmentPosition{Index: 1},
					Pieces:   metabase.Pieces{{1, n1}, {2, n2}},
				},
				{
					StreamID: s1,
					Position: metabase.SegmentPosition{Part: 1, Index: 2},
					Pieces:   metabase.Pieces{{3, n3}, {2, n2}},
				},
				{
					StreamID: s2,
					Position: metabase.SegmentPosition{Part: 1, Index: 0},
					Pieces:   metabase.Pieces{{1, n1}},
				},
				{
					StreamID: s2,
					Position: metabase.SegmentPosition{Part: 1, Index: 1},
					Pieces:   metabase.Pieces{},
				},
			}

			for _, e := range entries {
				_, err = rawdb.ExecContext(ctx, `
					INSERT INTO segments (
						stream_id, position, remote_pieces,
						root_piece_id, encrypted_key_nonce, encrypted_key,
						encrypted_size, plain_offset, plain_size, redundancy
					) VALUES (
						$1, $2, $3,
						$4, $5, $6,
						$7, $8, $9, $10
					)`,
					e.StreamID, e.Position, e.Pieces,
					// mock values
					testrand.PieceID(), []byte{1, 2}, []byte{1, 2},
					100, 100, 100, int64(0x10),
				)
				require.NoError(t, err)
			}

			err = allMigrations.Run(ctx, zaptest.NewLogger(t))
			require.NoError(t, err)

			for _, e := range entries {
				seg, err := db.GetSegmentByPosition(ctx, metabase.GetSegmentByPosition{
					StreamID: e.StreamID,
					Position: e.Position,
				})
				require.NoError(t, err)

				sortedPieces := e.Pieces
				sort.Slice(sortedPieces, func(i, k int) bool {
					return sortedPieces[i].Number < sortedPieces[k].Number
				})
				require.Equal(t, sortedPieces, seg.Pieces)
			}
		})
	}
}
