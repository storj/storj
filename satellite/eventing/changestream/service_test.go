// Copyright (C) 2025 Storj Labs, Inc.
// See LICENSE for copying information.

package changestream_test

import (
	"encoding/json"
	"os"
	"testing"

	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
	"go.uber.org/zap/zaptest/observer"

	"storj.io/common/testcontext"
	"storj.io/common/testrand"
	"storj.io/storj/private/testplanet"
	"storj.io/storj/satellite/eventing"
	"storj.io/storj/satellite/eventing/changestream"
	"storj.io/storj/satellite/metabase"
	"storj.io/storj/satellite/metabase/metabasetest"
	"storj.io/storj/shared/dbutil"
)

func TestProcessRecord(t *testing.T) {
	var r metabase.DataChangeRecord
	raw, err := os.ReadFile("./testdata/insert.json")
	require.NoError(t, err)
	err = json.Unmarshal(raw, &r)
	require.NoError(t, err)

	metabasetest.Run(t, func(ctx *testcontext.Context, t *testing.T, db *metabase.DB) {
		// Skip test if not using Spanner
		if db.Implementation() != dbutil.Spanner {
			t.Skip("test requires Spanner")
		}

		adapter := db.ChooseAdapter(testrand.UUID())

		observedZapCore, observedLogs := observer.New(zap.DebugLevel)
		observedLogger := zap.New(observedZapCore).Named("publisher")

		service, err := changestream.NewService(adapter, testplanet.NewLogger(t), changestream.Config{
			Buckets: eventing.BucketLocationTopicIDMap{
				changestream.TestBucket: "projects/testproject/topics/testtopic",
			},
			TestNewPublisherFn: func() (changestream.EventPublisher, error) {
				return changestream.NewLogPublisher(observedLogger), nil
			},
		})
		require.NoError(t, err)

		err = service.ProcessRecord(ctx, r)
		require.NoError(t, err)
		require.Equal(t, 1, observedLogs.FilterMessage("Publishing event").Len())
	})
}

func TestProcessRecord_NoMatchingBucket(t *testing.T) {
	var r metabase.DataChangeRecord
	raw, err := os.ReadFile("./testdata/insert.json")
	require.NoError(t, err)
	err = json.Unmarshal(raw, &r)
	require.NoError(t, err)

	metabasetest.Run(t, func(ctx *testcontext.Context, t *testing.T, db *metabase.DB) {
		// Skip test if not using Spanner
		if db.Implementation() != dbutil.Spanner {
			t.Skip("test requires Spanner")
		}

		adapter := db.ChooseAdapter(testrand.UUID())

		observedZapCore, observedLogs := observer.New(zap.DebugLevel)
		observedLogger := zap.New(observedZapCore).Named("eventing")

		service, err := changestream.NewService(adapter, observedLogger, changestream.Config{
			Buckets: eventing.BucketLocationTopicIDMap{
				metabase.BucketLocation{
					ProjectID:  testrand.UUID(),
					BucketName: metabase.BucketName(testrand.BucketName()),
				}: "projects/testproject/topics/testtopic",
			},
			TestNewPublisherFn: func() (changestream.EventPublisher, error) {
				return changestream.NewLogPublisher(observedLogger), nil
			},
		})
		require.NoError(t, err)

		err = service.ProcessRecord(ctx, r)
		require.Error(t, err)
		require.Equal(t, 1, observedLogs.FilterMessage("Failed to get publisher for bucket").Len())
		require.Zero(t, observedLogs.FilterMessage("Publishing event").Len())
	})
}

func TestProcessRecord_InvalidTopicName(t *testing.T) {
	metabasetest.Run(t, func(ctx *testcontext.Context, t *testing.T, db *metabase.DB) {
		// Skip test if not using Spanner
		if db.Implementation() != dbutil.Spanner {
			t.Skip("test requires Spanner")
		}

		adapter := db.ChooseAdapter(testrand.UUID())

		_, err := changestream.NewService(adapter, testplanet.NewLogger(t), changestream.Config{
			Buckets: eventing.BucketLocationTopicIDMap{
				changestream.TestBucket: "invalid/topic/name",
			},
		})
		require.Error(t, err)
	})
}
