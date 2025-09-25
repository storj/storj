// Copyright (C) 2025 Storj Labs, Inc.
// See LICENSE for copying information.

package changestream

import (
	"context"
	"sync"
	"time"

	"github.com/zeebo/errs"
	"go.uber.org/zap"
	"golang.org/x/exp/slices"
	"golang.org/x/sync/errgroup"

	"storj.io/storj/satellite/eventing"
	"storj.io/storj/satellite/metabase"
)

// Config holds configuration for the changestream service.
type Config struct {
	Feedname           string                            `help:"the (spanner) name of the changestream to listen on" default:"bucket_eventing"`
	Buckets            eventing.BucketLocationTopicIDMap `help:"defines which buckets are monitored for events (comma separated list of \"project_id:bucket_name:topic_id\")" default:""`
	TestNewPublisherFn func() (EventPublisher, error)
}

// Service implements a changestream processing service.
type Service struct {
	db         metabase.ChangeStreamAdapter
	log        *zap.Logger
	cfg        Config
	publishers map[metabase.BucketLocation]EventPublisher
	mu         sync.RWMutex
}

// NewService creates a new changestream service.
func NewService(db metabase.Adapter, log *zap.Logger, cfg Config) (*Service, error) {
	sdb, ok := db.(metabase.ChangeStreamAdapter)

	if !ok {
		return nil, errs.New("changestream service requires spanner adapter")
	}

	service := &Service{
		log:        log,
		db:         sdb,
		cfg:        cfg,
		publishers: make(map[metabase.BucketLocation]EventPublisher, len(cfg.Buckets)),
	}

	// Validate configured topic names
	for _, topicName := range cfg.Buckets {
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

	topicName, ok := s.cfg.Buckets[bucket]
	if !ok {
		return nil, errs.New("no topic configured for bucket")
	}

	projectID, topicID, err := ParseTopicName(topicName)
	if err != nil {
		return nil, err
	}

	var publisher EventPublisher
	if s.cfg.TestNewPublisherFn != nil {
		publisher, err = s.cfg.TestNewPublisherFn()
	} else {
		publisher, err = NewPubSubPublisher(ctx, PubSubConfig{
			ProjectID: projectID,
			TopicID:   topicID,
		})
	}
	if err != nil {
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

// Processor processes change stream records in batches (parallel). This contains the logic to follow child partitions.
func Processor(ctx context.Context, adapter metabase.ChangeStreamAdapter, feedName string, startTime time.Time, fn func(record metabase.DataChangeRecord) error) error {
	var mu sync.Mutex
	// TODO: from the spanner documentation it's not clear the graph structure returned by changestream. Might be enough to use []ChildPartitionsRecord.
	//  or we can execute all potential ready to listen partitions parallel not just one level.
	var todo [][]metabase.ChildPartitionsRecord
	for {
		if ctx.Err() != nil {
			return ctx.Err()
		}
		if len(todo) == 0 {
			partitions, err := adapter.ChangeStream(ctx, feedName, "", startTime, func(record metabase.DataChangeRecord) error {
				return fn(record)
			})
			if err != nil {
				return errs.Wrap(err)
			}
			todo = append(todo, partitions)
			continue
		}

		// TODO: make sure parents are already processed
		slices.SortFunc(todo, func(a, b []metabase.ChildPartitionsRecord) int {
			return a[0].StartTimestamp.Compare(b[0].StartTimestamp)
		})

		eg, childCtx := errgroup.WithContext(ctx)
		nextItem := todo[0]
		for _, p := range nextItem {
			for _, child := range p.ChildPartitions {
				eg.Go(func() error {
					child := child
					partitions, err := adapter.ChangeStream(childCtx, feedName, child.Token, p.StartTimestamp, func(record metabase.DataChangeRecord) error {
						return fn(record)
					})
					if err == nil {
						mu.Lock()
						if len(partitions) > 0 {
							todo = append(todo, partitions)
						}
						mu.Unlock()
					}
					return err
				})
			}
		}
		err := eg.Wait()
		if err != nil {
			return errs.Wrap(err)
		}

		todo = todo[1:]
	}
}
