// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package tally

import (
	"context"
	"time"

	"github.com/spacemonkeygo/monkit/v3"
	"github.com/zeebo/errs"
	"go.uber.org/zap"

	"storj.io/common/sync2"
	"storj.io/common/uuid"
	"storj.io/storj/satellite/accounting"
	"storj.io/storj/satellite/buckets"
	"storj.io/storj/satellite/metabase"
)

// Error is a standard error class for this package.
var (
	Error = errs.Class("tally")
	mon   = monkit.Package()
)

// Config contains configurable values for the tally service.
type Config struct {
	Interval             time.Duration `help:"how frequently the tally service should run" releaseDefault:"1h" devDefault:"30s" testDefault:"$TESTINTERVAL"`
	SaveRollupBatchSize  int           `help:"how large of batches SaveRollup should process at a time" default:"1000"`
	ReadRollupBatchSize  int           `help:"how large of batches GetBandwidthSince should process at a time" default:"10000"`
	UseRangedLoop        bool          `help:"whether to enable node tally with ranged loop" default:"true"`
	SaveTalliesBatchSize int           `help:"how large should be insert into tallies" default:"10000"`

	ListLimit          int           `help:"how many buckets to query in a batch" default:"2500"`
	AsOfSystemInterval time.Duration `help:"as of system interval" releaseDefault:"-5m" devDefault:"-1us" testDefault:"-1us"`
}

// Service is the tally service for data stored on each storage node.
//
// architecture: Chore
type Service struct {
	log    *zap.Logger
	config Config
	Loop   *sync2.Cycle

	metabase                *metabase.DB
	bucketsDB               buckets.DB
	liveAccounting          accounting.Cache
	storagenodeAccountingDB accounting.StoragenodeAccounting
	projectAccountingDB     accounting.ProjectAccounting
	nowFn                   func() time.Time
}

// New creates a new tally Service.
func New(log *zap.Logger, sdb accounting.StoragenodeAccounting, pdb accounting.ProjectAccounting, liveAccounting accounting.Cache, metabase *metabase.DB, bucketsDB buckets.DB, config Config) *Service {
	return &Service{
		log:    log,
		config: config,
		Loop:   sync2.NewCycle(config.Interval),

		metabase:                metabase,
		bucketsDB:               bucketsDB,
		liveAccounting:          liveAccounting,
		storagenodeAccountingDB: sdb,
		projectAccountingDB:     pdb,
		nowFn:                   time.Now,
	}
}

// Run the tally service loop.
func (service *Service) Run(ctx context.Context) (err error) {
	defer mon.Task()(&ctx)(&err)

	return service.Loop.Run(ctx, func(ctx context.Context) error {
		err := service.Tally(ctx)
		if err != nil {
			service.log.Error("tally failed", zap.Error(err))

			mon.Event("bucket_tally_error") //mon:locked
		}
		return nil
	})
}

// Close stops the service and releases any resources.
func (service *Service) Close() error {
	service.Loop.Close()
	return nil
}

// SetNow allows tests to have the Service act as if the current time is whatever
// they want. This avoids races and sleeping, making tests more reliable and efficient.
func (service *Service) SetNow(now func() time.Time) {
	service.nowFn = now
}

