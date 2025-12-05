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
	"storj.io/storj/satellite/entitlements"
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
	RetentionDays        int           `help:"how many days to retain tallies or zero to retain indefinitely" default:"365"`

	ListLimit            int           `help:"how many buckets to query in a batch" default:"2500"`
	AsOfSystemInterval   time.Duration `help:"as of system interval" releaseDefault:"-5m" devDefault:"-1us" testDefault:"-1us"`
	FixedReadTimestamp   bool          `help:"whether to use fixed (start of process) timestamp for DB reads from objects table" default:"true" testDefault:"false"`
	UsePartitionQuery    bool          `help:"whether to use partition query for DB reads from objects table" default:"false"`
	SmallObjectRemainder bool          `help:"whether to enable small object remainder accounting" default:"false"`
}

// ProductUsagePriceModel is defined to avoid import cycles.
type ProductUsagePriceModel struct {
	ProductID             int32
	StorageRemainderBytes int64
}

// PlacementProductMap is a global mapping of placement to product ID.
type PlacementProductMap map[int]int32

// GetProductByPlacement returns the product ID for a placement.
func (p PlacementProductMap) GetProductByPlacement(placement int) (int32, bool) {
	productID, ok := p[placement]
	return productID, ok
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
	productPrices           map[int32]ProductUsagePriceModel
	globalPlacementMap      PlacementProductMap
	nowFn                   func() time.Time
}

// New creates a new tally Service.
func New(log *zap.Logger, sdb accounting.StoragenodeAccounting, pdb accounting.ProjectAccounting, liveAccounting accounting.Cache, metabase *metabase.DB, bucketsDB buckets.DB, config Config, productPrices map[int32]ProductUsagePriceModel, globalPlacementMap PlacementProductMap) *Service {
	return &Service{
		log:    log,
		config: config,
		Loop:   sync2.NewCycle(config.Interval),

		metabase:                metabase,
		bucketsDB:               bucketsDB,
		liveAccounting:          liveAccounting,
		storagenodeAccountingDB: sdb,
		projectAccountingDB:     pdb,
		productPrices:           productPrices,
		globalPlacementMap:      globalPlacementMap,
		nowFn:                   time.Now,
	}
}

// Run the tally service loop.
func (service *Service) Run(ctx context.Context) (err error) {
	defer mon.Task()(&ctx)(&err)

	return service.Loop.Run(ctx, func(ctx context.Context) error {
		if err := service.Tally(ctx); err != nil {
			service.log.Error("tally failed", zap.Error(err))

			mon.Event("bucket_tally_error")
		}

		if err := service.Purge(ctx); err != nil {
			service.log.Error("tally purge failed", zap.Error(err))

			mon.Event("bucket_tally_purge_error")
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

				// read the method documentation why the increase is calculated in this way.
				storageIncr := -latestLiveTotals[projectID].Storage + tallyTotal.Storage + (delta / 2)
				// difference between cached project totals and latest tally collector.
				segmentIncr := tallyTotal.Segments - latestLiveTotals[projectID].Segments

				err = service.liveAccounting.UpdateProjectStorageAndSegmentUsage(ctx, projectID, storageIncr, segmentIncr)
				if err != nil {
					if accounting.ErrSystemOrNetError.Has(err) {
						service.log.Error(
							"tally isn't updating the live accounting storage or segment usages of the projects in this cycle",
							zap.Error(err),
						)
						return
					}

					service.log.Error(
						"tally isn't updating the live accounting storage or segment usage of the project in this cycle",
						zap.String("projectID", projectID.String()),
						zap.Error(err),
					)
				}
			}
		}
	}

	// add up all buckets
	collector := NewBucketTallyCollector(service.log.Named("observer"), service.nowFn(), service.metabase, service.bucketsDB, service.projectAccountingDB, service.productPrices, service.globalPlacementMap, service.config)
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
		monAccounting.IntVal("bucket_objects").Observe(bucket.ObjectCount)
		monAccounting.IntVal("bucket_segments").Observe(bucket.Segments())
		// monAccounting.IntVal("bucket_inline_segments").Observe(bucket.InlineSegments)
		// monAccounting.IntVal("bucket_remote_segments").Observe(bucket.RemoteSegments)

		monAccounting.IntVal("bucket_bytes").Observe(bucket.Bytes())
		// monAccounting.IntVal("bucket_inline_bytes").Observe(bucket.InlineBytes)
		// monAccounting.IntVal("bucket_remote_bytes").Observe(bucket.RemoteBytes)
		total.Combine(bucket)
	}
	monAccounting.IntVal("total_objects").Observe(total.ObjectCount)
	monAccounting.IntVal("total_segments").Observe(total.Segments())
	monAccounting.IntVal("total_bytes").Observe(total.Bytes())
	monAccounting.IntVal("total_pending_objects").Observe(total.PendingObjectCount)
	monAccounting.IntVal("total_metadata_size").Observe(total.MetadataSize)

	return errAtRest.Err()
}

