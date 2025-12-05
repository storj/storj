// Copyright (C) 2025 Storj Labs, Inc.
// See LICENSE for copying information.

package rangedloop_test

import (
	"context"
	"sync"
	"testing"

	"github.com/stretchr/testify/require"
	"go.uber.org/zap/zaptest"

	"storj.io/common/storj"
	"storj.io/common/testcontext"
	"storj.io/common/uuid"
	"storj.io/storj/satellite/metabase/avrometabase"
	"storj.io/storj/satellite/metabase/rangedloop"
	"storj.io/storj/satellite/metabase/rangedloop/rangedlooptest"
)

func TestAvro(t *testing.T) {
	// this test is using Avro files exported from Spanner instance
	// test data was created manually and contains:
	// * 10 segments
	// * 10 segments with expiration date set
	// * corrsponding nodes	aliases

	ctx := testcontext.New(t)
	for _, batchSize := range []int{0, 1, 2, 3, 5, 10, 100} {
		config := rangedloop.Config{
			Parallelism: 2,
			BatchSize:   batchSize,
		}

		segmentsAvroIterator := avrometabase.NewFileIterator("./testdata/*segments.avro*")
		nodeAliasesAvroIterator := avrometabase.NewFileIterator("./testdata/*node_aliases.avro*")

		splitter := rangedloop.NewAvroSegmentsSplitter(segmentsAvroIterator, nodeAliasesAvroIterator)

		segments := []rangedloop.Segment{}

		mu := sync.Mutex{}
		callbackObserver := &rangedlooptest.CallbackObserver{
			OnProcess: func(ctx context.Context, s []rangedloop.Segment) error {
				mu.Lock()
				defer mu.Unlock()
				segments = append(segments, s...)
				return nil
			},
		}

		service := rangedloop.NewService(zaptest.NewLogger(t), config, splitter, []rangedloop.Observer{
			callbackObserver,
		})

		_, err := service.RunOnce(ctx)
		require.NoError(t, err)

		require.Equal(t, 20, len(segments))

		expiredCount := 0
		for _, segment := range segments {
			require.NotEqual(t, uuid.UUID{}, segment.StreamID)
			require.NotZero(t, segment.CreatedAt)
			require.NotEqual(t, storj.RedundancyScheme{}, segment.Redundancy)
			require.NotZero(t, segment.EncryptedSize)
			require.NotZero(t, segment.PlainSize)
			require.NotEmpty(t, segment.RootPieceID)
			require.NotEmpty(t, segment.AliasPieces)
			require.NotEmpty(t, segment.Pieces)
			require.Equal(t, "avro", segment.Source)

			if segment.ExpiresAt != nil {
				expiredCount++
			}
		}
		require.Equal(t, 10, expiredCount)
	}
}