// Tally calculates data-at-rest usage once.
//
// How live accounting is calculated:
//
// At the beginning of the tally iteration, we get a map containing the current
// project totals from the cache- initialLiveTotals (our current estimation of
// the project totals). At the end of the tally iteration, we have the totals
// from what we saw during the metainfo loop.
//
// However, data which was uploaded during the loop may or may not have been
// seen in the metainfo loop. For this reason, we also read the live accounting
// totals again at the end of the tally iteration- latestLiveTotals.
//
// The difference between latest and initial indicates how much data was
// uploaded during the metainfo loop and is assigned to delta. However, again,
// we aren't certain how much of the delta is accounted for in the metainfo
// totals. For the reason we make an assumption that 50% of the data is
// accounted for. So to calculate the new live accounting totals, we sum the
// metainfo totals and 50% of the deltas.
func (service *Service) Tally(ctx context.Context) (err error) {
	defer mon.Task()(&ctx)(&err)

	// No-op unless that there isn't an error getting the
	// liveAccounting.GetAllProjectTotals
	updateLiveAccountingTotals := func(_ map[uuid.UUID]accounting.Usage) {}

	initialLiveTotals, err := service.liveAccounting.GetAllProjectTotals(ctx)
	if err != nil {
		service.log.Error(
			"tally won't update the live accounting storage usages of the projects in this cycle",
			zap.Error(err),
		)
	} else {
		updateLiveAccountingTotals = func(tallyProjectTotals map[uuid.UUID]accounting.Usage) {
			latestLiveTotals, err := service.liveAccounting.GetAllProjectTotals(ctx)
			if err != nil {
				service.log.Error(
					"tally isn't updating the live accounting storage usages of the projects in this cycle",
					zap.Error(err),
				)
				return
			}

			// empty projects are not returned by the metainfo observer. If a project exists
			// in live accounting, but not in tally projects, we would not update it in live accounting.
			// Thus, we add them and set the total to 0.
			for projectID := range latestLiveTotals {
				if _, ok := tallyProjectTotals[projectID]; !ok {
					tallyProjectTotals[projectID] = accounting.Usage{}
				}
			}

			for projectID, tallyTotal := range tallyProjectTotals {
				delta := latestLiveTotals[projectID].Storage - initialLiveTotals[projectID].Storage
				if delta < 0 {
					delta = 0
				}

				// read the method documentation why the increase passed to this method
				// is calculated in this way
				err = service.liveAccounting.AddProjectStorageUsage(ctx, projectID, -latestLiveTotals[projectID].Storage+tallyTotal.Storage+(delta/2))
				if err != nil {
					if accounting.ErrSystemOrNetError.Has(err) {
						service.log.Error(
							"tally isn't updating the live accounting storage usages of the projects in this cycle",
							zap.Error(err),
						)
						return
					}

					service.log.Error(
						"tally isn't updating the live accounting storage usage of the project in this cycle",
						zap.String("projectID", projectID.String()),
						zap.Error(err),
					)
				}

				// difference between cached project totals and latest tally collector
				increment := tallyTotal.Segments - latestLiveTotals[projectID].Segments

				err = service.liveAccounting.UpdateProjectSegmentUsage(ctx, projectID, increment)
				if err != nil {
					if accounting.ErrSystemOrNetError.Has(err) {
						service.log.Error(
							"tally isn't updating the live accounting segment usages of the projects in this cycle",
							zap.Error(err),
						)
						return
					}

					service.log.Error(
						"tally isn't updating the live accounting segment usage of the project in this cycle",
						zap.String("projectID", projectID.String()),
						zap.Error(err),
					)
				}
			}
		}
	}

	// add up all buckets
	collector := NewBucketTallyCollector(service.log.Named("observer"), service.nowFn(), service.metabase, service.bucketsDB, service.projectAccountingDB, service.config)
	err = collector.Run(ctx)
	if err != nil {
		return Error.Wrap(err)
	}

	if len(collector.Bucket) == 0 {
		return nil
	}

	// save the new results
	var errAtRest errs.Group

	// record bucket tallies to DB
	// TODO we should be able replace map with just slice
	intervalStart := service.nowFn()
	buffer := map[metabase.BucketLocation]*accounting.BucketTally{}
	for location, tally := range collector.Bucket {
		buffer[location] = tally

		if len(buffer) >= service.config.SaveTalliesBatchSize {
			// don't stop on error, we would like to store as much as possible
			errAtRest.Add(service.flushTallies(ctx, intervalStart, buffer))

			for key := range buffer {
				delete(buffer, key)
			}
		}
	}

	errAtRest.Add(service.flushTallies(ctx, intervalStart, buffer))

	updateLiveAccountingTotals(projectTotalsFromBuckets(collector.Bucket))

	var total accounting.BucketTally
	// TODO for now we don't have access to inline/remote stats per bucket
	// but that may change in the future. To get back those stats we would
	// most probably need to add inline/remote information to object in
	// metabase. We didn't decide yet if that is really needed right now.
	for _, bucket := range collector.Bucket {
		monAccounting.IntVal("bucket_objects").Observe(bucket.ObjectCount) //mon:locked
		monAccounting.IntVal("bucket_segments").Observe(bucket.Segments()) //mon:locked
		// monAccounting.IntVal("bucket_inline_segments").Observe(bucket.InlineSegments) //mon:locked
		// monAccounting.IntVal("bucket_remote_segments").Observe(bucket.RemoteSegments) //mon:locked

		monAccounting.IntVal("bucket_bytes").Observe(bucket.Bytes()) //mon:locked
		// monAccounting.IntVal("bucket_inline_bytes").Observe(bucket.InlineBytes) //mon:locked
		// monAccounting.IntVal("bucket_remote_bytes").Observe(bucket.RemoteBytes) //mon:locked
		total.Combine(bucket)
	}
	monAccounting.IntVal("total_objects").Observe(total.ObjectCount) //mon:locked
	monAccounting.IntVal("total_segments").Observe(total.Segments()) //mon:locked
	monAccounting.IntVal("total_bytes").Observe(total.Bytes())       //mon:locked
	monAccounting.IntVal("total_pending_objects").Observe(total.PendingObjectCount)

	return errAtRest.Err()
}

