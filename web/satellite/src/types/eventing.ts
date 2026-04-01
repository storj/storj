// Copyright (C) 2026 Storj Labs, Inc.
// See LICENSE for copying information.

// Event types matching satellite/eventing/notification.go
export enum EventType {
    ObjectCreatedPut = 's3:ObjectCreated:Put',
    ObjectCreatedCopy = 's3:ObjectCreated:Copy',
    ObjectCreatedAll = 's3:ObjectCreated:*',
    ObjectRemovedDelete = 's3:ObjectRemoved:Delete',
    ObjectRemovedDeleteMarkerCreated = 's3:ObjectRemoved:DeleteMarkerCreated',
    ObjectRemovedAll = 's3:ObjectRemoved:*',
}

// Notification configuration (maps to AWS S3 TopicConfiguration)
export interface BucketNotificationConfig {
    id?: string;           // Optional unique identifier
    topicArn: string;      // arn:gcp:pubsub::PROJECT_ID:TOPIC_ID
    events: EventType[];   // Selected event types
    filterPrefix?: string; // Object key prefix filter
    filterSuffix?: string; // Object key suffix filter
}

// Full notification configuration (from API)
export interface BucketNotificationConfiguration {
    topicConfigurations?: BucketNotificationConfig[];
}
