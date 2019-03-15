// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package tally

import (
	"context"
	"time"

	"github.com/gogo/protobuf/proto"
	"github.com/zeebo/errs"
	"go.uber.org/zap"

	"storj.io/storj/pkg/accounting"
	"storj.io/storj/pkg/bwagreement"
	"storj.io/storj/pkg/pb"
	"storj.io/storj/pkg/pointerdb"
	"storj.io/storj/pkg/storj"
	"storj.io/storj/storage"
)

// Config contains configurable values for tally
type Config struct {
	Interval time.Duration `help:"how frequently tally should run" default:"30s"`
}

// Tally is the service for accounting for data stored on each storage node
type Tally struct { // TODO: rename Tally to Service
	pointerdb       *pointerdb.Service
	overlay         pb.OverlayServer // TODO: this should be *overlay.Service
	limit           int
	logger          *zap.Logger
	ticker          *time.Ticker
	accountingDB    accounting.DB
	bwAgreementDB   bwagreement.DB
	bucketUsageDB   accounting.BucketUsage
	bucketBWUsageDB accounting.BucketBandwidthUsage
}

// New creates a new Tally
func New(logger *zap.Logger, acctDB accounting.DB, bwaDB bwagreement.DB, bucketDB accounting.BucketUsage, bucketBWDB accounting.BucketBandwidthUsage, pdb *pointerdb.Service, overlay pb.OverlayServer, limit int, interval time.Duration) *Tally {
	return &Tally{
		pointerdb:       pdb,
		overlay:         overlay,
		limit:           limit,
		logger:          logger,
		ticker:          time.NewTicker(interval),
		accountingDB:    acctDB,
		bwAgreementDB:   bwaDB,
		bucketUsageDB:   bucketDB,
		bucketBWUsageDB: bucketBWDB,
	}
}

// Run the Tally loop
func (t *Tally) Run(ctx context.Context) (err error) {
	t.logger.Info("Tally service starting up")
	defer mon.Task()(&ctx)(&err)
	for {
		if err = t.Tally(ctx); err != nil {
			t.logger.Error("Tally failed", zap.Error(err))
		}
		select {
		case <-t.ticker.C: // wait for the next interval to happen
		case <-ctx.Done(): // or the Tally is canceled via context
			return ctx.Err()
		}
	}
}

//Tally calculates data-at-rest and bandwidth usage once
func (t *Tally) Tally(ctx context.Context) error {

	//data at rest
	var errAtRest, errBWA error
	latestTally, nodeData, bucketsData, err := t.calculateAtRestData(ctx)
	if err != nil {
		errAtRest = errs.New("Query for data-at-rest failed : %v", err)
	} else if len(nodeData) > 0 {
		err = t.SaveAtRestRaw(ctx, latestTally, time.Now().UTC(), nodeData)
		if err != nil {
			errAtRest = errs.New("Saving data-at-rest failed : %v", err)
		}
	}

	//bandwdith
	tallyEnd, bwTotals, err := t.QueryBW(ctx)
	if err != nil {
		errBWA = errs.New("Query for bandwidth failed: %v", err)
	} else if len(bwTotals) > 0 {
		err = t.SaveBWRaw(ctx, tallyEnd, time.Now().UTC(), bwTotals)
		if err != nil {
			errBWA = errs.New("Saving for bandwidth failed : %v", err)
		}
	}

	bucketsData, err = t.QueryBucketBWUsage(ctx, bucketsData)
	if err != nil {
		errBWA = errs.New("QueryBucketBWUsage for bandwidth failed : %v", err)
	}
	errBWA = t.SaveBucketUsageRollup(ctx, tallyEnd, bucketsData)

	return errs.Combine(errAtRest, errBWA)
}