func (service *Service) flushTallies(ctx context.Context, intervalStart time.Time, tallies map[metabase.BucketLocation]*accounting.BucketTally) error {
	err := service.projectAccountingDB.SaveTallies(ctx, intervalStart, tallies)
	if err != nil {
		return Error.New("ProjectAccounting.SaveTallies failed: %v", err)
	}
	return nil
}

// BucketTallyCollector collects and adds up tallies for buckets.
type BucketTallyCollector struct {
	Now    time.Time
	Log    *zap.Logger
	Bucket map[metabase.BucketLocation]*accounting.BucketTally

	metabase            *metabase.DB
	bucketsDB           buckets.DB
	projectAccountingDB accounting.ProjectAccounting
	config              Config
}

// NewBucketTallyCollector returns a collector that adds up totals for buckets.
// The now argument controls when the collector considers objects to be expired.
func NewBucketTallyCollector(log *zap.Logger, now time.Time, db *metabase.DB, bucketsDB buckets.DB, projectAccountingDB accounting.ProjectAccounting, config Config) *BucketTallyCollector {
	return &BucketTallyCollector{
		Now:    now,
		Log:    log,
		Bucket: make(map[metabase.BucketLocation]*accounting.BucketTally),

		metabase:            db,
		bucketsDB:           bucketsDB,
		projectAccountingDB: projectAccountingDB,
		config:              config,
	}
}

// Run runs collecting bucket tallies.
func (observer *BucketTallyCollector) Run(ctx context.Context) (err error) {
	defer mon.Task()(&ctx)(&err)

	startingTime, err := observer.metabase.Now(ctx)
	if err != nil {
		return err
	}

	return observer.fillBucketTallies(ctx, startingTime)
}

// fillBucketTallies collects all bucket tallies and fills observer's buckets map with results.
func (observer *BucketTallyCollector) fillBucketTallies(ctx context.Context, startingTime time.Time) (err error) {
	defer mon.Task()(&ctx)(&err)

	var lastBucketLocation metabase.BucketLocation
	for {
		more, err := observer.bucketsDB.IterateBucketLocations(ctx, lastBucketLocation.ProjectID, lastBucketLocation.BucketName, observer.config.ListLimit, func(bucketLocations []metabase.BucketLocation) (err error) {
			fromBucket := bucketLocations[0]
			toBucket := bucketLocations[len(bucketLocations)-1]

			// Prepopulate the results with empty tallies. Otherwise, empty buckets will be unaccounted for
			// since they're not reached when iterating over objects in the metainfo DB.
			// We only do this for buckets whose last tally is non-empty because only one empty tally is
			// required for us to know that a bucket was empty the last time we checked.
			locs, err := observer.projectAccountingDB.GetNonEmptyTallyBucketsInRange(ctx, fromBucket, toBucket)
			if err != nil {
				return err
			}
			for _, loc := range locs {
				observer.Bucket[loc] = &accounting.BucketTally{BucketLocation: loc}
			}

			tallies, err := observer.metabase.CollectBucketTallies(ctx, metabase.CollectBucketTallies{
				From:               fromBucket,
				To:                 toBucket,
				AsOfSystemTime:     startingTime,
				AsOfSystemInterval: observer.config.AsOfSystemInterval,
				Now:                observer.Now,
			})
			if err != nil {
				return err
			}

			for _, tally := range tallies {
				bucket := observer.ensureBucket(tally.BucketLocation)
				bucket.TotalSegments = tally.TotalSegments
				bucket.TotalBytes = tally.TotalBytes
				bucket.MetadataSize = tally.MetadataSize
				bucket.ObjectCount = tally.ObjectCount
				bucket.PendingObjectCount = tally.PendingObjectCount
			}

			lastBucketLocation = bucketLocations[len(bucketLocations)-1]
			return nil
		})
		if err != nil {
			return err
		}
		if !more {
			break
		}
	}

	return nil
}

// ensureBucket returns bucket corresponding to the passed in path.
func (observer *BucketTallyCollector) ensureBucket(location metabase.BucketLocation) *accounting.BucketTally {
	bucket, exists := observer.Bucket[location]
	if !exists {
		bucket = &accounting.BucketTally{}
		bucket.BucketLocation = location
		observer.Bucket[location] = bucket
	}

	return bucket
}

func projectTotalsFromBuckets(buckets map[metabase.BucketLocation]*accounting.BucketTally) map[uuid.UUID]accounting.Usage {
	projectTallyTotals := make(map[uuid.UUID]accounting.Usage)
	for _, bucket := range buckets {
		projectUsage := projectTallyTotals[bucket.ProjectID]
		projectUsage.Storage += bucket.TotalBytes
		projectUsage.Segments += bucket.TotalSegments
		projectTallyTotals[bucket.ProjectID] = projectUsage
	}
	return projectTallyTotals
}

// using custom name to avoid breaking monitoring.
var monAccounting = monkit.ScopeNamed("storj.io/storj/satellite/accounting")
