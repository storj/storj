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
	"storj.io/storj/pkg/overlay"
	"storj.io/storj/pkg/pb"
	"storj.io/storj/pkg/pointerdb"
	"storj.io/storj/pkg/storj"
	"storj.io/storj/storage"
)

// Config contains configurable values for the tally service
type Config struct {
	Interval time.Duration `help:"how frequently the tally service should run" default:"1h" devDefault:"30s"`
}

// Service is the tally service for data stored on each storage node
type Service struct {
	logger       *zap.Logger
	pointerdb    *pointerdb.Service
	overlay      *overlay.Cache
	limit        int
	ticker       *time.Ticker
	accountingDB accounting.DB
}

// New creates a new tally Service
func New(logger *zap.Logger, accountingDB accounting.DB, pointerdb *pointerdb.Service, overlay *overlay.Cache, limit int, interval time.Duration) *Service {
	return &Service{
		logger:       logger,
		pointerdb:    pointerdb,
		overlay:      overlay,
		limit:        limit,
		ticker:       time.NewTicker(interval),
		accountingDB: accountingDB,
	}
}

// Run the tally service loop
func (t *Service) Run(ctx context.Context) (err error) {
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

//Tally calculates data-at-rest once
func (t *Service) Tally(ctx context.Context) error {
	var errAtRest error
	latestTally, nodeData, err := t.calculateAtRestData(ctx)
	if err != nil {
		errAtRest = errs.New("Query for data-at-rest failed : %v", err)
	} else {
		if len(nodeData) > 0 {
			err = t.SaveAtRestRaw(ctx, latestTally, time.Now().UTC(), nodeData)
			if err != nil {
				errAtRest = errs.New("Saving storage node data-at-rest failed : %v", err)
			}
		}
		if len(bucketData) > 0 {
			err = t.accountingDB.SaveBucketTallies(ctx, latestTally, bucketData)
			if err != nil {
				errBucketInfo = errs.New("Saving bucket storage data failed")
			}
		}
	}
	return errAtRest
}

// calculateAtRestData iterates through the pieces on pointerdb and calculates
// the amount of at-rest data stored on each respective node
func (t *Service) calculateAtRestData(ctx context.Context) (latestTally time.Time, nodeData map[storj.NodeID]float64, err error) {
	defer mon.Task()(&ctx)(&err)

	latestTally, err = t.accountingDB.LastTimestamp(ctx, accounting.LastAtRestTally)
	if err != nil {
		return latestTally, nodeData, bucketTallies, Error.Wrap(err)
	}
	nodeData = make(map[storj.NodeID]float64)
	bucketTallies = make(map[string]*accounting.BucketTally)

	var currentBucket string
	var bucketCount int64
	var totalTallies, currentBucketTally accounting.BucketTally

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
							currentBucketTally.Report("bucket")
							totalTallies.Combine(&currentBucketTally)

							// add currentBucketTally to bucketTallies
							bucketTallies[currentBucket] = &currentBucketTally
							currentBucketTally = accounting.BucketTally{}
						}
						currentBucket = bucketID
					}

					currentBucketTally.AddSegment(pointer, segment == "l")
				}

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
		return latestTally, nodeData, bucketTallies, Error.Wrap(err)
	}

	if currentBucket != "" {
		// wrap up the last bucket
		totalTallies.Combine(&currentBucketTally)
		bucketTallies[currentBucket] = &currentBucketTally
	}
	totalTallies.Report("total")
	mon.IntVal("bucket_count").Observe(bucketCount)

	if len(nodeData) == 0 {
		return latestTally, nodeData, bucketTallies, nil
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
	return latestTally, nodeData, bucketTallies, err
}

// SaveAtRestRaw records raw tallies of at-rest-data and updates the LastTimestamp
func (t *Service) SaveAtRestRaw(ctx context.Context, latestTally time.Time, created time.Time, nodeData map[storj.NodeID]float64) error {
	return t.accountingDB.SaveAtRestRaw(ctx, latestTally, created, nodeData)
}
