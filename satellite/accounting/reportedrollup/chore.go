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
	"storj.io/storj/satellite/orders"
)

var (
	mon = monkit.Package()

	// Error is the error class for this package
	Error = errs.Class("reportedrollup")
)

// Config is a configuration struct for the Chore.
type Config struct {
	Interval time.Duration `help:"how often to flush the reported serial rollups to the database" default:"5m"`
}

// Chore for flushing reported serials to the database as rollups.
//
// architecture: Chore
type Chore struct {
	log  *zap.Logger
	db   orders.DB
	Loop *sync2.Cycle
}

// NewChore creates new chore for flushing the reported serials to the database as rollups.
func NewChore(log *zap.Logger, db orders.DB, config Config) *Chore {
	return &Chore{
		log:  log,
		db:   db,
		Loop: sync2.NewCycle(config.Interval),
	}
}

// Run starts the reported rollups chore.
func (chore *Chore) Run(ctx context.Context) (err error) {
	defer mon.Task()(&ctx)(&err)
	return chore.Loop.Run(ctx, func(ctx context.Context) error {
		err := chore.RunOnce(ctx, time.Now())
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

	for {
		done, err := chore.runOnceHelper(ctx, now)
		if err != nil {
			return errs.Wrap(err)
		}
		if done {
			return nil
		}
	}
}

// TODO: jeeze make configurable
const (
	defaultQueueBatchSize           = 10000
	defaultRollupBatchSize          = 1000
	defaultConsumedSerialsBatchSize = 10000
)

func (chore *Chore) readWork(ctx context.Context, now time.Time, queue orders.Queue) (
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

	// Loop until our batch is big enough, but not too big in any dimension.
	for len(byBucket) < defaultRollupBatchSize &&
		len(byStoragenode) < defaultRollupBatchSize &&
		len(seenConsumedSerials) < defaultConsumedSerialsBatchSize {

		// Get a batch of pending serials from the queue.
		pendingSerials, queueDone, err := queue.GetPendingSerialsBatch(ctx, defaultQueueBatchSize)
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
			projectID, bucketName, err := orders.SplitBucketID(row.BucketID)
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
				projectID:  projectID,
				bucketName: string(bucketName),
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
			break
		}
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
		bucketRollups, storagenodeRollups, consumedSerials, done, err = chore.readWork(ctx, now, queue)
		if err != nil {
			return errs.Wrap(err)
		}

		// Now that we have work, write it all in its own transaction.
		return errs.Wrap(chore.db.WithTransaction(ctx, func(ctx context.Context, tx orders.Transaction) error {
			now := time.Now()

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
