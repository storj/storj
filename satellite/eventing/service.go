// Copyright (C) 2025 Storj Labs, Inc.
// See LICENSE for copying information.

package eventing

import (
	"context"
	"encoding/base64"
	"sync"
	"time"

	"github.com/zeebo/errs"
	"go.uber.org/zap"

	"storj.io/common/uuid"
	"storj.io/eventkit"
	"storj.io/storj/satellite/buckets"
	"storj.io/storj/satellite/eventing/eventingconfig"
	"storj.io/storj/satellite/metabase"
	"storj.io/storj/satellite/metabase/changestream"
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
	db         changestream.Adapter
	buckets    BucketNotificationConfigGetter
	projects   PublicProjectIDGetter
	enabled    eventingconfig.Config
	cfg        Config
	publishers map[metabase.BucketLocation]Publisher
	mu         sync.RWMutex
}

// NewService creates a new changestream service.
func NewService(log *zap.Logger, sdb changestream.Adapter, buckets BucketNotificationConfigGetter, projects PublicProjectIDGetter, enabled eventingconfig.Config, cfg Config) (*Service, error) {
	service := &Service{
		log:        log,
		db:         sdb,
		buckets:    buckets,
		projects:   projects,
		enabled:    enabled,
		cfg:        cfg,
		publishers: make(map[metabase.BucketLocation]Publisher),
	}

	// Validate configured topic names
	for _, topicName := range enabled.Buckets {
		_, _, err := ParseTopicName(topicName)
		if err != nil {
			return nil, err
		}
	}

	return service, nil
}

// Run starts the changestream processing loop.
func (s *Service) Run(ctx context.Context) (err error) {
	defer mon.Task()(&ctx)(&err)

	return changestream.Processor(ctx, s.log, s.db, s.cfg.Feedname, time.Now(), func(record changestream.DataChangeRecord) error {
		// Ignore errors here, they are logged inside ProcessRecord
		_ = s.ProcessRecord(ctx, record)
		return nil
	})
}

