// Copyright (C) 2025 Storj Labs, Inc.
// See LICENSE for copying information.

package metainfo

import (
	"context"

	"go.uber.org/zap"

	"storj.io/common/uuid"
	"storj.io/storj/satellite/eventing"
)

// shouldTransmitEvent determines whether to generate change stream events for a bucket operation.
// It checks:
// 1. Bucket notification configuration exists
// 2. At least one of the provided event types matches configuration
// 3. Object key matches filter rules (prefix/suffix)
// Returns true if all conditions are met, false otherwise.
func (endpoint *Endpoint) shouldTransmitEvent(ctx context.Context, projectID uuid.UUID, bucketName string, objectKey []byte, eventTypes ...string) bool {
	// Get notification configuration (cache handles database fallback internally)
	config, err := endpoint.bucketEventingCache.GetBucketNotificationConfig(ctx, []byte(bucketName), projectID)
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

	// Check if object key matches filter rules.
	// Filter prefix/suffix are stored URL-encoded (per S3 spec), so encode the key to match.
	if !eventing.MatchFilters(eventing.EncodeForS3Event(objectKey), config.FilterPrefix, config.FilterSuffix) {
		return false
	}

	return true
}
