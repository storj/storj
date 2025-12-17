// Copyright (C) 2025 Storj Labs, Inc.
// See LICENSE for copying information.

package metainfo

import (
	"context"

	"go.uber.org/zap"

	"storj.io/common/uuid"
	"storj.io/storj/satellite/buckets"
	"storj.io/storj/satellite/eventing"
)

// shouldTransmitEvent determines whether to generate change stream events for a bucket operation.
// It checks:
// 1. Project-level gating (Projects.Enabled) - must be enabled
// 2. Bucket notification configuration exists
// 3. At least one of the provided event types matches configuration
// 4. Object key matches filter rules (prefix/suffix)
// Returns true if all conditions are met, false otherwise.
func (endpoint *Endpoint) shouldTransmitEvent(ctx context.Context, projectID uuid.UUID, bucketName string, objectKey []byte, eventTypes ...string) bool {
	// Check project-level gating first
	if !endpoint.bucketEventing.Projects.Enabled(projectID) {
		return false
	}

	// Get notification configuration (cache handles database fallback internally)
	var config *buckets.NotificationConfig
	var err error
	if endpoint.bucketEventingCache != nil {
		config, err = endpoint.bucketEventingCache.GetBucketNotificationConfig(ctx, []byte(bucketName), projectID)
	} else {
		// Cache disabled - query database directly
		config, err = endpoint.buckets.GetBucketNotificationConfig(ctx, []byte(bucketName), projectID)
	}
	if err != nil {
		// Fail-safe mode: if both cache and database fail, return true to let eventing service decide
		endpoint.log.Warn("Failed to get bucket notification config, failing safe (TransmitEvent=true)",
			zap.Stringer("project_id", projectID),
			zap.String("bucket_name", bucketName),
			zap.Error(err))
		return true
	}

	if config == nil {
		// No configuration exists
		return false
	}

	// Check if at least one event type matches
	anyMatches := false
	for _, eventType := range eventTypes {
		if eventing.MatchEventType(eventType, config.Events) {
			anyMatches = true
			break
		}
	}
	if !anyMatches {
		return false
	}

	// Check if object key matches filter rules
	if !eventing.MatchFilters(objectKey, config.FilterPrefix, config.FilterSuffix) {
		return false
	}

	return true
}
