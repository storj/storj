// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package reportedrollup

import (
	"context"
	"time"

	"github.com/spacemonkeygo/monkit/v3"
	"github.com/zeebo/errs"
	"go.uber.org/zap"

	"storj.io/common/pb"
	"storj.io/common/storj"
	"storj.io/common/sync2"
	"storj.io/common/uuid"
	"storj.io/storj/satellite/metainfo/metabase"
	"storj.io/storj/satellite/orders"
)

var (
	mon = monkit.Package()

	// Error is the error class for this package.
	Error = errs.Class("reportedrollup")
)

// Config is a configuration struct for the Chore.
type Config struct {
	Interval       time.Duration `help:"how often to flush the reported serial rollups to the database" default:"5m"`
	QueueBatchSize int           `help:"default queue batch size" default:"10000"`
}

// Chore for flushing reported serials to the database as rollups.
//
// architecture: Chore
type Chore struct {
	log    *zap.Logger
	db     orders.DB
	config Config

	Loop *sync2.Cycle
}

// NewChore creates new chore for flushing the reported serials to the database as rollups.
func NewChore(log *zap.Logger, db orders.DB, config Config) *Chore {
	if config.QueueBatchSize == 0 {
		config.QueueBatchSize = 10000
	}

	return &Chore{
		log:    log,
		db:     db,
		config: config,

		Loop: sync2.NewCycle(config.Interval),
	}
}

// Run starts the reported rollups chore.
func (chore *Chore) Run(ctx context.Context) (err error) {
	defer mon.Task()(&ctx)(&err)
	return chore.Loop.Run(ctx, func(ctx context.Context) error {
		err := chore.runOnceNow(ctx, time.Now)
		if err != nil {
			chore.log.Error("error flushing reported rollups", zap.Error(err))
		}
		return nil
	})
}

// Close stops the reported rollups chore.
func (chore *Chore) Close() error {
	chore.Loop.Close()
	return nil
}

// RunOnce finds expired bandwidth as of 'now' and inserts rollups into the appropriate tables.
func (chore *Chore) RunOnce(ctx context.Context, now time.Time) (err error) {
	defer mon.Task()(&ctx)(&err)

	return chore.runOnceNow(ctx, func() time.Time { return now })
}

// runOnceNow runs the helper repeatedly, calling the nowFn each time it runs it. It does that
// until the helper returns that it is done or an error occurs.
//
// This function exists because tests want to use RunOnce and have a single fixed time for
// reproducibility, but the chore loop wants to use whatever time.Now is every time the helper
// is run.
func (chore *Chore) runOnceNow(ctx context.Context, nowFn func() time.Time) (err error) {
	defer mon.Task()(&ctx)(&err)

	for {
		done, err := chore.runOnceHelper(ctx, nowFn())
		if err != nil {
			return errs.Wrap(err)
		}
		if done {
			return nil
		}
	}
}

