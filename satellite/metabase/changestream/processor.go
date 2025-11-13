// Copyright (C) 2025 Storj Labs, Inc.
// See LICENSE for copying information.

package changestream

import (
	"context"
	"time"

	"cloud.google.com/go/spanner"
	"github.com/spacemonkeygo/monkit/v3"
	"go.uber.org/zap"
	"golang.org/x/sync/errgroup"
	"google.golang.org/grpc/codes"

	"storj.io/common/errs2"
)

var mon = monkit.Package()

// notifyingAdapter wraps an Adapter to add notification capability for the processor.
type notifyingAdapter struct {
	Adapter
	notifyCh chan struct{}
}

// newNotifyingAdapter creates a new notifying adapter wrapper.
func newNotifyingAdapter(adapter Adapter) *notifyingAdapter {
	return &notifyingAdapter{
		Adapter:  adapter,
		notifyCh: make(chan struct{}, 1), // Buffered to avoid blocking
	}
}

// UpdateChangeStreamPartitionState wraps the underlying call and sends a notification when transitioning to Finished.
func (n *notifyingAdapter) UpdateChangeStreamPartitionState(ctx context.Context, feedName, partitionToken string, state PartitionState) error {
	err := n.Adapter.UpdateChangeStreamPartitionState(ctx, feedName, partitionToken, state)
	if err == nil && state == StateFinished {
		// Notify when a partition finishes so its children can be scheduled
		n.notify()
	}
	return err
}

// notify sends a non-blocking notification.
func (n *notifyingAdapter) notify() {
	select {
	case n.notifyCh <- struct{}{}:
		// Notification sent successfully
	default:
		// Channel already has a notification pending, no need to send another
	}
}

// Processor processes change stream records in batches (parallel). This contains the logic to follow child partitions.
func Processor(ctx context.Context, log *zap.Logger, adapter Adapter, feedName string, startTime time.Time, fn func(record DataChangeRecord) error) (err error) {
	defer mon.Task()(&ctx)(&err)

	// Wrap the adapter with notification capability
	notifier := newNotifyingAdapter(adapter)

	log.Info("Starting change stream processor", zap.String("Change Stream", feedName))

	err = processLoop(ctx, log, notifier, feedName, startTime, fn)
	if errs2.IgnoreCanceled(err) != nil && spanner.ErrCode(err) != codes.Canceled {
		log.Error("Change stream processor exited with error", zap.String("Change Stream", feedName), zap.Error(err))
	} else {
		log.Info("Change stream processor exited", zap.String("Change Stream", feedName))
	}

	return err
}