// Purge removes tallies older than the retention period.
func (service *Service) Purge(ctx context.Context) (err error) {
	defer mon.Task()(&ctx)(&err)

	if service.config.RetentionDays == 0 {
		return nil
	}

	olderThan := service.nowFn().AddDate(0, 0, -service.config.RetentionDays)
	count, err := service.projectAccountingDB.DeleteTalliesBefore(ctx, olderThan)
	if err != nil {
		return Error.New("ProjectAccounting.DeleteTalliesOlderThan failed: %v", err)
	}
	monAccounting.IntVal("bucket_tallies_purged").Observe(count)
	if count > 0 {
		service.log.Info("Purged old bucket storage tallies", zap.Time("olderThan", olderThan), zap.Int64("count (estimation)", count))
	}
	return nil
}

func (service *Service) flushTallies(ctx context.Context, intervalStart time.Time, tallies map[metabase.BucketLocation]*accounting.BucketTally) error {
	if err := service.projectAccountingDB.SaveTallies(ctx, intervalStart, tallies); err != nil {
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
	productPrices       map[int32]ProductUsagePriceModel
	globalPlacementMap  PlacementProductMap
	config              Config
}

// NewBucketTallyCollector returns a collector that adds up totals for buckets.
// The now argument controls when the collector considers objects to be expired.
func NewBucketTallyCollector(log *zap.Logger, now time.Time, db *metabase.DB, bucketsDB buckets.DB, projectAccountingDB accounting.ProjectAccounting, productPrices map[int32]ProductUsagePriceModel, globalPlacementMap PlacementProductMap, config Config) *BucketTallyCollector {
	return &BucketTallyCollector{
		Now:    now,
		Log:    log,
		Bucket: make(map[metabase.BucketLocation]*accounting.BucketTally),

		metabase:            db,
		bucketsDB:           bucketsDB,
		projectAccountingDB: projectAccountingDB,
		productPrices:       productPrices,
		globalPlacementMap:  globalPlacementMap,
		config:              config,
	}
}

// Run runs collecting bucket tallies.
func (observer *BucketTallyCollector) Run(ctx context.Context) (err error) {
	defer mon.Task()(&ctx)(&err)

	return observer.fillBucketTallies(ctx)
}

// fillBucketTallies collects all bucket tallies and fills observer's buckets map with results.
func (observer *BucketTallyCollector) fillBucketTallies(ctx context.Context) (err error) {
	defer mon.Task()(&ctx)(&err)

	startTime := time.Time{}
	if observer.config.FixedReadTimestamp {
		startTime = time.Now()
	}

	// N.B.: IterateBucketLocations only iterates buckets that are not deleted
	// but we may need to create zero tallies for deleted buckets!
	// So we're going to use IterateBucketLocations to pace ourselves. We're
	// going to keep track of the max bucket we saw from the previous page and
	// use that as the beginning of the range when we consider the next page.
	// This means we won't accidentally skip anything and all possible buckets
	// will be included in the fence posts we provide.  This also means we will
	// need to do a final sweep from the max to the end after all pages are done.
	//
	// As a concrete example to make this problem more understandable,
	// imagine that we have the following live buckets:
	//
	//  * apivorous
	//  * clearness
	//  * corymbed
	//  * dekastere
	//  * indomitable
	//  * moatlike
	//  * peripyloric
	//  * relocator
	//  * schizonts
	//  * steelmaking
	//
	// Further, imagine that the following buckets were recently deleted:
	//  * acanthopteran
	//  * cowishness
	//  * jargoner
	//  * scientarium
	//  * yokeableness
	//
	// IterateBucketLocations only looks at live buckets. So let's say we call
	// IterateBucketLocations and get two pages of 5 buckets each. The first
	// page will give us apivorous to indomitable, and the second page will give
	// us moatlike to steelmaking. So if we then call
	// `GetPreviouslyNonEmptyTallyBucketsInRange` or `GetBucketsWithEntitlementsInRange`
	// with these ranges (apivorous through indomitable, moatlike through steelmaking), we will
	// get cowishness and scientarium, but we will *not* pick up acanthoperan,
	// jargoner, or yokeableness. acanthoperan is before the first range,
	// jargoner is between the two ranges, and yokeableness is after the last
	// range.
	//
	// So instead what we're going to do is start from the first possible
	// bucket and go through indomitable, then from *indomitable* through
	// steelmaking (not moatlike), and finally a special call to go from
	// steelmaking through the end.
	//
	// A great question to ask (I'm asking myself right now!) is whether we
	// should even use IterateBucketLocations at all! Why not just make
	// GetPreviouslyNonEmptyTallyBucketsInRange or GetBucketsWithEntitlementsInRange page?

	// we're going to start with the maxBucket we've seen so far to be the min
	// bucket possible.
	maxBucketSoFar := metabase.BucketLocation{
		ProjectID:  uuid.UUID{},
		BucketName: "",
	}

	// this is the function for handling a bucket page
	bucketPageHandler := func(bucketLocations []metabase.BucketLocation) (err error) {
		fromBucket := maxBucketSoFar
		toBucket := bucketLocations[len(bucketLocations)-1]
		maxBucketSoFar = toBucket

		// Prepopulate the results with empty tallies. Otherwise, empty buckets will be unaccounted for
		// since they're not reached when iterating over objects in the metainfo DB.
		// We only do this for buckets whose last tally is non-empty because only one empty tally is
		// required for us to know that a bucket was empty the last time we checked.
		//
		// When SmallObjectRemainder is enabled, all unique remainder values in the range are collected
		// and passed to CollectBucketTallies, which calculates all remainder variants in a single query.
		if observer.config.SmallObjectRemainder {
			err = observer.fillTalliesWithStorageRemainder(ctx, fromBucket, toBucket, startTime)
			if err != nil {
				return err
			}
		} else {
			locs, err := observer.projectAccountingDB.GetPreviouslyNonEmptyTallyBucketsInRange(ctx, fromBucket, toBucket, observer.config.AsOfSystemInterval)
			if err != nil {
				return err
			}
			for _, loc := range locs {
				observer.Bucket[loc] = &accounting.BucketTally{BucketLocation: loc}
			}

			tallies, err := observer.metabase.CollectBucketTallies(ctx, metabase.CollectBucketTallies{
				From:               fromBucket,
				To:                 toBucket,
				AsOfSystemTime:     startTime,
				AsOfSystemInterval: observer.config.AsOfSystemInterval,
				Now:                observer.Now,
				UsePartitionQuery:  observer.config.UsePartitionQuery,
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
		}

		return nil
	}

	// iterate through all live bucket pages
	err = observer.bucketsDB.IterateBucketLocations(ctx, observer.config.ListLimit, bucketPageHandler)
	if err != nil {
		return err
	}
	// now go from the last bucket to the end of the keyspace
	return bucketPageHandler([]metabase.BucketLocation{
		maxBucketSoFar,
		{
			ProjectID: uuid.Max(),
			// no project can be uuid.Max(), so even though we don't have a specific
			// bucket name, the max project id effectively helps us clear the full
			// keyspace.
			BucketName: "",
		}})
}

// fillTalliesWithStorageRemainder collects bucket tallies with storage remainder accounting.
// It uses a single query that calculates all remainder values simultaneously.
func (observer *BucketTallyCollector) fillTalliesWithStorageRemainder(ctx context.Context, fromBucket, toBucket metabase.BucketLocation, startTime time.Time) error {
	// Get ALL buckets in range (both with and without previous tallies) along with their placement/entitlements.
	bucketsWithEntitlements, err := observer.projectAccountingDB.GetBucketsWithEntitlementsInRange(ctx, fromBucket, toBucket, entitlements.ProjectScopePrefix)
	if err != nil {
		return err
	}

	// Map each bucket to its remainder value based on placement and entitlements.
	bucketToRemainder := make(map[metabase.BucketLocation]int64)
	uniqueRemainders := make(map[int64]bool)

	for _, bucket := range bucketsWithEntitlements {
		// Only prepopulate buckets that had objects in a previous tally.
		// This matches the behavior of the non-SmallObjectRemainder path.
		if bucket.HasPreviousTally {
			observer.Bucket[bucket.Location] = &accounting.BucketTally{BucketLocation: bucket.Location}
		}

		// Resolve product ID from placement.
		var productID int32
		if bucket.ProjectFeatures.PlacementProductMappings != nil {
			// Check project-specific entitlements first.
			if pid, ok := bucket.ProjectFeatures.PlacementProductMappings[bucket.Placement]; ok {
				productID = pid
			} else if observer.globalPlacementMap != nil {
				// Fall back to global placement map.
				productID, _ = observer.globalPlacementMap.GetProductByPlacement(int(bucket.Placement))
			}
		} else if observer.globalPlacementMap != nil {
			// No entitlements, use global placement map.
			productID, _ = observer.globalPlacementMap.GetProductByPlacement(int(bucket.Placement))
		}

		// Get remainder for this product.
		remainder := int64(0)
		if product, ok := observer.productPrices[productID]; ok {
			remainder = product.StorageRemainderBytes
		}

		bucketToRemainder[bucket.Location] = remainder
		uniqueRemainders[remainder] = true
	}

	// Convert unique remainders map to slice.
	var remainders []int64
	for remainder := range uniqueRemainders {
		remainders = append(remainders, remainder)
	}

	// Call CollectBucketTallies once with all remainder values.
	// This calculates all remainder variants in a single query.
	tallies, err := observer.metabase.CollectBucketTallies(ctx, metabase.CollectBucketTallies{
		From:               fromBucket,
		To:                 toBucket,
		AsOfSystemTime:     startTime,
		AsOfSystemInterval: observer.config.AsOfSystemInterval,
		Now:                observer.Now,
		UsePartitionQuery:  observer.config.UsePartitionQuery,
		StorageRemainders:  remainders,
	})
	if err != nil {
		return err
	}

	// Apply the correct remainder value to each bucket.
	for _, tally := range tallies {
		remainder, ok := bucketToRemainder[tally.BucketLocation]
		if !ok {
			// Bucket not in our entitlements list, skip it.
			// This should not happen.
			continue
		}

		bucket := observer.ensureBucket(tally.BucketLocation)
		bucket.TotalSegments = tally.TotalSegments
		bucket.MetadataSize = tally.MetadataSize
		bucket.ObjectCount = tally.ObjectCount
		bucket.PendingObjectCount = tally.PendingObjectCount

		// Get the bytes value for this bucket's remainder from the map
		bucket.TotalBytes = tally.BytesByRemainder[remainder]
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