func (chore *Chore) readWork(ctx context.Context, queue orders.Queue) (
	bucketRollups []orders.BucketBandwidthRollup,
	storagenodeRollups []orders.StoragenodeBandwidthRollup,
	consumedSerials []orders.ConsumedSerial,
	done bool, err error,
) {
	defer mon.Task()(&ctx)(&err)

	// Variables and types to keep track of bucket bandwidth rollups
	type bucketKey struct {
		projectID  uuid.UUID
		bucketName string
		action     pb.PieceAction
	}
	byBucket := make(map[bucketKey]uint64)

	// Variables and types to keep track of storagenode bandwidth rollups
	type storagenodeKey struct {
		nodeID storj.NodeID
		action pb.PieceAction
	}
	byStoragenode := make(map[storagenodeKey]uint64)

	// Variables to keep track of which serial numbers were consumed
	type consumedSerialKey struct {
		nodeID       storj.NodeID
		serialNumber storj.SerialNumber
	}
	seenConsumedSerials := make(map[consumedSerialKey]struct{})

	// Get a batch of pending serials from the queue.
	pendingSerials, queueDone, err := queue.GetPendingSerialsBatch(ctx, chore.config.QueueBatchSize)
	if err != nil {
		return nil, nil, nil, false, errs.Wrap(err)
	}

	for _, row := range pendingSerials {
		row := row

		// If we have seen this serial inside of this function already, don't
		// count it again and record it now.
		key := consumedSerialKey{
			nodeID:       row.NodeID,
			serialNumber: row.SerialNumber,
		}
		if _, exists := seenConsumedSerials[key]; exists {
			continue
		}
		seenConsumedSerials[key] = struct{}{}

		// Parse the node id, project id, and bucket name from the reported serial.
		bucket, err := metabase.ParseBucketPrefix(metabase.BucketPrefix(row.BucketID)) // TODO: rename row.BucketID -> row.BucketPrefix
		if err != nil {
			chore.log.Error("bad row inserted into reported serials",
				zap.Binary("bucket_id", row.BucketID),
				zap.String("node_id", row.NodeID.String()),
				zap.String("serial_number", row.SerialNumber.String()))
			continue
		}
		action := pb.PieceAction(row.Action)
		settled := row.Settled

		// Update our batch state to include it.
		byBucket[bucketKey{
			projectID:  bucket.ProjectID,
			bucketName: bucket.BucketName,
			action:     action,
		}] += settled

		byStoragenode[storagenodeKey{
			nodeID: row.NodeID,
			action: action,
		}] += settled

		consumedSerials = append(consumedSerials, orders.ConsumedSerial{
			NodeID:       row.NodeID,
			SerialNumber: row.SerialNumber,
			ExpiresAt:    row.ExpiresAt,
		})
	}

	// If we didn't get a full batch, the queue must have run out. We should signal
	// this fact to our caller so that they can stop looping.
	if queueDone {
		done = true
	}

	// Convert bucket rollups into a slice.
	for key, settled := range byBucket {
		bucketRollups = append(bucketRollups, orders.BucketBandwidthRollup{
			ProjectID:  key.projectID,
			BucketName: key.bucketName,
			Action:     key.action,
			Settled:    int64(settled),
		})
	}

	// Convert storagenode rollups into a slice.
	for key, settled := range byStoragenode {
		storagenodeRollups = append(storagenodeRollups, orders.StoragenodeBandwidthRollup{
			NodeID:  key.nodeID,
			Action:  key.action,
			Settled: int64(settled),
		})
	}

	chore.log.Debug("Read work",
		zap.Int("bucket_rollups", len(bucketRollups)),
		zap.Int("storagenode_rollups", len(storagenodeRollups)),
		zap.Int("consumed_serials", len(consumedSerials)),
		zap.Bool("done", done),
	)

	return bucketRollups, storagenodeRollups, consumedSerials, done, nil
}

func (chore *Chore) runOnceHelper(ctx context.Context, now time.Time) (done bool, err error) {
	defer mon.Task()(&ctx)(&err)

	err = chore.db.WithQueue(ctx, func(ctx context.Context, queue orders.Queue) error {
		var (
			bucketRollups      []orders.BucketBandwidthRollup
			storagenodeRollups []orders.StoragenodeBandwidthRollup
			consumedSerials    []orders.ConsumedSerial
		)

		// Read the work we should insert.
		bucketRollups, storagenodeRollups, consumedSerials, done, err = chore.readWork(ctx, queue)
		if err != nil {
			return errs.Wrap(err)
		}

		// Now that we have work, write it all in its own transaction.
		return errs.Wrap(chore.db.WithTransaction(ctx, func(ctx context.Context, tx orders.Transaction) error {
			if err := tx.UpdateBucketBandwidthBatch(ctx, now, bucketRollups); err != nil {
				return errs.Wrap(err)
			}
			if err := tx.UpdateStoragenodeBandwidthBatch(ctx, now, storagenodeRollups); err != nil {
				return errs.Wrap(err)
			}
			if err := tx.CreateConsumedSerialsBatch(ctx, consumedSerials); err != nil {
				return errs.Wrap(err)
			}
			return nil
		}))
	})
	return done, errs.Wrap(err)
}
