// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package tally

import (
	"context"
	"time"

	"github.com/gogo/protobuf/proto"
	"github.com/skyrings/skyring-common/tools/uuid"
	"github.com/zeebo/errs"
	"go.uber.org/zap"

	"storj.io/storj/pkg/pb"
	"storj.io/storj/pkg/storj"
	"storj.io/storj/satellite/accounting"
	"storj.io/storj/satellite/accounting/live"
	"storj.io/storj/satellite/metainfo"
	"storj.io/storj/satellite/overlay"
	"storj.io/storj/storage"
)

// Config contains configurable values for the tally service
type Config struct {
	Interval time.Duration `help:"how frequently the tally service should run" releaseDefault:"1h" devDefault:"30s"`
}

// Service is the tally service for data stored on each storage node
type Service struct {
	logger                  *zap.Logger
	metainfo                *metainfo.Service
	overlay                 *overlay.Cache
	limit                   int
	ticker                  *time.Ticker
	storagenodeAccountingDB accounting.StoragenodeAccounting
	projectAccountingDB     accounting.ProjectAccounting
	liveAccounting          live.Service
}

// New creates a new tally Service
func New(logger *zap.Logger, sdb accounting.StoragenodeAccounting, pdb accounting.ProjectAccounting, liveAccounting live.Service, metainfo *metainfo.Service, overlay *overlay.Cache, limit int, interval time.Duration) *Service {
	return &Service{
		logger:                  logger,
		metainfo:                metainfo,
		overlay:                 overlay,
		limit:                   limit,
		ticker:                  time.NewTicker(interval),
		storagenodeAccountingDB: sdb,
		projectAccountingDB:     pdb,
		liveAccounting:          liveAccounting,
	}
}

// Run the tally service loop
func (t *Service) Run(ctx context.Context) (err error) {
	defer mon.Task()(&ctx)(&err)
	t.logger.Info("Tally service starting up")

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

// Tally calculates data-at-rest usage once
func (t *Service) Tally(ctx context.Context) (err error) {
	defer mon.Task()(&ctx)(&err)
	// The live accounting store will only keep a delta to space used relative
	// to the latest tally. Since a new tally is beginning, we will zero it out
	// now. There is a window between this call and the point where the tally DB
	// transaction starts, during which some changes in space usage may be
	// double-counted (counted in the tally and also counted as a delta to
	// the tally). If that happens, it will be fixed at the time of the next
	// tally run.
	t.liveAccounting.ResetTotals()

	var errAtRest, errBucketInfo error
	latestTally, nodeData, bucketData, err := t.CalculateAtRestData(ctx)
	if err != nil {
		errAtRest = errs.New("Query for data-at-rest failed : %v", err)
	} else {
		if len(nodeData) > 0 {
			err = t.storagenodeAccountingDB.SaveTallies(ctx, latestTally, nodeData)
			if err != nil {
				errAtRest = errs.New("Saving storage node data-at-rest failed : %v", err)
			}
		}

		if len(bucketData) > 0 {
			_, err = t.projectAccountingDB.SaveTallies(ctx, latestTally, bucketData)
			if err != nil {
				errBucketInfo = errs.New("Saving bucket storage data failed")
			}
		}
	}
	return errs.Combine(errAtRest, errBucketInfo)
}

// CalculateAtRestData iterates through the pieces on metainfo and calculates
// the amount of at-rest data stored in each bucket and on each respective node
func (t *Service) CalculateAtRestData(ctx context.Context) (latestTally time.Time, nodeData map[storj.NodeID]float64, bucketTallies map[string]*accounting.BucketTally, err error) {
	defer mon.Task()(&ctx)(&err)

	latestTally, err = t.storagenodeAccountingDB.LastTimestamp(ctx, accounting.LastAtRestTally)
	if err != nil {
		return latestTally, nodeData, bucketTallies, Error.Wrap(err)
	}
	nodeData = make(map[storj.NodeID]float64)
	bucketTallies = make(map[string]*accounting.BucketTally)

	var totalTallies accounting.BucketTally

	err = t.metainfo.Iterate(ctx, "", "", true, false,
		func(ctx context.Context, it storage.Iterator) error {
			var item storage.ListItem
			for it.Next(ctx, &item) {

				pointer := &pb.Pointer{}
				err = proto.Unmarshal(item.Value, pointer)
				if err != nil {
					return Error.Wrap(err)
				}

				pathElements := storj.SplitPath(storj.Path(item.Key))
				// check to make sure there are at least *4* path elements. the first three
				// are project, segment, and bucket name, but we want to make sure we're talking
				// about an actual object, and that there's an object name specified
				if len(pathElements) >= 4 {
					project, segment, bucketName := pathElements[0], pathElements[1], pathElements[2]

					bucketID := storj.JoinPaths(project, bucketName)

					bucketTally := bucketTallies[bucketID]
					projectID, err := uuid.Parse(project)
					if err != nil {
						return Error.Wrap(err)
					}
					if bucketTally == nil {
						bucketTally = &accounting.BucketTally{}
						bucketTally.ProjectID = projectID[:]
						bucketTally.BucketName = []byte(bucketName)

						bucketTallies[bucketID] = bucketTally
					}

					bucketTally.AddSegment(pointer, segment == "l")
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

	for _, bucketTally := range bucketTallies {
		bucketTally.Report("bucket")
		totalTallies.Combine(bucketTally)
	}

	totalTallies.Report("total")

	//store byte hours, not just bytes
	numHours := time.Now().Sub(latestTally).Hours()
	if latestTally.IsZero() {
		numHours = 1.0 //todo: something more considered?
	}
	latestTally = time.Now().UTC()

	if len(nodeData) == 0 {
		return latestTally, nodeData, bucketTallies, nil
	}
	for k := range nodeData {
		nodeData[k] *= numHours //calculate byte hours
	}
	return latestTally, nodeData, bucketTallies, err
}
