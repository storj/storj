// Copyright (C) 2025 Storj Labs, Inc.
// See LICENSE for copying information.

package metainfo

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap/zaptest"

	"storj.io/common/testcontext"
	"storj.io/common/uuid"
	"storj.io/storj/satellite/buckets"
	"storj.io/storj/satellite/eventing"
	"storj.io/storj/satellite/eventing/eventingconfig"
)

func TestEndpoint_ShouldTransmitEvent(t *testing.T) {
	ctx := testcontext.New(t)
	defer ctx.Cleanup()

	projectID := uuid.UUID{1, 2, 3, 4, 5}
	bucketName := "test-bucket"
	objectKey := []byte("path/to/object.jpg")

	t.Run("no bucket config", func(t *testing.T) {
		endpoint := setupEndpointForEventing(t, eventingTestConfig{
			bucketConfig: nil, // No config
		})

		result := endpoint.shouldTransmitEvent(ctx, projectID, bucketName, objectKey, eventing.EventTypeObjectCreatedPut)
		assert.False(t, result, "should return false when no bucket config exists")
	})

	t.Run("event type does not match", func(t *testing.T) {
		endpoint := setupEndpointForEventing(t, eventingTestConfig{
			bucketConfig: &buckets.NotificationConfig{
				Events: []string{eventing.EventTypeObjectCreatedCopy}, // Different event type
			},
		})

		result := endpoint.shouldTransmitEvent(ctx, projectID, bucketName, objectKey, eventing.EventTypeObjectCreatedPut)
		assert.False(t, result, "should return false when event type doesn't match")
	})

	t.Run("object key does not match prefix filter", func(t *testing.T) {
		endpoint := setupEndpointForEventing(t, eventingTestConfig{
			bucketConfig: &buckets.NotificationConfig{
				Events:       []string{eventing.EventTypeObjectCreatedPut},
				FilterPrefix: []byte("images/"), // Object key doesn't start with this
			},
		})

		result := endpoint.shouldTransmitEvent(ctx, projectID, bucketName, objectKey, eventing.EventTypeObjectCreatedPut)
		assert.False(t, result, "should return false when object key doesn't match prefix filter")
	})

	t.Run("object key does not match suffix filter", func(t *testing.T) {
		endpoint := setupEndpointForEventing(t, eventingTestConfig{
			bucketConfig: &buckets.NotificationConfig{
				Events:       []string{eventing.EventTypeObjectCreatedPut},
				FilterSuffix: []byte(".png"), // Object key doesn't end with this
			},
		})

		result := endpoint.shouldTransmitEvent(ctx, projectID, bucketName, objectKey, eventing.EventTypeObjectCreatedPut)
		assert.False(t, result, "should return false when object key doesn't match suffix filter")
	})

	t.Run("all conditions met - exact event match", func(t *testing.T) {
		endpoint := setupEndpointForEventing(t, eventingTestConfig{
			bucketConfig: &buckets.NotificationConfig{
				Events:       []string{eventing.EventTypeObjectCreatedPut},
				FilterPrefix: []byte("path/"),
				FilterSuffix: []byte(".jpg"),
			},
		})

		result := endpoint.shouldTransmitEvent(ctx, projectID, bucketName, objectKey, eventing.EventTypeObjectCreatedPut)
		assert.True(t, result, "should return true when all conditions are met")
	})

	t.Run("all conditions met - wildcard event match", func(t *testing.T) {
		endpoint := setupEndpointForEventing(t, eventingTestConfig{
			bucketConfig: &buckets.NotificationConfig{
				Events:       []string{eventing.EventTypeObjectCreatedAll},
				FilterPrefix: []byte("path/"),
				FilterSuffix: []byte(".jpg"),
			},
		})

		result := endpoint.shouldTransmitEvent(ctx, projectID, bucketName, objectKey, eventing.EventTypeObjectCreatedPut)
		assert.True(t, result, "should return true with wildcard event match")
	})

	t.Run("all conditions met - no filters", func(t *testing.T) {
		endpoint := setupEndpointForEventing(t, eventingTestConfig{
			bucketConfig: &buckets.NotificationConfig{
				Events: []string{eventing.EventTypeObjectCreatedPut},
				// No filters - all object keys should match
			},
		})

		result := endpoint.shouldTransmitEvent(ctx, projectID, bucketName, objectKey, eventing.EventTypeObjectCreatedPut)
		assert.True(t, result, "should return true when no filters are specified")
	})

	t.Run("fail-safe mode - cache returns error", func(t *testing.T) {
		endpoint := setupEndpointForEventing(t, eventingTestConfig{
			bucketConfigErr: Error.New("cache error"),
		})

		result := endpoint.shouldTransmitEvent(ctx, projectID, bucketName, objectKey, eventing.EventTypeObjectCreatedPut)
		assert.True(t, result, "should return true (fail-safe) when cache returns error")
	})

	t.Run("empty object key with no filters", func(t *testing.T) {
		endpoint := setupEndpointForEventing(t, eventingTestConfig{
			bucketConfig: &buckets.NotificationConfig{
				Events: []string{eventing.EventTypeObjectCreatedPut},
			},
		})

		result := endpoint.shouldTransmitEvent(ctx, projectID, bucketName, []byte{}, eventing.EventTypeObjectCreatedPut)
		assert.True(t, result, "should return true for empty object key with no filters")
	})

	t.Run("empty object key with prefix filter", func(t *testing.T) {
		endpoint := setupEndpointForEventing(t, eventingTestConfig{
			bucketConfig: &buckets.NotificationConfig{
				Events:       []string{eventing.EventTypeObjectCreatedPut},
				FilterPrefix: []byte("prefix/"),
			},
		})

		result := endpoint.shouldTransmitEvent(ctx, projectID, bucketName, []byte{}, eventing.EventTypeObjectCreatedPut)
		assert.False(t, result, "should return false when empty object key doesn't match prefix filter")
	})

	t.Run("multiple event types configured", func(t *testing.T) {
		endpoint := setupEndpointForEventing(t, eventingTestConfig{
			bucketConfig: &buckets.NotificationConfig{
				Events: []string{
					eventing.EventTypeObjectCreatedPut,
					eventing.EventTypeObjectCreatedCopy,
					eventing.EventTypeObjectRemovedDelete,
				},
			},
		})

		// Test each event type
		assert.True(t, endpoint.shouldTransmitEvent(ctx, projectID, bucketName, objectKey, eventing.EventTypeObjectCreatedPut))
		assert.True(t, endpoint.shouldTransmitEvent(ctx, projectID, bucketName, objectKey, eventing.EventTypeObjectCreatedCopy))
		assert.True(t, endpoint.shouldTransmitEvent(ctx, projectID, bucketName, objectKey, eventing.EventTypeObjectRemovedDelete))
		// Test an event type that isn't in the list
		assert.False(t, endpoint.shouldTransmitEvent(ctx, projectID, bucketName, objectKey, eventing.EventTypeObjectRemovedDeleteMarkerCreated))
	})

	t.Run("multiple event types passed - at least one matches", func(t *testing.T) {
		endpoint := setupEndpointForEventing(t, eventingTestConfig{
			bucketConfig: &buckets.NotificationConfig{
				Events: []string{eventing.EventTypeObjectRemovedDelete},
			},
		})

		// Pass both delete event types - should return true because one matches
		result := endpoint.shouldTransmitEvent(ctx, projectID, bucketName, objectKey,
			eventing.EventTypeObjectRemovedDelete, eventing.EventTypeObjectRemovedDeleteMarkerCreated)
		assert.True(t, result, "should return true when at least one event type matches")
	})

	t.Run("multiple event types passed - none match", func(t *testing.T) {
		endpoint := setupEndpointForEventing(t, eventingTestConfig{
			bucketConfig: &buckets.NotificationConfig{
				Events: []string{eventing.EventTypeObjectCreatedPut},
			},
		})

		// Pass both delete event types - should return false because neither matches
		result := endpoint.shouldTransmitEvent(ctx, projectID, bucketName, objectKey,
			eventing.EventTypeObjectRemovedDelete, eventing.EventTypeObjectRemovedDeleteMarkerCreated)
		assert.False(t, result, "should return false when no event types match")
	})

	t.Run("multiple event types passed - wildcard matches all", func(t *testing.T) {
		endpoint := setupEndpointForEventing(t, eventingTestConfig{
			bucketConfig: &buckets.NotificationConfig{
				Events: []string{eventing.EventTypeObjectRemovedAll},
			},
		})

		// Pass both delete event types - should return true because wildcard matches both
		result := endpoint.shouldTransmitEvent(ctx, projectID, bucketName, objectKey,
			eventing.EventTypeObjectRemovedDelete, eventing.EventTypeObjectRemovedDeleteMarkerCreated)
		assert.True(t, result, "should return true when wildcard matches multiple event types")
	})
}