// ProcessRecord processes a single change stream record.
func (s *Service) ProcessRecord(ctx context.Context, record changestream.DataChangeRecord) (err error) {
	// Replace private project ID with public project ID in the record
	privateID, err := s.ReplaceProjectID(ctx, record)
	if err != nil {
		s.log.Error("Failed to replace project ID in record", zap.Error(err))
		return err
	}

	// Log the record for debugging purposes
	s.log.Debug("Received change record", zap.Any("Record", record))

	// Check if project is enabled for bucket eventing
	if !s.enabled.Projects.Enabled(privateID) {
		s.log.Debug("Project not enabled for bucket eventing, skipping",
			zap.Stringer("project_id", privateID)) // TODO: switch to public ID once we have it here
		return nil
	}

	// Convert the change stream record to an S3 event
	event, err := ConvertModsToEvent(record)
	if err != nil {
		s.log.Error("Failed to convert mods to event", zap.Error(err))
		return err
	}

	if len(event.Records) == 0 {
		// The commit-object transaction generates two change records: deleting
		// the pending object and inserting the committed object. Both are
		// included in the change stream because they are part of the same
		// transaction. Deleting the pending object does not generate any S3
		// event it will come here.
		ek.Event("change_record_no_events",
			eventkit.String("table", record.TableName),
			eventkit.String("mod_type", record.ModType),
			eventkit.String("transaction_tag", record.TransactionTag),
			eventkit.Int64("mods_count", int64(len(record.Mods))))

		s.log.Debug("Nothing to publish")
		return nil
	}

	projectPublicID := event.Records[0].S3.Bucket.OwnerIdentity.PrincipalId
	bucketName := event.Records[0].S3.Bucket.Name
	eventName := event.Records[0].EventName
	objectKey := []byte(event.Records[0].S3.Object.Key)

	// Get bucket notification configuration
	config := s.getConfigWithRetry(ctx, privateID, projectPublicID, bucketName)
	if config == nil {
		s.log.Warn("No notification configuration exists for bucket",
			zap.String("project_public_id", projectPublicID),
			zap.String("bucket_name", bucketName))
		return nil
	}

	// Evaluate event type matching (secondary filter)
	if !MatchEventType(eventName, config.Events) {
		s.log.Warn("Event type does not match configuration, skipping (cache inconsistency)",
			zap.String("event_name", eventName),
			zap.Strings("configured_events", config.Events))
		return nil
	}

	// Evaluate filter rules (secondary filter)
	if !MatchFilters(objectKey, config.FilterPrefix, config.FilterSuffix) {
		s.log.Warn("Object key does not match filter rules, skipping (cache inconsistency)",
			zap.String("object_key", string(objectKey)),
			zap.String("prefix", string(config.FilterPrefix)),
			zap.String("suffix", string(config.FilterSuffix)))
		return nil
	}

	// Get the publisher for the bucket
	publisher, err := s.GetPublisher(ctx, metabase.BucketLocation{
		ProjectID:  privateID,
		BucketName: metabase.BucketName(bucketName),
	}, config.TopicName)
	if err != nil {
		ek.Event("failed_to_get_publisher",
			eventkit.String("table", record.TableName),
			eventkit.String("project_public_id", projectPublicID),
			eventkit.String("bucket", bucketName),
			eventkit.String("mod_type", record.ModType),
			eventkit.String("error", err.Error()))

		s.log.Error("Failed to get publisher for bucket",
			zap.String("project_public_id", projectPublicID),
			zap.String("bucket_name", bucketName),
			zap.Error(err))
		return err
	}

	// Calculate and track processing latency
	latency := time.Since(record.CommitTimestamp)
	// Get message size
	messageSize, err := event.JSONSize()
	if err != nil {
		s.log.Error("Failed to marshal event for size calculation", zap.Error(err))
		// Continue with publish even if size calculation fails
	}

	// Publish the event
	err = publisher.Publish(ctx, event)
	if err != nil {
		ek.Event("publish_failed",
			eventkit.String("transaction_tag", record.TransactionTag),
			eventkit.String("project_public_id", projectPublicID),
			eventkit.String("bucket", bucketName),
			eventkit.String("event_name", eventName),
			eventkit.Int64("message_size_bytes", messageSize),
			eventkit.Duration("latency", latency),
			eventkit.String("error", err.Error()))

		s.log.Error("Failed to publish event",
			zap.String("project_public_id", projectPublicID),
			zap.String("bucket_name", bucketName),
			zap.Error(err))
		return err
	}

	// Track detailed success with eventkit
	ek.Event("publish_success",
		eventkit.String("transaction_tag", record.TransactionTag),
		eventkit.String("project_public_id", projectPublicID),
		eventkit.String("bucket", bucketName),
		eventkit.String("event_name", eventName),
		eventkit.Int64("message_size_bytes", messageSize),
		eventkit.Duration("latency", latency))

	s.log.Debug("Published event", zap.Any("event", event))

	return nil
}

// ReplaceProjectID replaces the private project ID in the record with the corresponding public project ID.
func (s *Service) ReplaceProjectID(ctx context.Context, record changestream.DataChangeRecord) (replaced uuid.UUID, err error) {
	for _, mod := range record.Mods {
		keys, err := parseNullJSONMap(mod.Keys, "keys")
		if err != nil {
			return uuid.UUID{}, errs.New("failed to parse keys: %w", err)
		}

		if projectID, ok := keys["project_id"]; ok {
			projectIDString, ok := projectID.(string)
			if !ok {
				return uuid.UUID{}, errs.New("project_id is not a string")
			}

			projectIDBytes, err := base64.StdEncoding.DecodeString(projectIDString)
			if err != nil {
				return uuid.UUID{}, errs.New("invalid base64 project_id: %w", err)
			}

			// TODO: what if mods span multiple projects?
			replaced, err = uuid.FromBytes(projectIDBytes)
			if err != nil {
				return uuid.UUID{}, errs.New("invalid project_id uuid: %w", err)
			}

			publicID, err := s.projects.GetPublicID(ctx, replaced)
			if err != nil {
				return uuid.UUID{}, errs.New("failed to get public project ID: %w", err)
			}

			keys["project_id"] = base64.StdEncoding.EncodeToString(publicID.Bytes())
		}
	}
	return replaced, nil
}

