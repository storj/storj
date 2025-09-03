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

	"storj.io/storj/satellite/metabase"
)

// Config holds configuration for the changestream service.
type Config struct {
	Feedname string `help:"the (spanner) name of the changestream to listen on" default:"bucket_eventing"`
}

// Service implements a changestream processing service.
type Service struct {
	db        metabase.ChangeStreamAdapter
	log       *zap.Logger
	cfg       Config
	publisher EventPublisher
}

// NewService creates a new changestream service.
func NewService(db metabase.Adapter, log *zap.Logger, cfg Config, publisher EventPublisher) (*Service, error) {
	sdb, ok := db.(metabase.ChangeStreamAdapter)

	if !ok {
		return nil, errs.New("changestream service requires spanner adapter")
	}
	return &Service{
		log:       log,
		db:        sdb,
		cfg:       cfg,
		publisher: publisher,
	}, nil
}

// Run starts the changestream processing loop.
func (s *Service) Run(ctx context.Context) error {
	// TODO: we need to persist the last processed timestamp, time to time
	start := time.Now()
	return Processor(ctx, s.db, s.cfg.Feedname, start, func(record metabase.DataChangeRecord) error {
		for _, mod := range record.Mods {
			s.log.Debug("received change record",
				zap.String("table", record.TableName),
				zap.Time("commit_timestamp", record.CommitTimestamp),
				zap.String("mod_type", record.ModType),
				zap.String("record_sequence", record.RecordSequence),
				zap.String("transaction_tag", record.TransactionTag),
				zap.Stringer("keys", mod.Keys),
				zap.Stringer("old_values", mod.OldValues),
				zap.Stringer("new_values", mod.NewValues))
		}

		event, err := ConvertModsToEvent(record)
		if err != nil {
			s.log.Error("failed to convert mods to event", zap.Error(err))
			return nil
		}

		if len(event.Records) == 0 {
			// Nothing to publish
			return nil
		}

		err = s.publisher.Publish(ctx, event)
		if err != nil {
			s.log.Error("failed to publish event", zap.Error(err))
			return nil
		}
		return nil
	})
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