// eventingTestConfig holds configuration for setting up test endpoints.
type eventingTestConfig struct {
	bucketConfig    *buckets.NotificationConfig
	bucketConfigErr error
}

// setupEndpointForEventing creates a test endpoint configured for eventing tests.
func setupEndpointForEventing(t *testing.T, cfg eventingTestConfig) *Endpoint {
	log := zaptest.NewLogger(t)

	// Create mock buckets DB
	bucketsDB := &mockBucketsDB{
		config: cfg.bucketConfig,
		err:    cfg.bucketConfigErr,
	}

	// Create buckets service wrapping the mock DB
	bucketsService := &buckets.Service{
		DB: bucketsDB,
	}

	eventingConfig := eventingconfig.Config{
		Cache: eventingconfig.CacheConfig{
			TTL:      time.Minute,
			Capacity: 100,
		},
	}

	cache, err := eventing.NewConfigCache(bucketsDB, eventingConfig)
	require.NoError(t, err)

	return &Endpoint{
		log:                 log,
		buckets:             bucketsService,
		bucketEventingCache: cache,
	}
}

// mockBucketsDB implements the buckets.DB interface for testing.
type mockBucketsDB struct {
	buckets.DB // Embed to satisfy interface

	config *buckets.NotificationConfig
	err    error
}

func (m *mockBucketsDB) GetBucketNotificationConfig(ctx context.Context, bucketName []byte, projectID uuid.UUID) (*buckets.NotificationConfig, error) {
	if m.err != nil {
		return nil, m.err
	}
	return m.config, nil
}
