// Copyright (C) 2025 Storj Labs, Inc.
// See LICENSE for copying information.

package eventing

import (
	"context"
	"sync"

	"github.com/zeebo/errs"
	"go.uber.org/zap"

	"storj.io/common/uuid"
	"storj.io/eventkit"
	"storj.io/storj/satellite/buckets"
	"storj.io/storj/satellite/metabase"
)

var ek = eventkit.Package()

// Config holds configuration for the changestream service.
type Config struct {
	Feedname string `help:"the (spanner) name of the changestream to listen on" default:"bucket_eventing"`

	TestNewPublisherFn func() (Publisher, error) `noflag:"true"`
}

// PublicProjectIDGetter is an interface for looking up public project IDs.
type PublicProjectIDGetter interface {
	// GetPublicID returns the public project ID for a given project ID.
	GetPublicID(ctx context.Context, id uuid.UUID) (uuid.UUID, error)
}

// BucketNotificationConfigGetter is an interface for retrieving bucket notification configurations.
type BucketNotificationConfigGetter interface {
	// GetBucketNotificationConfig retrieves the notification configuration for a bucket.
	// Returns nil if no configuration exists.
	GetBucketNotificationConfig(ctx context.Context, bucketName []byte, projectID uuid.UUID) (*buckets.NotificationConfig, error)
}

// Service implements a changestream processing service.
type Service struct {
	log        *zap.Logger
	source     EventSource
	buckets    BucketNotificationConfigGetter
	projects   PublicProjectIDGetter
	cfg        Config
	publishers map[metabase.BucketLocation]Publisher
	mu         sync.RWMutex
}

// NewService creates a new changestream service.
func NewService(log *zap.Logger, source EventSource, buckets BucketNotificationConfigGetter, projects PublicProjectIDGetter, cfg Config) *Service {
	return &Service{
		log:        log,
		source:     source,
		buckets:    buckets,
		projects:   projects,
		cfg:        cfg,
		publishers: make(map[metabase.BucketLocation]Publisher),
	}
}

// Run starts the event source processing loop.
func (s *Service) Run(ctx context.Context) (err error) {
	defer mon.Task()(&ctx)(&err)

	return s.source.Listen(ctx, func(event ChangeEvent) (PendingResult, error) {
		return s.ProcessEvent(ctx, event)
	})
}

// ProcessEvent processes a single ChangeEvent and returns a
// PendingResult whose Get method blocks until the event is confirmed delivered.
// Skipped events (no config, filtered out, etc.) return a pre-resolved result.
func (s *Service) ProcessEvent(ctx context.Context, event ChangeEvent) (_ PendingResult, err error) {
	defer mon.Task()(&ctx)(&err)

	// Resolve the private project ID to the public project ID.
	projectPublicID, err := s.projects.GetPublicID(ctx, event.ProjectID)
	if err != nil {
		s.log.Error("Failed to get public project ID", zap.Error(err))
		return nil, err
	}

	bucketName := string(event.BucketName)
	eventName := event.EventName
	commitTimestamp := event.CommitTimestamp

	s.log.Debug("Received change event",
		zap.Stringer("project_public_id", projectPublicID),
		zap.String("bucket_name", bucketName),
		zap.String("object_key", string(event.ObjectKey)),
		zap.String("event_name", eventName),
		zap.Int64("version", int64(event.Version)),
		zap.Int64("total_plain_size", event.TotalPlainSize),
		zap.Stringer("stream_id", event.StreamID),
		zap.Time("commit_timestamp", commitTimestamp))

	// The object key is URL-encoded (per S3 spec), and filter prefix/suffix are
	// stored URL-encoded as configured by the user, so we compare them as-is.
	objectKey := EncodeForS3Event([]byte(event.ObjectKey))

	config, err := s.buckets.GetBucketNotificationConfig(ctx, []byte(event.BucketName), event.ProjectID)
	if err != nil {
		s.log.Error("Failed to get bucket notification config",
			zap.Stringer("project_public_id", projectPublicID),
			zap.String("bucket_name", bucketName),
			zap.Error(err))
		return nil, err
	}

	if config == nil {
		s.log.Warn("No notification configuration exists for bucket, skipping",
			zap.Stringer("project_public_id", projectPublicID),
			zap.String("bucket_name", bucketName))
		return ImmediateResult(commitTimestamp), nil
	}

	if !MatchEventType(eventName, config.Events) {
		s.log.Warn("Event type does not match configuration, skipping",
			zap.Stringer("project_public_id", projectPublicID),
			zap.String("bucket_name", bucketName),
			zap.String("event_name", eventName),
			zap.Strings("configured_events", config.Events))
		return ImmediateResult(commitTimestamp), nil
	}

	if !MatchFilters(objectKey, config.FilterPrefix, config.FilterSuffix) {
		s.log.Warn("Object key does not match filter rules, skipping",
			zap.Stringer("project_public_id", projectPublicID),
			zap.String("bucket_name", bucketName),
			zap.String("object_key", string(objectKey)),
			zap.String("configured_prefix", string(config.FilterPrefix)),
			zap.String("configured_suffix", string(config.FilterSuffix)))
		return ImmediateResult(commitTimestamp), nil
	}

	s3event := buildS3Event(event, projectPublicID, config.ConfigID)

	publisher, err := s.GetPublisher(ctx, event.ProjectID, projectPublicID, bucketName, config.TopicName)
	if err != nil {
		// Error already logged in GetPublisher.
		if userConfigError(err) {
			// User misconfiguration (deleted project, missing permissions, etc.) - drop silently.
			ek.Event("publish_failed",
				eventkit.String("project_public_id", projectPublicID.String()),
				eventkit.String("bucket", bucketName),
				eventkit.String("event_name", eventName),
				eventkit.Bool("user_config_error", true),
				eventkit.String("error", err.Error()))

			return ImmediateResult(commitTimestamp), nil
		}
		return nil, err
	}

	data, err := s3event.Bytes()
	if err != nil {
		return nil, errs.New("failed to marshal event: %w", err)
	}

	s.log.Debug("Submitting event for publishing", zap.Any("event", s3event))

	return publisher.Publish(ctx, data, PublishMetadata{
		Log:             s.log,
		Timestamp:       commitTimestamp,
		ProjectPublicID: projectPublicID.String(),
		BucketName:      bucketName,
		EventName:       eventName,
		TopicName:       publisher.TopicName(),
		MessageSize:     int64(len(data)),
	}), nil
}

