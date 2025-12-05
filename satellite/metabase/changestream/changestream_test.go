// Copyright (C) 2025 Storj Labs, Inc.
// See LICENSE for copying information.

package changestream_test

import (
	"context"
	"testing"
	"time"

	"cloud.google.com/go/spanner"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap/zaptest"
	"google.golang.org/grpc/codes"

	"storj.io/common/errs2"
	"storj.io/common/testcontext"
	"storj.io/storj/satellite/metabase"
	"storj.io/storj/satellite/metabase/changestream"
	"storj.io/storj/satellite/metabase/metabasetest"
	"storj.io/storj/shared/dbutil"
)

func TestChangeStream(t *testing.T) {
	log := zaptest.NewLogger(t)
	metabasetest.Run(t, func(ctx *testcontext.Context, t *testing.T, db *metabase.DB) {
		streamId := metabasetest.RandObjectStream()
		adapter := db.ChooseAdapter(streamId.ProjectID)

		// Skip test if not using Spanner
		if db.Implementation() != dbutil.Spanner {
			t.Skip("test requires Spanner adapter")
		}

		// Verify that SpannerAdapter implements Adapter interface
		spannerAdapter, ok := adapter.(*metabase.SpannerAdapter)
		require.True(t, ok, "adapter should be SpannerAdapter")

		changeStreamAdapter := changestream.Adapter(spannerAdapter)

		changefeedName := "test_interface_changefeed"

		err := changeStreamAdapter.TestCreateChangeStream(ctx, changefeedName)
		require.NoError(t, err)

		startTime := time.Now()

		feedCtx, cancel := context.WithCancel(ctx)
		changes := make(chan changestream.DataChangeRecord)
		feedErr := make(chan error)
		go func() {
			err = changestream.Processor(feedCtx, log, spannerAdapter, changefeedName, startTime, func(record changestream.DataChangeRecord) error {
				changes <- record
				return nil
			})
			feedErr <- err
		}()

		err = adapter.TestingBatchInsertObjects(ctx, []metabase.RawObject{
			{
				ObjectStream: metabase.ObjectStream{
					ProjectID:  streamId.ProjectID,
					BucketName: streamId.BucketName,
					ObjectKey:  streamId.ObjectKey,
				},
			},
		})
		require.NoError(t, err)

		select {
		case change := <-changes:
			require.Equal(t, "objects", change.TableName)
			require.Equal(t, "INSERT", change.ModType)
		case err := <-feedErr:
			require.NoError(t, err)
		}

		err = adapter.TestingDeleteAll(ctx)
		require.NoError(t, err)

		select {
		case change := <-changes:
			require.Equal(t, "objects", change.TableName)
			require.Equal(t, "DELETE", change.ModType)
		case err := <-feedErr:
			require.NoError(t, err)
		}

		cancel()
		err = <-feedErr
		if spanner.ErrCode(err) != codes.Canceled {
			require.NoError(t, errs2.IgnoreCanceled(err))
		}

		err = changeStreamAdapter.TestDeleteChangeStream(ctx, changefeedName)
		require.NoError(t, err)
	})
}
