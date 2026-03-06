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

// notifyingBatcher wraps a MetadataBatcher and sends a notification when a
// partition transitions to StateFinished, so the main loop can schedule children.
type notifyingBatcher struct {
	*MetadataBatcher
	notifyCh chan struct{}
}

func newNotifyingBatcher(batcher *MetadataBatcher) *notifyingBatcher {
	return &notifyingBatcher{
		MetadataBatcher: batcher,
		notifyCh:        make(chan struct{}, 1),
	}
}

// UpdatePartitionState buffers the state and notifies when transitioning to StateFinished.
func (n *notifyingBatcher) UpdatePartitionState(state PartitionState, partitionTokens ...string) {
	n.MetadataBatcher.UpdatePartitionState(state, partitionTokens...)
	if state == StateFinished {
		n.notify()
	}
}

// notify sends a non-blocking notification.
func (n *notifyingBatcher) notify() {
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

	log.Info("Starting change stream processor", zap.String("change_stream", feedName))

	err = processLoop(ctx, log, adapter, feedName, startTime, fn)
	if errs2.IgnoreCanceled(err) != nil && spanner.ErrCode(err) != codes.Canceled {
		log.Error("Change stream processor exited with error", zap.String("change_stream", feedName), zap.Error(err))
	} else {
		log.Info("Change stream processor exited", zap.String("change_stream", feedName))
	}

	return err
}

func processLoop(ctx context.Context, log *zap.Logger, adapter Adapter, feedName string, startTime time.Time, fn func(record DataChangeRecord) error) (err error) {
	defer mon.Task()(&ctx)(&err)

	batcher := newNotifyingBatcher(NewMetadataBatcher(log, adapter, feedName))

	eg, childCtx := errgroup.WithContext(ctx)

	eg.Go(func() error {
		noMetadata, err := adapter.ChangeStreamNoPartitionMetadata(childCtx, feedName)
		if err != nil {
			return err
		}

		if noMetadata {
			log.Debug("No partition metadata found. Adding the initial partition",
				zap.String("change_stream", feedName))

			batcher.AddChildPartition("", nil, startTime)
			if err := batcher.Flush(childCtx); err != nil {
				return err
			}

			// Notify to process the newly created initial partition
			batcher.notify()
		} else {
			unfinished, err := adapter.GetChangeStreamPartitionsByState(childCtx, feedName, StateRunning)
			if err != nil {
				return err
			}

			log.Debug("Unfinished partitions found",
				zap.String("change_stream", feedName),
				zap.Int("count", len(unfinished)))

			for partitionToken, startTime := range unfinished {
				partitionToken, startTime := partitionToken, startTime
				eg.Go(func() error {
					return processPartition(childCtx, log, adapter, batcher, feedName, partitionToken, startTime, fn)
				})
			}

			// Send an initial notification to process any existing Created/Scheduled partitions
			// This prevents deadlock when restarting with only non-Running partitions
			batcher.notify()
		}

		flushTicker := time.NewTicker(1 * time.Second)
		defer flushTicker.Stop()

		for {
			select {
			case <-childCtx.Done():
				log.Debug("Context cancelled, performing final flush", zap.String("change_stream", feedName))
				flushCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
				err := batcher.Flush(flushCtx)
				cancel()
				return err

			case <-flushTicker.C:
				log.Debug("Flush ticker triggered", zap.String("change_stream", feedName))
				if err := batcher.Flush(childCtx); err != nil {
					return err
				}

			case <-batcher.notifyCh:
				log.Debug("Received partition notification", zap.String("change_stream", feedName))

				// Flush first so StateFinished is persisted before SchedulePartitions reads it
				if err := batcher.Flush(childCtx); err != nil {
					return err
				}

				count, err := adapter.ScheduleChangeStreamPartitions(childCtx, feedName)
				if err != nil {
					return err
				}

				log.Debug("New partitions scheduled",
					zap.String("change_stream", feedName),
					zap.Int64("count", count))

				if count == 0 {
					continue
				}

				scheduled, err := adapter.GetChangeStreamPartitionsByState(childCtx, feedName, StateScheduled)
				if err != nil {
					return err
				}

				log.Debug("Scheduled partitions found",
					zap.String("change_stream", feedName),
					zap.Int("count", len(scheduled)))

				partitionTokens := make([]string, 0, len(scheduled))
				for partitionToken := range scheduled {
					log.Debug("Mark partition as running",
						zap.String("change_stream", feedName),
						zap.String("partition_token", partitionToken))
					partitionTokens = append(partitionTokens, partitionToken)
				}
				batcher.UpdatePartitionState(StateRunning, partitionTokens...)

				if err := batcher.Flush(childCtx); err != nil {
					return err
				}

				for partitionToken, startTime := range scheduled {
					partitionToken, startTime := partitionToken, startTime
					eg.Go(func() error {
						return processPartition(childCtx, log, adapter, batcher, feedName, partitionToken, startTime, fn)
					})
				}
			}
		}
	})

	return eg.Wait()
}

func processPartition(ctx context.Context, log *zap.Logger, adapter Adapter, batcher *notifyingBatcher, feedName, partitionToken string, startTime time.Time, fn func(record DataChangeRecord) error) (err error) {
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
				zap.String("change_stream", feedName),
				zap.String("partition_token", partitionToken),
				zap.Time("commit_timestamp", dataChange.CommitTimestamp))

			batcher.UpdatePartitionWatermark(partitionToken, dataChange.CommitTimestamp)
		}
		for _, partition := range record.ChildPartitionsRecord {
			for _, child := range partition.ChildPartitions {
				log.Debug("Received child partition. Adding it to metabase",
					zap.String("change_stream", feedName),
					zap.Time("start_timestamp", partition.StartTimestamp),
					zap.String("record_sequence", partition.RecordSequence),
					zap.String("partition_token", partitionToken),
					zap.String("child_token", child.Token),
					zap.Strings("parent_tokens", child.ParentPartitionTokens))

				batcher.AddChildPartition(child.Token, child.ParentPartitionTokens, partition.StartTimestamp)
			}
		}
		for _, hb := range record.HeartbeatRecord {
			log.Debug("Received heartbeat. Updating partition watermark",
				zap.String("change_stream", feedName),
				zap.String("partition_token", partitionToken),
				zap.Time("timestamp", hb.Timestamp))

			batcher.UpdatePartitionWatermark(partitionToken, hb.Timestamp)
		}
		return nil
	})
	if err != nil {
		return err
	}

	log.Debug("Mark partition as finished",
		zap.String("change_stream", feedName),
		zap.String("partition_token", partitionToken))

	// UpdatePartitionState also triggers a notification so the main loop
	// will flush and schedule children of this partition.
	batcher.UpdatePartitionState(StateFinished, partitionToken)

	return nil
}
