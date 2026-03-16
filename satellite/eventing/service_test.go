// Copyright (C) 2025 Storj Labs, Inc.
// See LICENSE for copying information.

package eventing_test

import (
	"context"
	"encoding/json"
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
	"go.uber.org/zap/zaptest/observer"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"storj.io/common/testcontext"
	"storj.io/common/testrand"
	"storj.io/common/uuid"
	"storj.io/storj/private/testplanet"
	"storj.io/storj/private/testredis"
	"storj.io/storj/satellite/buckets"
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

var _ eventing.PublicProjectIDGetter = &TestPublicProjectIDs{}

type TestBucketsDB struct {
	buckets.DB
	config *buckets.NotificationConfig
}

func (db *TestBucketsDB) GetBucketNotificationConfig(ctx context.Context, bucketName []byte, projectID uuid.UUID) (*buckets.NotificationConfig, error) {
	return db.config, nil
}

func TestProcessRecord(t *testing.T) {
	raw, err := os.ReadFile("./testdata/commit-object-insert.json")
	require.NoError(t, err)

	metabasetest.Run(t, func(ctx *testcontext.Context, t *testing.T, db *metabase.DB) {
		// Skip test if not using Spanner
		if db.Implementation() != dbutil.Spanner {
			t.Skip("test requires Spanner")
		}

		adapter := db.ChooseAdapter(testrand.UUID()).(*metabase.SpannerAdapter)

		// Create cache with testredis - shared across all subtests
		redis, err := testredis.Mini(ctx)
		require.NoError(t, err)
		defer ctx.Check(redis.Close)

		setupTest := func(t *testing.T, config *buckets.NotificationConfig) (*eventing.Service, *observer.ObservedLogs) {
			observedZapCore, observedLogs := observer.New(zap.DebugLevel)
			observedLogger := zap.New(observedZapCore).Named("publisher")

			bucketsDB := &TestBucketsDB{config: config}

			cache, err := eventing.NewConfigCache(testplanet.NewLogger(t), bucketsDB, eventingconfig.Config{
				Cache: eventingconfig.CacheConfig{
					Address: "redis://" + redis.Addr(),
					TTL:     time.Hour,
				},
			})
			require.NoError(t, err)
			defer func() { _ = cache.Close() }()

			service := eventing.NewService(testplanet.NewLogger(t), adapter, cache, &TestPublicProjectIDs{}, eventing.Config{
				TestNewPublisherFn: func() (eventing.Publisher, error) {
					return eventing.NewLogPublisher(observedLogger), nil
				},
			})

			return service, observedLogs
		}

		t.Run("with wildcard event match", func(t *testing.T) {
			service, observedLogs := setupTest(t, &buckets.NotificationConfig{
				ConfigID:  "TestConfigId",
				TopicName: "projects/testproject/topics/testtopic",
				Events:    []string{"s3:ObjectCreated:*"},
			})

			var r changestream.DataChangeRecord
			err = json.Unmarshal(raw, &r)
			require.NoError(t, err)

			_, err := service.ProcessRecord(ctx, r)
			require.NoError(t, err)

			// Check that the event was published
			publishLog := observedLogs.FilterMessage("Publishing event")
			require.Equal(t, 1, publishLog.Len())

			// Find the data field in the log entry and unmarshal it
			dataField, ok := publishLog.All()[0].ContextMap()["data"]
			require.True(t, ok, "data field not found in log entry")
			dataStr, ok := dataField.(string)
			require.True(t, ok, "data field is not string")
			var event eventing.Event
			require.NoError(t, json.Unmarshal([]byte(dataStr), &event))
			require.Len(t, event.Records, 1)
			record := event.Records[0]

			// Check if the project ID was replaced with the public ID
			require.Equal(t, TestPublicProjectID.String(), record.S3.Bucket.OwnerIdentity.PrincipalId)

			// Check if configuration id was set with the one from the bucket configuration
			require.Equal(t, "TestConfigId", record.S3.ConfigurationId)
		})

		t.Run("with specific event type", func(t *testing.T) {
			service, observedLogs := setupTest(t, &buckets.NotificationConfig{
				TopicName: "projects/testproject/topics/testtopic",
				Events:    []string{"s3:ObjectCreated:Put"},
			})

			var r changestream.DataChangeRecord
			err = json.Unmarshal(raw, &r)
			require.NoError(t, err)

			_, err := service.ProcessRecord(ctx, r)
			require.NoError(t, err)

			// Check that the event was published
			publishLog := observedLogs.FilterMessage("Publishing event")
			require.Equal(t, 1, publishLog.Len())
		})

		t.Run("with non-matching event type", func(t *testing.T) {
			service, observedLogs := setupTest(t, &buckets.NotificationConfig{
				TopicName: "projects/testproject/topics/testtopic",
				Events:    []string{"s3:ObjectRemoved:*"}, // Different event type
			})

			var r changestream.DataChangeRecord
			err = json.Unmarshal(raw, &r)
			require.NoError(t, err)

			_, err := service.ProcessRecord(ctx, r)
			require.NoError(t, err)

			// Event should not be published because event type doesn't match
			publishLog := observedLogs.FilterMessage("Publishing event")
			require.Zero(t, publishLog.Len())
		})

		t.Run("with multiple event types", func(t *testing.T) {
			service, observedLogs := setupTest(t, &buckets.NotificationConfig{
				TopicName: "projects/testproject/topics/testtopic",
				Events: []string{
					"s3:ObjectCreated:Put",
					"s3:ObjectCreated:Copy",
					"s3:ObjectRemoved:Delete",
				},
			})

			var r changestream.DataChangeRecord
			err = json.Unmarshal(raw, &r)
			require.NoError(t, err)

			_, err := service.ProcessRecord(ctx, r)
			require.NoError(t, err)

			// Check that the event was published
			publishLog := observedLogs.FilterMessage("Publishing event")
			require.Equal(t, 1, publishLog.Len())
		})

		t.Run("with no notification config", func(t *testing.T) {
			service, observedLogs := setupTest(t, nil)

			var r changestream.DataChangeRecord
			err = json.Unmarshal(raw, &r)
			require.NoError(t, err)

			_, err := service.ProcessRecord(ctx, r)
			require.NoError(t, err)

			// No event should be published when no config exists
			publishLog := observedLogs.FilterMessage("Publishing event")
			require.Zero(t, publishLog.Len())
		})

	})
}

func TestProcessRecord_PublisherUserConfigError(t *testing.T) {
	raw, err := os.ReadFile("./testdata/commit-object-insert.json")
	require.NoError(t, err)

	metabasetest.Run(t, func(ctx *testcontext.Context, t *testing.T, db *metabase.DB) {
		// Skip test if not using Spanner
		if db.Implementation() != dbutil.Spanner {
			t.Skip("test requires Spanner")
		}

		adapter := db.ChooseAdapter(testrand.UUID()).(*metabase.SpannerAdapter)

		redis, err := testredis.Mini(ctx)
		require.NoError(t, err)
		defer ctx.Check(redis.Close)

		bucketsDB := &TestBucketsDB{config: &buckets.NotificationConfig{
			TopicName: "projects/testproject/topics/testtopic",
			Events:    []string{"s3:ObjectCreated:*"},
		}}

		cache, err := eventing.NewConfigCache(testplanet.NewLogger(t), bucketsDB, eventingconfig.Config{
			Cache: eventingconfig.CacheConfig{
				Address: "redis://" + redis.Addr(),
				TTL:     time.Hour,
			},
		})
		require.NoError(t, err)
		defer func() { _ = cache.Close() }()

		service := eventing.NewService(testplanet.NewLogger(t), adapter, cache, &TestPublicProjectIDs{}, eventing.Config{
			TestNewPublisherFn: func() (eventing.Publisher, error) {
				return nil, status.Error(codes.PermissionDenied, "missing IAM permission")
			},
		})

		var r changestream.DataChangeRecord
		require.NoError(t, json.Unmarshal(raw, &r))

		// User config error in GetPublisher should be silently dropped — no error returned.
		result, err := service.ProcessRecord(ctx, r)
		require.NoError(t, err)
		require.NotNil(t, result)

		// Result should be immediately ready (ImmediateResult).
		select {
		case <-result.Ready():
		default:
			t.Fatal("expected result to be immediately ready")
		}
		require.NoError(t, result.Get(ctx))
	})
}

func TestGetPublisher_TopicChange(t *testing.T) {
	ctx := testcontext.New(t)
	defer ctx.Cleanup()

	observedZapCore, observedLogs := observer.New(zap.InfoLevel)
	observedLogger := zap.New(observedZapCore).Named("eventing")

	bucketsDB := &TestBucketsDB{}

	// Create cache with testredis
	redis, err := testredis.Mini(ctx)
	require.NoError(t, err)
	defer ctx.Check(redis.Close)

	cache, err := eventing.NewConfigCache(testplanet.NewLogger(t), bucketsDB, eventingconfig.Config{
		Cache: eventingconfig.CacheConfig{
			Address: "redis://" + redis.Addr(),
			TTL:     time.Hour,
		},
	})
	require.NoError(t, err)
	defer func() { _ = cache.Close() }()

	// Track how many times the publisher factory is called
	publisherCallCount := 0

	service := eventing.NewService(observedLogger, nil, cache, &TestPublicProjectIDs{}, eventing.Config{
		TestNewPublisherFn: func() (eventing.Publisher, error) {
			publisherCallCount++
			return eventing.NewLogPublisher(observedLogger), nil
		},
	})

	// Get publisher with @log topic (LogPublisher always returns "@log" as topic name)
	topic1 := "@log"
	publisher1, err := service.GetPublisher(ctx, eventing.TestProjectID, TestPublicProjectID, eventing.TestBucket, topic1)
	require.NoError(t, err)
	require.NotNil(t, publisher1)
	assert.Equal(t, "@log", publisher1.TopicName())
	assert.Equal(t, 1, publisherCallCount, "should create publisher once")

	// Get publisher again with same topic - should return cached instance
	publisher1Again, err := service.GetPublisher(ctx, eventing.TestProjectID, TestPublicProjectID, eventing.TestBucket, topic1)
	require.NoError(t, err)
	assert.Same(t, publisher1, publisher1Again, "should return same cached publisher instance")
	assert.Equal(t, 1, publisherCallCount, "should not create new publisher")
	assert.Zero(t, observedLogs.FilterMessage("Topic name changed for bucket, closing old publisher").Len(), "should not log topic change")

	// Get publisher with different topic - should invalidate cache and create new publisher
	// Even though TestNewPublisherFn ignores the topic parameter, the cache invalidation
	// logic should still detect the mismatch and close/recreate the publisher
	topic2 := "projects/test-project/topics/topic2"
	publisher2, err := service.GetPublisher(ctx, eventing.TestProjectID, TestPublicProjectID, eventing.TestBucket, topic2)
	require.NoError(t, err)
	require.NotNil(t, publisher2)
	assert.NotSame(t, publisher1, publisher2, "should create new publisher instance")
	assert.Equal(t, 2, publisherCallCount, "should create new publisher for different topic")
	assert.Equal(t, 1, observedLogs.FilterMessage("Topic name changed for bucket, closing old publisher").Len(), "should log topic change once")
}