// GetPublisher returns a Publisher for the given bucket location, initializing it if necessary.
// The topicName parameter specifies the Pub/Sub topic for the publisher.
// If a cached publisher exists but has a different topic name, the old publisher is closed
// and a new one is created with the updated topic.
func (s *Service) GetPublisher(ctx context.Context, projectID, projectPublicID uuid.UUID, bucketName, topicName string) (_ Publisher, err error) {
	defer mon.Task()(&ctx)(&err)

	bucket := metabase.BucketLocation{
		ProjectID:  projectID,
		BucketName: metabase.BucketName(bucketName),
	}

	s.mu.RLock()
	if publisher, ok := s.publishers[bucket]; ok {
		// Check if the cached publisher's topic matches the requested topic
		if publisher.TopicName() == topicName {
			s.mu.RUnlock()
			return publisher, nil
		}
		// Topic changed - need to invalidate cache
	}
	s.mu.RUnlock()

	// Upgrade to write lock to create new publisher
	s.mu.Lock()
	defer s.mu.Unlock()

	// Double-check in case another goroutine created/updated it while we were waiting for the write lock
	if publisher, ok := s.publishers[bucket]; ok {
		if publisher.TopicName() == topicName {
			return publisher, nil
		}

		// Topic changed - close old publisher and remove from cache
		s.log.Info("Topic name changed for bucket, closing old publisher",
			zap.Stringer("project_public_id", projectPublicID),
			zap.Stringer("bucket_name", bucket.BucketName),
			zap.String("old_topic", publisher.TopicName()),
			zap.String("new_topic", topicName))

		if err := publisher.Close(); err != nil {
			s.log.Warn("Failed to close old publisher",
				zap.Stringer("project_public_id", projectPublicID),
				zap.Stringer("bucket_name", bucket.BucketName),
				zap.String("old_topic", publisher.TopicName()),
				zap.Error(err))
		}

		delete(s.publishers, bucket)
	}

	var publisher Publisher
	if s.cfg.TestNewPublisherFn != nil {
		publisher, err = s.cfg.TestNewPublisherFn()
	} else {
		publisher, err = NewPublisher(ctx, topicName)
	}
	if err != nil {
		s.log.Error("Failed to get publisher for bucket",
			zap.Stringer("project_public_id", projectPublicID),
			zap.Stringer("bucket_name", bucket.BucketName),
			zap.String("topic", topicName),
			zap.Error(err))
		return nil, err
	}

	s.publishers[bucket] = publisher

	return publisher, nil
}

// Close closes resources.
func (s *Service) Close() error {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var err error
	for _, publisher := range s.publishers {
		err = errs.Combine(err, publisher.Close())
	}
	return err
}