// calculateAtRestData iterates through the pieces on pointerdb and calculates
// the amount of at-rest data stored on each respective node
func (t *Tally) calculateAtRestData(ctx context.Context) (latestTally time.Time, nodeData map[storj.NodeID]float64, bucketsData map[string]accounting.BucketRollup, err error) {
	defer mon.Task()(&ctx)(&err)

	latestTally, err = t.accountingDB.LastTimestamp(ctx, accounting.LastAtRestTally)
	if err != nil {
		return latestTally, nodeData, bucketsData, Error.Wrap(err)
	}
	nodeData = make(map[storj.NodeID]float64)
	bucketsData = make(map[string]accounting.BucketRollup)

	var currentBucket, currentBucketName string
	var bucketCount int64
	var totalStats, currentBucketStats stats
	var currentBucketData accounting.BucketRollup

	err = t.pointerdb.Iterate("", "", true, false,
		func(it storage.Iterator) error {
			var item storage.ListItem
			for it.Next(&item) {

				pointer := &pb.Pointer{}
				err = proto.Unmarshal(item.Value, pointer)
				if err != nil {
					return Error.Wrap(err)
				}

				pathElements := storj.SplitPath(storj.Path(item.Key))
				// check to make sure there are at least *4* path elements. the first three
				// are project, segment, and bucket name, but we want to make sure we're talking
				// about an actual object, and that there's an object name specified

				// handle conditions with buckets with no files
				if len(pathElements) == 3 {
					bucketCount++
				} else if len(pathElements) >= 4 {

					project, segment, bucketName := pathElements[0], pathElements[1], pathElements[2]
					bucketID := storj.JoinPaths(project, bucketName)

					// paths are iterated in order, so everything in a bucket is
					// iterated together. When a project or bucket changes,
					// the previous bucket is completely finished.
					if currentBucket != bucketID {

						if currentBucket != "" {
							// report the previous bucket and add to the totals
							currentBucketStats.Report("bucket")
							totalStats.Combine(&currentBucketStats)
							currentBucketStats = stats{}

							// store the current bucket data
							bucketsData[currentBucketName] = currentBucketData
						}

						currentBucketData = accounting.BucketRollup{
							BucketID:  currentBucketName,
							ProjectID: project,
						}

						currentBucket = bucketID
						currentBucketName = bucketName
					}

					currentBucketStats.AddSegment(pointer, segment == "l")
				}

				currentBucketData = sumBucketData(currentBucketData, pointer)

				remote := pointer.GetRemote()
				if remote == nil {
					continue
				}
				pieces := remote.GetRemotePieces()
				if pieces == nil {
					t.logger.Debug("no pieces on remote segment")
					continue
				}
				segmentSize := pointer.GetSegmentSize()
				redundancy := remote.GetRedundancy()
				if redundancy == nil {
					t.logger.Debug("no redundancy scheme present")
					continue
				}
				minReq := redundancy.GetMinReq()
				if minReq <= 0 {
					t.logger.Debug("pointer minReq must be an int greater than 0")
					continue
				}
				pieceSize := segmentSize / int64(minReq)
				for _, piece := range pieces {
					nodeData[piece.NodeId] += float64(pieceSize)
				}
			}
			return nil
		},
	)
	if err != nil {
		return latestTally, nodeData, bucketsData, Error.Wrap(err)
	}

	if currentBucket != "" {
		// wrap up the last bucket
		totalStats.Combine(&currentBucketStats)
	}
	totalStats.Report("total")
	mon.IntVal("bucket_count").Observe(bucketCount)

	// store the current bucket data for the last bucket
	bucketsData[currentBucketName] = currentBucketData

	if len(nodeData) == 0 {
		return latestTally, nodeData, bucketsData, nil
	}

	//store byte hours, not just bytes
	numHours := time.Now().Sub(latestTally).Hours()
	if latestTally.IsZero() {
		numHours = 1.0 //todo: something more considered?
	}
	latestTally = time.Now()
	for k := range nodeData {
		nodeData[k] *= numHours //calculate byte hours
	}
	return latestTally, nodeData, bucketsData, err
}

func sumBucketData(curr accounting.BucketRollup, pointer *pb.Pointer) accounting.BucketRollup {
	var inline []byte
	var segmentSize int64
	var remoteSegmentCount, inlineSegmentCount, objectsCount uint

	if pointer.GetType() == pb.Pointer_INLINE {

		inline = pointer.GetInlineSegment()
		inlineSegmentCount++
	}

	if pointer.GetType() == pb.Pointer_REMOTE {
		segmentSize = pointer.GetSegmentSize()
		remoteSegmentCount++
	}

	objectsCount++
	metadataSize := uint64(len(pointer.GetMetadata()))

	rollup := accounting.BucketRollup{
		RemoteStoredData: curr.RemoteStoredData + uint64(segmentSize),
		InlineStoredData: curr.InlineStoredData + uint64(len(inline)),
		RemoteSegments:   curr.RemoteSegments + remoteSegmentCount,
		InlineSegments:   curr.InlineSegments + inlineSegmentCount,
		Objects:          curr.Objects + objectsCount,
		MetadataSize:     curr.MetadataSize + metadataSize,
		RollupEndTime:    time.Now(),
	}

	return rollup
}