func processLoop(ctx context.Context, log *zap.Logger, adapter *notifyingAdapter, feedName string, startTime time.Time, fn func(record DataChangeRecord) error) (err error) {
	defer mon.Task()(&ctx)(&err)

	eg, childCtx := errgroup.WithContext(ctx)

	eg.Go(func() error {
		noMetadata, err := adapter.ChangeStreamNoPartitionMetadata(childCtx, feedName)
		if err != nil {
			return err
		}

		if noMetadata {
			log.Debug("No partition metadata found. Adding the initial partition",
				zap.String("Change Stream", feedName))

			err = adapter.AddChangeStreamPartition(childCtx, feedName, "", nil, startTime)
			if err != nil {
				return err
			}

			// Notify to process the newly created initial partition
			adapter.notify()
		} else {
			unfinished, err := adapter.GetChangeStreamPartitionsByState(childCtx, feedName, StateRunning)
			if err != nil {
				return err
			}

			log.Debug("Unfinished partitions found",
				zap.String("Change Stream", feedName),
				zap.Int("Count", len(unfinished)))

			for partitionToken, startTime := range unfinished {
				partitionToken, startTime := partitionToken, startTime
				eg.Go(func() error {
					return processPartition(childCtx, log, adapter, feedName, partitionToken, startTime, fn)
				})
			}

			// Send an initial notification to process any existing Created/Scheduled partitions
			// This prevents deadlock when restarting with only non-Running partitions
			adapter.notify()
		}

		for {
			select {
			case <-childCtx.Done():
				return childCtx.Err()
			case <-adapter.notifyCh:
				log.Debug("Received partition notification", zap.String("Change Stream", feedName))
			}

			count, err := adapter.ScheduleChangeStreamPartitions(childCtx, feedName)
			if err != nil {
				return err
			}

			log.Debug("New partitions scheduled",
				zap.String("Change Stream", feedName),
				zap.Int64("Count", count))

			scheduled, err := adapter.GetChangeStreamPartitionsByState(childCtx, feedName, StateScheduled)
			if err != nil {
				return err
			}

			log.Debug("Scheduled partitions found",
				zap.String("Change Stream", feedName),
				zap.Int("Count", len(scheduled)))

			if len(scheduled) == 0 {
				continue
			}

			for partitionToken, startTime := range scheduled {
				partitionToken, startTime := partitionToken, startTime

				log.Debug("Mark partition as running",
					zap.String("Change Stream", feedName),
					zap.String("Partition Token", partitionToken))

				err = adapter.UpdateChangeStreamPartitionState(childCtx, feedName, partitionToken, StateRunning)
				if err != nil {
					return err
				}

				eg.Go(func() error {
					return processPartition(childCtx, log, adapter, feedName, partitionToken, startTime, fn)
				})
			}
		}
	})

	return eg.Wait()
}

func processPartition(ctx context.Context, log *zap.Logger, adapter *notifyingAdapter, feedName, partitionToken string, startTime time.Time, fn func(record DataChangeRecord) error) (err error) {
	defer mon.Task()(&ctx)(&err)

	err = adapter.ReadChangeStreamPartition(ctx, feedName, partitionToken, startTime, func(record ChangeRecord) error {
		for _, dataChange := range record.DataChangeRecord {
			// We don't log the data change here as may contain sensitive information.
			// The callback function is expected to log it if needed after filtering sensitive data.
			err := fn(*dataChange)
			if err != nil {
				return err
			}

			log.Debug("Received data change. Updating partition watermark",
				zap.String("Change Stream", feedName),
				zap.String("Partition Token", partitionToken),
				zap.Time("Commit Timestamp", dataChange.CommitTimestamp))

			err = adapter.UpdateChangeStreamPartitionWatermark(ctx, feedName, partitionToken, dataChange.CommitTimestamp)
			if err != nil {
				return err
			}
		}
		for _, partition := range record.ChildPartitionsRecord {
			for _, child := range partition.ChildPartitions {
				log.Debug("Received child partition. Adding it to metabase",
					zap.String("Change Stream", feedName),
					zap.Time("Start Timestamp", partition.StartTimestamp),
					zap.String("Record Sequence", partition.RecordSequence),
					zap.String("Partition Token", partitionToken),
					zap.String("Child Token", child.Token),
					zap.Strings("Parent Tokens", child.ParentPartitionTokens))

				err := adapter.AddChangeStreamPartition(ctx, feedName, child.Token, child.ParentPartitionTokens, partition.StartTimestamp)
				if err != nil {
					return err
				}
			}
		}
		for _, hb := range record.HeartbeatRecord {
			log.Debug("Received heartbeat. Updating partition watermark",
				zap.String("Change Stream", feedName),
				zap.String("Partition Token", partitionToken),
				zap.Time("Timestamp", hb.Timestamp))

			err = adapter.UpdateChangeStreamPartitionWatermark(ctx, feedName, partitionToken, hb.Timestamp)
			if err != nil {
				return err
			}
		}
		return nil
	})
	if err != nil {
		return err
	}

	log.Debug("Mark partition as finished",
		zap.String("Change Stream", feedName),
		zap.String("Partition Token", partitionToken))

	err = adapter.UpdateChangeStreamPartitionState(ctx, feedName, partitionToken, StateFinished)
	if err != nil {
		return err
	}

	return nil
}
