// Copyright (C) 2025 Storj Labs, Inc.
// See LICENSE for copying information.

package eventing

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"sync"
	"time"

	"github.com/spacemonkeygo/monkit/v3"
	"github.com/zeebo/errs"
	"go.uber.org/zap"

	"storj.io/common/uuid"
	"storj.io/eventkit"
	"storj.io/storj/satellite/eventing/eventingconfig"
	"storj.io/storj/satellite/metabase"
	"storj.io/storj/satellite/metabase/changestream"
)

var ek = eventkit.Package()

// Config holds configuration for the changestream service.
type Config struct {
	Feedname string `help:"the (spanner) name of the changestream to listen on" default:"bucket_eventing"`

	TestNewPublisherFn func() (EventPublisher, error) `noflag:"true"`
}

// PublicProjectIDer is an interface for looking up public project IDs.
type PublicProjectIDer interface {
	// GetPublicID returns the public project ID for a given project ID.
	GetPublicID(ctx context.Context, id uuid.UUID) (uuid.UUID, error)
}

// Service implements a changestream processing service.
type Service struct {
	log        *zap.Logger
	db         changestream.Adapter
	projects   PublicProjectIDer
	enabled    eventingconfig.Config
	cfg        Config
	publishers map[metabase.BucketLocation]EventPublisher
	mu         sync.RWMutex
}

