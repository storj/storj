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
	"storj.io/storj/satellite/eventing/eventingconfig"
	"storj.io/storj/satellite/metabase"
	"storj.io/storj/satellite/metabase/changestream"
)

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
	return changestream.Processor(ctx, s.log, s.db, s.cfg.Feedname, start, func(record changestream.DataChangeRecord) error {
		// Ignore errors here, they are logged inside ProcessRecord
		_ = s.ProcessRecord(ctx, record)
		return nil
	})
}

// ProcessRecord processes a single change stream record.
func (s *Service) ProcessRecord(ctx context.Context, record changestream.DataChangeRecord) error {
	// Replace private project ID with public project ID in the record
	privateID, err := s.ReplaceProjectID(ctx, record)
	if err != nil {
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
		s.log.Error("Failed to convert mods to event", zap.Error(err))
		return err
	}

	if len(event.Records) == 0 {
		s.log.Debug("Nothing to publish")
		return nil
	}

	// Get the publisher for the bucket
	publisher, err := s.GetPublisher(ctx, metabase.BucketLocation{
		ProjectID:  privateID,
		BucketName: metabase.BucketName(event.Records[0].S3.Bucket.Name),
	})
	if err != nil {
		s.log.Error("Failed to get publisher for bucket",
			zap.String("Project Public ID", event.Records[0].S3.Bucket.OwnerIdentity.PrincipalId),
			zap.String("Bucket", event.Records[0].S3.Bucket.Name),
			zap.Error(err))
		return err
	}

	// Publish the event
	err = publisher.Publish(ctx, event)
	if err != nil {
		s.log.Error("Failed to publish event",
			zap.String("Project Public ID", event.Records[0].S3.Bucket.OwnerIdentity.PrincipalId),
			zap.String("Bucket", event.Records[0].S3.Bucket.Name),
			zap.Error(err))
		return err
	}
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
