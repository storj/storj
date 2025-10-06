// Copyright (C) 2025 Storj Labs, Inc.
// See LICENSE for copying information.

package changestream

import (
	"context"
	"sync"
	"time"

	"github.com/zeebo/errs"
	"go.uber.org/zap"

	"storj.io/storj/satellite/eventing"
	"storj.io/storj/satellite/metabase"
)

// Config holds configuration for the changestream service.
type Config struct {
	Feedname string `help:"the (spanner) name of the changestream to listen on" default:"bucket_eventing"`

	TestNewPublisherFn func() (EventPublisher, error) `noflag:"true"`
}

// Service implements a changestream processing service.
type Service struct {
	db          metabase.ChangeStreamAdapter
	log         *zap.Logger
	eventingCfg eventing.Config
	cfg         Config
	publishers  map[metabase.BucketLocation]EventPublisher
	mu          sync.RWMutex
}

// NewService creates a new changestream service.
func NewService(db metabase.Adapter, log *zap.Logger, eventingCfg eventing.Config, cfg Config) (*Service, error) {
	sdb, ok := db.(metabase.ChangeStreamAdapter)

	if !ok {
		return nil, errs.New("changestream service requires spanner adapter")
	}

	service := &Service{
		log:         log,
		db:          sdb,
		eventingCfg: eventingCfg,
		cfg:         cfg,
		publishers:  make(map[metabase.BucketLocation]EventPublisher, len(eventingCfg.Buckets)),
	}

	// Validate configured topic names
	for _, topicName := range eventingCfg.Buckets {
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
	return Processor(ctx, s.db, s.cfg.Feedname, start, func(record metabase.DataChangeRecord) error {
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

		// Ignore errors here, they are logged inside ProcessRecord
		_ = s.ProcessRecord(ctx, record)

		return nil
	})
}

// ProcessRecord processes a single change stream record.
func (s *Service) ProcessRecord(ctx context.Context, record metabase.DataChangeRecord) error {
	event, err := ConvertModsToEvent(record)
	if err != nil {
		s.log.Error("Failed to convert mods to event", zap.Error(err))
		return err
	}

	if len(event.Records) == 0 {
		s.log.Debug("Nothing to publish")
		return nil
	}

	publisher, err := s.GetPublisher(ctx, event.Bucket)
	if err != nil {
		s.log.Error("Failed to get publisher for bucket",
			zap.Stringer("Project ID", event.Bucket.ProjectID),
			zap.Stringer("Bucket", event.Bucket.BucketName))
		return err
	}

	err = publisher.Publish(ctx, event)
	if err != nil {
		s.log.Error("Failed to publish event",
			zap.Stringer("Project ID", event.Bucket.ProjectID),
			zap.Stringer("Bucket", event.Bucket.BucketName),
			zap.Error(err))
		return err
	}

	return nil
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

	topicName, ok := s.eventingCfg.Buckets[bucket]
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