// getConfigWithRetry retrieves bucket notification configuration with
// exponential backoff retry. Retries indefinitely with exponential backoff
// (1s, 2s, 4s, 8s), then continues at 8s intervals. This blocks until
// configuration is successfully retrieved (may be nil, which is valid).
// Emits bucket_eventing_config_lookup_critical event when reaching max delay
// to trigger PagerDuty alerting for prolonged infrastructure failures.
func (s *Service) getConfigWithRetry(ctx context.Context, privateID uuid.UUID, projectPublicID, bucketName string) *buckets.NotificationConfig {
	retryDelays := []time.Duration{1 * time.Second, 2 * time.Second, 4 * time.Second, 8 * time.Second}
	maxDelay := 8 * time.Second
	attempt := 0

	for {
		config, err := s.buckets.GetBucketNotificationConfig(ctx, []byte(bucketName), privateID)
		if err == nil {
			// Success - config retrieved (may be nil, which is valid)
			return config
		}

		// Failed to get config - this is an infrastructure error
		// Determine delay: exponential backoff up to maxDelay, then constant maxDelay
		var delay time.Duration
		if attempt < len(retryDelays) {
			delay = retryDelays[attempt]
		} else {
			delay = maxDelay
			// TODO: Improve handling of failing Pub/Sub topics. Currently, when publishing fails,
			// we block processing of the entire change stream partition. The critical event
			// emitted below pages the on-call person to investigate.
			//
			// Emit critical event when reaching max delay to trigger alerting
			if attempt == len(retryDelays) {
				ek.Event("bucket_eventing_config_lookup_critical",
					eventkit.String("project_public_id", projectPublicID),
					eventkit.String("bucket", bucketName),
					eventkit.Int64("attempt", int64(attempt+1)),
					eventkit.String("error", err.Error()))
			}
		}

		s.log.Warn("Failed to get bucket notification config, retrying",
			zap.String("project_public_id", projectPublicID),
			zap.String("bucket_name", bucketName),
			zap.Int("attempt", attempt+1),
			zap.Duration("retry_after", delay),
			zap.Error(err))

		time.Sleep(delay)
		attempt++
	}
}

// GetPublisher returns a Publisher for the given bucket location, initializing it if necessary.
// The topicName parameter specifies the Pub/Sub topic for the publisher.
// If a cached publisher exists but has a different topic name, the old publisher is closed
// and a new one is created with the updated topic.
func (s *Service) GetPublisher(ctx context.Context, bucket metabase.BucketLocation, topicName string) (Publisher, error) {
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
			zap.Stringer("project_id", bucket.ProjectID),
			zap.Stringer("bucket_name", bucket.BucketName),
			zap.String("old_topic", publisher.TopicName()),
			zap.String("new_topic", topicName))

		if err := publisher.Close(); err != nil {
			s.log.Warn("Failed to close old publisher",
				zap.Stringer("project_id", bucket.ProjectID),
				zap.Stringer("bucket_name", bucket.BucketName),
				zap.String("old_topic", publisher.TopicName()),
				zap.Error(err))
		}

		delete(s.publishers, bucket)
	}

	var publisher Publisher
	var err error
	if s.cfg.TestNewPublisherFn != nil {
		publisher, err = s.cfg.TestNewPublisherFn()
		if err != nil {
			return nil, err
		}
	} else {
		publisher, err = NewPublisher(ctx, topicName)
		if err != nil {
			return nil, err
		}
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
