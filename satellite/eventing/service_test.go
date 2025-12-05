// Copyright (C) 2025 Storj Labs, Inc.
// See LICENSE for copying information.

package eventing_test

import (
	"context"
	"encoding/json"
	"os"
	"testing"

	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
	"go.uber.org/zap/zaptest/observer"

	"storj.io/common/testcontext"
	"storj.io/common/testrand"
	"storj.io/common/uuid"
	"storj.io/storj/private/testplanet"
	"storj.io/storj/satellite/eventing"
	"storj.io/storj/satellite/eventing/eventingconfig"
	"storj.io/storj/satellite/metabase"
	"storj.io/storj/satellite/metabase/changestream"
	"storj.io/storj/satellite/metabase/metabasetest"
	"storj.io/storj/shared/dbutil"
)

var TestPublicProjectID = uuid.UUID([16]byte{0x1d, 0x2e, 0x3f, 0x4c, 0x5b, 0x6a, 0x7d, 0x8e, 0x9f, 0xa0, 0xb1, 0xc2, 0xd3, 0xe4, 0xf5, 0x06})

type TestPublicProjectIDs struct{}

func (p *TestPublicProjectIDs) GetPublicID(ctx context.Context, id uuid.UUID) (uuid.UUID, error) {
	return TestPublicProjectID, nil
}

var _ eventing.PublicProjectIDer = &TestPublicProjectIDs{}

func TestProcessRecord(t *testing.T) {
	var r changestream.DataChangeRecord
	raw, err := os.ReadFile("./testdata/commit-object-insert.json")
	require.NoError(t, err)
	err = json.Unmarshal(raw, &r)
	require.NoError(t, err)

	metabasetest.Run(t, func(ctx *testcontext.Context, t *testing.T, db *metabase.DB) {
		// Skip test if not using Spanner
		if db.Implementation() != dbutil.Spanner {
			t.Skip("test requires Spanner")
		}

		adapter := db.ChooseAdapter(testrand.UUID()).(*metabase.SpannerAdapter)

		observedZapCore, observedLogs := observer.New(zap.DebugLevel)
		observedLogger := zap.New(observedZapCore).Named("publisher")

		service, err := eventing.NewService(testplanet.NewLogger(t), adapter, &TestPublicProjectIDs{}, eventingconfig.Config{
			Buckets: eventingconfig.BucketLocationTopicIDMap{
				metabase.BucketLocation{
					ProjectID:  eventing.TestProjectID,
					BucketName: metabase.BucketName(eventing.TestBucket),
				}: "projects/testproject/topics/testtopic",
			},
		}, eventing.Config{
			TestNewPublisherFn: func() (eventing.EventPublisher, error) {
				return eventing.NewLogPublisher(observedLogger), nil
			},
		})
		require.NoError(t, err)

		err = service.ProcessRecord(ctx, r)
		require.NoError(t, err)

		// Check that the event was published
		publishLog := observedLogs.FilterMessage("Publishing event")
		require.Equal(t, 1, publishLog.Len())

		// Find the event field in the log entry and check if the project ID was replaced with the public ID
		eventField, ok := publishLog.All()[0].ContextMap()["event"]
		require.True(t, ok, "event field not found in log entry")
		event, ok := eventField.(eventing.Event)
		require.True(t, ok, "event field is not a changestream.Event")
		record := event.Records[0]
		require.Len(t, event.Records, 1)
		require.Equal(t, TestPublicProjectID.String(), record.S3.Bucket.OwnerIdentity.PrincipalId)
	})
}

func TestProcessRecord_NoMatchingBucket(t *testing.T) {
	var r changestream.DataChangeRecord
	raw, err := os.ReadFile("./testdata/commit-object-insert.json")
	require.NoError(t, err)
	err = json.Unmarshal(raw, &r)
	require.NoError(t, err)

	metabasetest.Run(t, func(ctx *testcontext.Context, t *testing.T, db *metabase.DB) {
		// Skip test if not using Spanner
		if db.Implementation() != dbutil.Spanner {
			t.Skip("test requires Spanner")
		}

		adapter := db.ChooseAdapter(testrand.UUID()).(*metabase.SpannerAdapter)

		observedZapCore, observedLogs := observer.New(zap.DebugLevel)
		observedLogger := zap.New(observedZapCore).Named("eventing")

		service, err := eventing.NewService(observedLogger, adapter, &TestPublicProjectIDs{}, eventingconfig.Config{
			Buckets: eventingconfig.BucketLocationTopicIDMap{
				metabase.BucketLocation{
					ProjectID:  testrand.UUID(),
					BucketName: metabase.BucketName(testrand.BucketName()),
				}: "projects/testproject/topics/testtopic",
			},
		}, eventing.Config{
			TestNewPublisherFn: func() (eventing.EventPublisher, error) {
				return eventing.NewLogPublisher(observedLogger), nil
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

		adapter := db.ChooseAdapter(testrand.UUID()).(*metabase.SpannerAdapter)

		_, err := eventing.NewService(testplanet.NewLogger(t), adapter, &TestPublicProjectIDs{}, eventingconfig.Config{
			Buckets: eventingconfig.BucketLocationTopicIDMap{
				metabase.BucketLocation{
					ProjectID:  eventing.TestProjectID,
					BucketName: metabase.BucketName(eventing.TestBucket),
				}: "invalid/topic/name",
			},
		}, eventing.Config{})
		require.Error(t, err)
	})
}