// SaveAtRestRaw records raw tallies of at-rest-data and updates the LastTimestamp
func (t *Tally) SaveAtRestRaw(ctx context.Context, latestTally time.Time, created time.Time, nodeData map[storj.NodeID]float64) error {
	return t.accountingDB.SaveAtRestRaw(ctx, latestTally, created, nodeData)
}

// QueryBW queries bandwidth allocation database, selecting all new contracts since the last collection run time.
// Grouping by action type, storage node ID and adding total of bandwidth to granular data table.
func (t *Tally) QueryBW(ctx context.Context) (time.Time, map[storj.NodeID][]int64, error) {
	var bwTotals map[storj.NodeID][]int64
	now := time.Now()
	lastBwTally, err := t.accountingDB.LastTimestamp(ctx, accounting.LastBandwidthTally)
	if err != nil {
		return now, bwTotals, Error.Wrap(err)
	}
	bwTotals, err = t.bwAgreementDB.GetTotals(ctx, lastBwTally, now)
	if err != nil {
		return now, bwTotals, Error.Wrap(err)
	}
	if len(bwTotals) == 0 {
		t.logger.Info("Tally found no new bandwidth allocations")
		return now, bwTotals, nil
	}
	return now, bwTotals, nil
}

// SaveBWRaw records granular tallies (sums of bw agreement values) to the database and updates the LastTimestamp
func (t *Tally) SaveBWRaw(ctx context.Context, tallyEnd time.Time, created time.Time, bwTotals map[storj.NodeID][]int64) error {
	return t.accountingDB.SaveBWRaw(ctx, tallyEnd, created, bwTotals)
}

// QueryBucketBWUsage queries the bucketBandwidthUsage table to retrieve records with bucket bwagreement info
func (t *Tally) QueryBucketBWUsage(ctx context.Context, bucketsData map[string]accounting.BucketRollup) (map[string]accounting.BucketRollup, error) {
	for bucketID, bucketRollup := range bucketsData {
		getSum, err := t.sumBucketBWUsage(ctx, bucketID, pb.BandwidthAction_GET)
		if err != nil {
			return nil, err
		}
		bucketRollup.GetEgress = getSum

		auditSum, err := t.sumBucketBWUsage(ctx, bucketID, pb.BandwidthAction_GET_AUDIT)
		if err != nil {
			return nil, err
		}
		bucketRollup.AuditEgress = auditSum

		repairSum, err := t.sumBucketBWUsage(ctx, bucketID, pb.BandwidthAction_GET_REPAIR)
		if err != nil {
			return nil, err
		}
		bucketRollup.RepairEgress = repairSum
	}

	return bucketsData, nil
}

func (t *Tally) sumBucketBWUsage(ctx context.Context, bucketID string, action pb.BandwidthAction) (uint64, error) {
	var sum uint64
	bwUsageRecords, err := t.bucketBWUsageDB.GetAllByBucketIDAndAction(ctx, bucketID, action)
	if err != nil {
		return sum, err
	}

	// For each bwUsageRecords, sum the total
	for _, record := range bwUsageRecords {
		sum += uint64(record.Total)
	}
	return sum, nil
}

// SaveBucketUsageRollup records granular tallies (sums of bw agreement values) to the database and updates the LastTimestamp
func (t *Tally) SaveBucketUsageRollup(ctx context.Context, tallyEnd time.Time, bucketsData map[string]accounting.BucketRollup) error {
	for _, rollup := range bucketsData {
		rollup.RollupEndTime = tallyEnd
		if _, err := t.bucketUsageDB.Create(ctx, rollup); err != nil {
			return err
		}
	}
	return nil
}