// NewService creates a new changestream service.
func NewService(log *zap.Logger, sdb changestream.Adapter, projects PublicProjectIDer, enabled eventingconfig.Config, cfg Config) (*Service, error) {
	service := &Service{
		log:        log,
		db:         sdb,
		projects:   projects,
		enabled:    enabled,
		cfg:        cfg,
		publishers: make(map[metabase.BucketLocation]EventPublisher, len(enabled.Buckets)),
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
func (s *Service) Run(ctx context.Context) error {
	// TODO: we need to persist the last processed timestamp, time to time
	start := time.Now()
	return changestream.Processor(ctx, s.db, s.cfg.Feedname, start, func(record changestream.DataChangeRecord) error {
		// Ignore errors here, they are logged inside ProcessRecord
		_ = s.ProcessRecord(ctx, record)
		return nil
	})
}

// ProcessRecord processes a single change stream record.
func (s *Service) ProcessRecord(ctx context.Context, record changestream.DataChangeRecord) (err error) {
	defer mon.Task()(&ctx)(&err)

	// Track all incoming change records
	mon.Counter("change_records_received_total").Inc(1)
	ek.Event("change_record_received",
		eventkit.String("table", record.TableName),
		eventkit.String("mod_type", record.ModType),
		eventkit.Bool("is_system_transaction", record.IsSystemTransaction))

	// Track system transactions (these might be unexpected if we're filtering properly)
	if record.IsSystemTransaction {
		mon.Counter("change_records_system_transaction_total").Inc(1)
		ek.Event("change_record_system_transaction",
			eventkit.String("table", record.TableName),
			eventkit.String("mod_type", record.ModType))
	}

	// Replace private project ID with public project ID in the record
	privateID, err := s.ReplaceProjectID(ctx, record)
	if err != nil {
		mon.Counter("record_processing_failed_total",
			monkit.NewSeriesTag("reason", "project_id_lookup")).Inc(1)
		s.log.Error("Failed to replace project ID in record", zap.Error(err))
		return err
	}

	// Log the record for debugging purposes
	for _, mod := range record.Mods {
		s.log.Debug("Received change record",
			zap.String("table", record.TableName),
			zap.Time("commit_timestamp", record.CommitTimestamp),
			zap.String("mod_type", record.ModType),
			zap.String("record_sequence", record.RecordSequence),
			zap.String("transaction_tag", record.TransactionTag),
			zap.Stringer("keys", mod.Keys),
			zap.Stringer("old_values", mod.OldValues),
			zap.Stringer("new_values", mod.NewValues))
	}

	// Convert the change stream record to an S3 event
	event, err := ConvertModsToEvent(record)
	if err != nil {
		// Track conversion errors (unexpected - indicates malformed data)
		mon.Counter("change_records_conversion_error_total").Inc(1)
		mon.Counter("record_processing_failed_total",
			monkit.NewSeriesTag("reason", "conversion_error")).Inc(1)

		ek.Event("change_record_conversion_error",
			eventkit.String("table", record.TableName),
			eventkit.String("mod_type", record.ModType),
			eventkit.String("error", err.Error()))

		s.log.Error("Failed to convert mods to event", zap.Error(err))
		return err
	}

	if len(event.Records) == 0 {
		// Track unexpected empty records (no publishable events generated)
		// This indicates transactions that shouldn't be in the change stream
		mon.Counter("change_records_no_events_total").Inc(1)
		mon.Counter("change_records_no_events_by_type_total",
			monkit.NewSeriesTag("mod_type", record.ModType)).Inc(1)

		ek.Event("change_record_no_events",
			eventkit.String("table", record.TableName),
			eventkit.String("mod_type", record.ModType),
			eventkit.Int64("mods_count", int64(len(record.Mods))))

		s.log.Debug("Nothing to publish")
		return nil
	}

	// Extract common fields for metrics
	projectPublicID := event.Records[0].S3.Bucket.OwnerIdentity.PrincipalId
	bucketName := event.Records[0].S3.Bucket.Name

	// Track event processing with monkit counters (for Prometheus scraping)
	mon.Counter("event_processed_total").Inc(1)
	mon.Counter("event_by_type_total", monkit.NewSeriesTag("mod_type", record.ModType)).Inc(1)
	mon.Counter("event_by_project_total",
		monkit.NewSeriesTag("project_id", projectPublicID),
		monkit.NewSeriesTag("mod_type", record.ModType)).Inc(1)
	mon.Counter("event_by_bucket_total",
		monkit.NewSeriesTag("project_id", projectPublicID),
		monkit.NewSeriesTag("bucket", bucketName),
		monkit.NewSeriesTag("mod_type", record.ModType)).Inc(1)

	// Track detailed events with eventkit (for backend aggregation)
	ek.Event("event_processed")
	ek.Event("event_by_type",
		eventkit.String("mod_type", record.ModType))
	ek.Event("event_by_project",
		eventkit.String("project_id", projectPublicID),
		eventkit.String("mod_type", record.ModType))
	ek.Event("event_by_bucket",
		eventkit.String("project_id", projectPublicID),
		eventkit.String("bucket", bucketName),
		eventkit.String("mod_type", record.ModType))

	// Get the publisher for the bucket
	publisher, err := s.GetPublisher(ctx, metabase.BucketLocation{
		ProjectID:  privateID,
		BucketName: metabase.BucketName(event.Records[0].S3.Bucket.Name),
	})
	if err != nil {
		// Track buckets without configured topics (unexpected records for unconfigured buckets)
		if errs.Is(err, errs.New("no topic configured for bucket")) {
			mon.Counter("change_records_no_topic_total").Inc(1)
			mon.Counter("record_processing_failed_total",
				monkit.NewSeriesTag("reason", "no_topic")).Inc(1)

			ek.Event("change_record_no_topic",
				eventkit.String("project_public_id", projectPublicID),
				eventkit.String("bucket", bucketName),
				eventkit.String("mod_type", record.ModType))
		} else {
			mon.Counter("record_processing_failed_total",
				monkit.NewSeriesTag("reason", "publisher_error")).Inc(1)
		}

		s.log.Error("Failed to get publisher for bucket",
			zap.String("Project Public ID", event.Records[0].S3.Bucket.OwnerIdentity.PrincipalId),
			zap.String("Bucket", event.Records[0].S3.Bucket.Name),
			zap.Error(err))
		return err
	}

	// Calculate and track processing latency
	duration := time.Since(record.CommitTimestamp)

	// Track latency with monkit histogram (for Prometheus percentiles)
	mon.DurationVal("processing_latency").Observe(duration)
	mon.DurationVal("processing_latency_by_bucket",
		monkit.NewSeriesTag("project_id", projectPublicID),
		monkit.NewSeriesTag("bucket", bucketName)).Observe(duration)

	// Calculate event message size (serialize to JSON to get actual size)
	eventJSON, err := json.Marshal(event)
	if err != nil {
		s.log.Error("Failed to marshal event for size calculation", zap.Error(err))
		// Continue with publish even if size calculation fails
	}
	messageSize := int64(len(eventJSON))

	// Track message size with monkit (for min/max/avg via Prometheus)
	mon.IntVal("message_size_bytes").Observe(messageSize)
	mon.IntVal("message_size_bytes_by_bucket",
		monkit.NewSeriesTag("project_id", projectPublicID),
		monkit.NewSeriesTag("bucket", bucketName)).Observe(messageSize)

	// Track detailed latency and size with eventkit (for backend analysis)
	ek.Event("processing_latency",
		eventkit.String("project_public_id", projectPublicID),
		eventkit.String("bucket", bucketName),
		eventkit.String("mod_type", record.ModType),
		eventkit.Duration("duration", duration),
		eventkit.Int64("duration_ms", duration.Milliseconds()),
		eventkit.Int64("message_size_bytes", messageSize),
	)

	// Publish the event
	err = publisher.Publish(ctx, event)
	if err != nil {
		// Track publish failures with monkit (these are retryable at partition level)
		mon.Counter("publish_failed_total",
			monkit.NewSeriesTag("project_id", projectPublicID),
			monkit.NewSeriesTag("bucket", bucketName)).Inc(1)
		mon.Counter("record_processing_failed_total",
			monkit.NewSeriesTag("reason", "publish_failed")).Inc(1)

		// Track detailed failure with eventkit
		ek.Event("publish_failed",
			eventkit.String("project_public_id", projectPublicID),
			eventkit.String("bucket", bucketName),
			eventkit.String("mod_type", record.ModType),
			eventkit.Int64("message_size_bytes", messageSize),
			eventkit.String("error", err.Error()),
			eventkit.Bool("retryable", true))

		s.log.Error("Failed to publish event",
			zap.String("Project Public ID", projectPublicID),
			zap.String("Bucket", bucketName),
			zap.Error(err))
		return err
	}

	// Track publish success with monkit
	mon.Counter("publish_success_total",
		monkit.NewSeriesTag("project_id", projectPublicID),
		monkit.NewSeriesTag("bucket", bucketName)).Inc(1)

	// Track detailed success with eventkit
	ek.Event("publish_success",
		eventkit.String("project_public_id", projectPublicID),
		eventkit.String("bucket", bucketName),
		eventkit.String("mod_type", record.ModType),
		eventkit.Int64("message_size_bytes", messageSize))

	s.log.Debug("Published event", zap.Any("Event", event))

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

// GetPublisher returns an EventPublisher for the given bucket location, initializing it if necessary.
func (s *Service) GetPublisher(ctx context.Context, bucket metabase.BucketLocation) (EventPublisher, error) {
	s.mu.RLock()
	if publisher, ok := s.publishers[bucket]; ok {
		s.mu.RUnlock()
		return publisher, nil
	}
	s.mu.RUnlock()

	// Upgrade to write lock to create new publisher
	s.mu.Lock()
	defer s.mu.Unlock()

	// Double-check in case another goroutine created it while we were waiting for the write lock
	if publisher, ok := s.publishers[bucket]; ok {
		return publisher, nil
	}

	topicName, ok := s.enabled.Buckets[bucket]
	if !ok {
		return nil, errs.New("no topic configured for bucket")
	}

	var publisher EventPublisher
	if s.cfg.TestNewPublisherFn != nil {
		var err error
		publisher, err = s.cfg.TestNewPublisherFn()
		if err != nil {
			return nil, err
		}
	} else if topicName == "@log" {
		publisher = NewLogPublisher(s.log)
	} else {
		projectID, topicID, err := ParseTopicName(topicName)
		if err != nil {
			return nil, err
		}

		publisher, err = NewPubSubPublisher(ctx, PubSubConfig{
			ProjectID: projectID,
			TopicID:   topicID,
		})
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
