// Copyright (C) 2026 Storj Labs, Inc.
// See LICENSE for copying information.

// Package s3event defines S3-compatible bucket event types.
package s3event

// Event represents an S3-compatible bucket event type.
type Event string

const (
	eventCategoryObjectCreated = "ObjectCreated"
	eventCategoryObjectRemoved = "ObjectRemoved"
)

// S3-compatible bucket event types.
const (
	ObjectCreatedPut                 Event = eventCategoryObjectCreated + ":Put"
	ObjectCreatedCopy                Event = eventCategoryObjectCreated + ":Copy"
	ObjectRemovedDelete              Event = eventCategoryObjectRemoved + ":Delete"
	ObjectRemovedDeleteMarkerCreated Event = eventCategoryObjectRemoved + ":DeleteMarkerCreated"
	ObjectCreatedAll                 Event = eventCategoryObjectCreated + ":*"
	ObjectRemovedAll                 Event = eventCategoryObjectRemoved + ":*"
)

// Name returns the event name without the "s3:" prefix, as used in published event records.
func (e Event) Name() string {
	return string(e)
}

// S3Name returns the event name with the "s3:" prefix, as used in bucket notification configuration.
func (e Event) S3Name() string {
	return "s3:" + string(e)
}
