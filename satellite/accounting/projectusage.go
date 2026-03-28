// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package accounting

import (
	"context"
	"time"

	"github.com/spacemonkeygo/monkit/v3"
	"github.com/zeebo/errs"
	"go.uber.org/zap"

	"storj.io/common/memory"
	"storj.io/common/uuid"
	"storj.io/eventkit"
	"storj.io/storj/satellite/metabase"
)

const noLimits = -1

var (
	mon = monkit.Package()
	ek  = eventkit.Package()
)

// ErrProjectUsage general error for project usage.
var ErrProjectUsage = errs.Class("project usage")

// ErrProjectLimitExceeded is used when the configured limits of a project are reached.
var ErrProjectLimitExceeded = errs.Class("project limit")

// Service is handling project usage related logic.
//
// architecture: Service
type Service struct {
	log                 *zap.Logger
	projectAccountingDB ProjectAccounting
	liveAccounting      Cache
	metabaseDB          metabase.DB
	bandwidthCacheTTL   time.Duration
	nowFn               func() time.Time

	defaultMaxStorage   memory.Size
	defaultMaxBandwidth memory.Size
	defaultMaxSegments  int64
	asOfSystemInterval  time.Duration
}

// NewService created new instance of project usage service.
func NewService(log *zap.Logger, projectAccountingDB ProjectAccounting, liveAccounting Cache, metabaseDB metabase.DB, bandwidthCacheTTL time.Duration,
	defaultMaxStorage, defaultMaxBandwidth memory.Size, defaultMaxSegments int64, asOfSystemInterval time.Duration) *Service {
	return &Service{
		log:                 log,
		projectAccountingDB: projectAccountingDB,
		liveAccounting:      liveAccounting,
		metabaseDB:          metabaseDB,
		bandwidthCacheTTL:   bandwidthCacheTTL,

		defaultMaxStorage:   defaultMaxStorage,
		defaultMaxBandwidth: defaultMaxBandwidth,
		defaultMaxSegments:  defaultMaxSegments,

		asOfSystemInterval: asOfSystemInterval,
		nowFn:              time.Now,
	}
}

// BandwidthLimit contains the results of checking bandwidth limits.
type BandwidthLimit struct {
	Exceeds bool
	Limit   memory.Size

	// BandwidthThresholds and BandwidthResets are populated only when checkThresholds is true.
	BandwidthThresholds []ProjectUsageThreshold
	BandwidthResets     []ProjectUsageThreshold
}

// ExceedsBandwidthUsage returns whether the bandwidth usage limits have been exceeded
// for a project in the past month (30 days). The usage limit is (e.g 25GB) multiplied by the redundancy
// expansion factor, so that the uplinks have a raw limit.
// When checkThresholds is true, also detects bandwidth threshold crossings for notification events.
//
// Among others,it can return one of the following errors returned by
// storj.io/storj/satellite/accounting.Cache except the ErrKeyNotFound, wrapped
// by ErrProjectUsage.
func (usage *Service) ExceedsBandwidthUsage(ctx context.Context, limits ProjectLimits, checkThresholds bool) (bwLimit BandwidthLimit, err error) {
	defer mon.Task()(&ctx)(&err)

	if unlimitedDownloads(limits.Bandwidth) {
		return BandwidthLimit{}, nil
	}

	bwLimit.Limit = usage.defaultMaxBandwidth
	if limits.Bandwidth != nil {
		bwLimit.Limit = memory.Size(*limits.Bandwidth)
	}

	// Get the current bandwidth usage from cache.
	bandwidthUsage, err := usage.liveAccounting.GetProjectBandwidthUsage(ctx, limits.ProjectID, usage.nowFn())
	if err != nil {
		// Verify if the cache key was not found.
		if ErrKeyNotFound.Has(err) {
			// Get current bandwidth value from database.
			now := usage.nowFn()
			bandwidthUsage, err = usage.GetProjectBandwidth(ctx, limits.ProjectID, now.Year(), now.Month(), now.Day())
			if err != nil {
				return BandwidthLimit{}, ErrProjectUsage.Wrap(err)
			}

			// Create cache key with database value.
			_, err = usage.liveAccounting.InsertProjectBandwidthUsage(ctx, limits.ProjectID, bandwidthUsage, usage.bandwidthCacheTTL, usage.nowFn())
			if err != nil {
				return BandwidthLimit{}, ErrProjectUsage.Wrap(err)
			}
		}
	}

	bwLimit.Exceeds = bandwidthUsage >= bwLimit.Limit.Int64()

	if checkThresholds {
		flags, err := usage.liveAccounting.GetProjectNotificationFlags(ctx, limits.ProjectID)
		if err != nil {
			if ErrKeyNotFound.Has(err) {
				flags = limits.NotificationFlags
			} else {
				usage.log.Error("error while getting project notification flags", zap.Error(err))
			}
		}
		bwLimit.BandwidthThresholds, bwLimit.BandwidthResets = detectBandwidthThresholds(bandwidthUsage, bwLimit.Limit.Int64(), flags)
	}

	return bwLimit, nil
}

// UploadLimit contains upload limit characteristics.
type UploadLimit struct {
	ExceedsStorage  bool
	StorageLimit    memory.Size
	ExceedsSegments bool
	SegmentsLimit   int64
}

// ExceedsUploadLimits returns combined checks for storage and segment limits.
// Supply nonzero headroom parameters to check if there is room for a new object.
func (usage *Service) ExceedsUploadLimits(
	ctx context.Context, storageSizeHeadroom int64, segmentCountHeadroom int64, limits ProjectLimits) (limit UploadLimit) {
	defer mon.Task()(&ctx)(nil)

	// Check for unlimited uploads before setting limits
	if unlimitedUploads(limits.Usage, limits.Segments) {
		limit.ExceedsSegments = false
		limit.ExceedsStorage = false
		return limit
	}

	limit.SegmentsLimit = usage.defaultMaxSegments
	if limits.Segments != nil {
		limit.SegmentsLimit = *limits.Segments
	}

	limit.StorageLimit = usage.defaultMaxStorage
	if limits.Usage != nil {
		limit.StorageLimit = memory.Size(*limits.Usage)
	}

	storageUsage, segmentUsage, err := usage.GetProjectStorageAndSegmentUsage(ctx, limits.ProjectID)
	if err != nil {
		usage.log.Error("error while getting storage/segments usage", zap.Error(err))
	}

	limit.ExceedsSegments = (segmentUsage + segmentCountHeadroom) > limit.SegmentsLimit
	limit.ExceedsStorage = (storageUsage + storageSizeHeadroom) > limit.StorageLimit.Int64()

	return limit
}

type limitThresholdEntry struct {
	pct int
	bit ProjectUsageThreshold
}

// detectBandwidthThresholds determines which egress threshold events should be emitted.
// current is the accumulated bandwidth usage for the current month in bytes.
// bandwidthLimit is the project's bandwidth limit in bytes.
// flags is the project's NotificationFlags bitmask: EgressNotificationsEnabled must be set
// for any events to fire; EgressUsage80/EgressUsage100 bits track whether the corresponding
// threshold email has already been sent (preventing duplicate emails).
func detectBandwidthThresholds(current, bandwidthLimit int64, flags int) (thresholds, resets []ProjectUsageThreshold) {
	if flags&int(EgressNotificationsEnabled) == 0 {
		return nil, nil
	}

	// Process in descending order so we emit only the highest newly-exceeded threshold.
	for _, e := range []limitThresholdEntry{
		{100, EgressUsage100},
		{80, EgressUsage80},
	} {
		value := bandwidthLimit * int64(e.pct) / 100
		flagSet := flags&int(e.bit) != 0
		if current >= value && !flagSet {
			thresholds = append(thresholds, e.bit)
			break // skip lower thresholds — only the highest newly-exceeded one gets emitted.
		}
		if current < value && flagSet {
			resets = append(resets, e.bit)
		}
	}
	return thresholds, resets
}

// AddProjectUsageUpToLimit increases segment and storage usage up to the projects limit.
// If the limit is exceeded, neither usage is increased and accounting.ErrProjectLimitExceeded is returned.
func (usage *Service) AddProjectUsageUpToLimit(ctx context.Context, projectID uuid.UUID, storage int64, segments int64, limits ProjectLimits) (err error) {
	defer mon.Task()(&ctx, projectID)(&err)

	segmentsLimit := usage.defaultMaxSegments
	if limits.Segments != nil {
		segmentsLimit = *limits.Segments
	}

	storageLimit := usage.defaultMaxStorage
	if limits.Usage != nil {
		storageLimit = memory.Size(*limits.Usage)
	}

	err = usage.liveAccounting.AddProjectStorageUsageUpToLimit(ctx, projectID, storage, storageLimit.Int64())
	if err != nil {
		return err
	}

	err = usage.liveAccounting.AddProjectSegmentUsageUpToLimit(ctx, projectID, segments, segmentsLimit)
	if ErrProjectLimitExceeded.Has(err) {
		// roll back storage increase
		err = usage.liveAccounting.UpdateProjectStorageAndSegmentUsage(ctx, projectID, -1*storage, 0)
		if err != nil {
			return err
		}
	}

	return err
}

// GetProjectStorageTotals returns total amount of storage used by project.
//
// It can return one of the following errors returned by
// storj.io/storj/satellite/accounting.Cache.GetProjectStorageUsage except the
// ErrKeyNotFound, wrapped by ErrProjectUsage.
func (usage *Service) GetProjectStorageTotals(ctx context.Context, projectID uuid.UUID) (total int64, err error) {
	defer mon.Task()(&ctx, projectID)(&err)

	total, err = usage.liveAccounting.GetProjectStorageUsage(ctx, projectID)
	if ErrKeyNotFound.Has(err) {
		return 0, nil
	}

	return total, ErrProjectUsage.Wrap(err)
}

// GetProjectBandwidthTotals returns total amount of allocated bandwidth used for past 30 days.
func (usage *Service) GetProjectBandwidthTotals(ctx context.Context, projectID uuid.UUID) (_ int64, err error) {
	defer mon.Task()(&ctx, projectID)(&err)

	// from the beginning of the current month
	year, month, _ := usage.nowFn().Date()

	total, err := usage.projectAccountingDB.GetProjectBandwidth(ctx, projectID, year, month, 1, usage.asOfSystemInterval)
	return total, ErrProjectUsage.Wrap(err)
}

// GetProjectBandwidth returns project allocated bandwidth for the specified year, month and day.
func (usage *Service) GetProjectBandwidth(ctx context.Context, projectID uuid.UUID, year int, month time.Month, day int) (_ int64, err error) {
	defer mon.Task()(&ctx, projectID)(&err)

	total, err := usage.projectAccountingDB.GetProjectBandwidth(ctx, projectID, year, month, day, usage.asOfSystemInterval)
	return total, ErrProjectUsage.Wrap(err)
}

// GetProjectStorageLimit returns current project storage limit.
func (usage *Service) GetProjectStorageLimit(ctx context.Context, projectID uuid.UUID) (_ memory.Size, err error) {
	defer mon.Task()(&ctx, projectID)(&err)
	storageLimit, err := usage.projectAccountingDB.GetProjectStorageLimit(ctx, projectID)
	if err != nil {
		return 0, ErrProjectUsage.Wrap(err)
	}

	if storageLimit == nil {
		return usage.defaultMaxStorage, nil
	}

	return memory.Size(*storageLimit), nil
}

// GetProjectBandwidthLimit returns current project bandwidth limit.
func (usage *Service) GetProjectBandwidthLimit(ctx context.Context, projectID uuid.UUID) (_ memory.Size, err error) {
	defer mon.Task()(&ctx, projectID)(&err)
	bandwidthLimit, err := usage.projectAccountingDB.GetProjectBandwidthLimit(ctx, projectID)
	if err != nil {
		return 0, ErrProjectUsage.Wrap(err)
	}

	if bandwidthLimit == nil {
		return usage.defaultMaxBandwidth, nil
	}

	return memory.Size(*bandwidthLimit), nil
}

// GetProjectSegmentLimit returns current project segment limit.
func (usage *Service) GetProjectSegmentLimit(ctx context.Context, projectID uuid.UUID) (_ memory.Size, err error) {
	defer mon.Task()(&ctx, projectID)(&err)
	segmentLimit, err := usage.projectAccountingDB.GetProjectSegmentLimit(ctx, projectID)
	if err != nil {
		return 0, ErrProjectUsage.Wrap(err)
	}

	if segmentLimit == nil {
		return memory.Size(usage.defaultMaxSegments), nil
	}

	return memory.Size(*segmentLimit), nil
}

// GetProjectLimits returns all project limits including user specified usage and bandwidth limits.
func (usage *Service) GetProjectLimits(ctx context.Context, projectID uuid.UUID) (_ *ProjectLimits, err error) {
	defer mon.Task()(&ctx, projectID)(&err)
	limits, err := usage.projectAccountingDB.GetProjectLimits(ctx, projectID)
	if err != nil {
		return nil, ErrProjectUsage.Wrap(err)
	}

	if limits.Segments == nil {
		limits.Segments = &usage.defaultMaxSegments
	}
	if limits.Bandwidth == nil {
		bandwidth := usage.defaultMaxBandwidth.Int64()
		limits.Bandwidth = &bandwidth
	}
	if limits.Usage == nil {
		storage := usage.defaultMaxStorage.Int64()
		limits.Usage = &storage
	}

	return &limits, nil
}

// GetProjectBandwidthUsage get the current bandwidth usage from cache.
//
// It can return one of the following errors returned by
// storj.io/storj/satellite/accounting.Cache.GetProjectBandwidthUsage, wrapped
// by ErrProjectUsage.
func (usage *Service) GetProjectBandwidthUsage(ctx context.Context, projectID uuid.UUID) (currentUsed int64, err error) {
	return usage.liveAccounting.GetProjectBandwidthUsage(ctx, projectID, usage.nowFn())
}

// UpdateProjectBandwidthUsage increments the bandwidth cache key for a specific project.
//
// It can return one of the following errors returned by
// storj.io/storj/satellite/accounting.Cache.UpdateProjectBandwidthUsage, wrapped
// by ErrProjectUsage.
func (usage *Service) UpdateProjectBandwidthUsage(ctx context.Context, limits ProjectLimits, increment int64) (err error) {
	if unlimitedDownloads(limits.Bandwidth) {
		return nil
	}
	return usage.liveAccounting.UpdateProjectBandwidthUsage(ctx, limits.ProjectID, increment, usage.bandwidthCacheTTL, usage.nowFn())
}

// GetProjectStorageAndSegmentUsage get the current storage and segment usage from cache.
//
// It can return one of the following errors returned by
// storj.io/storj/satellite/accounting.Cache.GetProjectStorageAndSegmentUsage.
func (usage *Service) GetProjectStorageAndSegmentUsage(ctx context.Context, projectID uuid.UUID) (storage, segments int64, err error) {
	return usage.liveAccounting.GetProjectStorageAndSegmentUsage(ctx, projectID)
}

// UpdateProjectStorageAndSegmentUsage increments the storage and segment cache keys for a specific project.
func (usage *Service) UpdateProjectStorageAndSegmentUsage(ctx context.Context, limits ProjectLimits, storageIncr, segmentIncr int64) (err error) {
	defer mon.Task()(&ctx, limits.ProjectID)(&err)
	if unlimitedUploads(limits.Usage, limits.Segments) {
		return nil
	}
	return usage.liveAccounting.UpdateProjectStorageAndSegmentUsage(ctx, limits.ProjectID, storageIncr, segmentIncr)
}

// UpdateProjectNotificationFlags sets the notification flags for the project in the live accounting cache.
func (usage *Service) UpdateProjectNotificationFlags(ctx context.Context, projectID uuid.UUID, flags int) (err error) {
	defer mon.Task()(&ctx, projectID)(&err)
	return usage.liveAccounting.UpdateProjectNotificationFlags(ctx, projectID, flags)
}

// DetectStorageThresholds returns any storage threshold or reset events triggered by committing
// an object of the given encrypted size. It must be called after the committed size has already
// been added to the live accounting cache, so that storageUsage reflects the post-commit total.
// committedSize is subtracted to derive the pre-commit usage for crossing-detection.
func (usage *Service) DetectStorageThresholds(ctx context.Context, projectID uuid.UUID, committedSize int64, limits ProjectLimits) (thresholds, resets []ProjectUsageThreshold) {
	defer mon.Task()(&ctx, projectID)(nil)

	flags, err := usage.liveAccounting.GetProjectNotificationFlags(ctx, projectID)
	if err != nil {
		if ErrKeyNotFound.Has(err) {
			flags = limits.NotificationFlags
		} else {
			usage.log.Error("error while getting project notification flags for threshold detection", zap.Error(err))
			return nil, nil
		}
	}

	if flags&int(StorageNotificationsEnabled) == 0 {
		return nil, nil
	}

	storageUsage, _, err := usage.liveAccounting.GetProjectStorageAndSegmentUsage(ctx, projectID)
	if err != nil {
		usage.log.Error("error while getting storage usage for threshold detection", zap.Error(err))
		return nil, nil
	}

	storageLimit := usage.defaultMaxStorage.Int64()
	if limits.Usage != nil {
		storageLimit = *limits.Usage
	}

	return detectStorageThresholds(storageUsage-committedSize, storageUsage, storageLimit, flags)
}

// detectStorageThresholds determines which storage threshold events should be emitted.
// before and after are the pre- and post-commit storage usage in bytes.
// storageLimit is the project's storage limit in bytes.
// flags is the project's NotificationFlags bitmask: StorageNotificationsEnabled must be set
// for any events to fire; StorageUsage80/StorageUsage100 bits track whether the corresponding
// threshold email has already been sent (preventing duplicate emails).
func detectStorageThresholds(before, after, storageLimit int64, flags int) (thresholds, resets []ProjectUsageThreshold) {
	// Process in descending order so we emit only the highest newly-crossed threshold.
	for _, e := range []limitThresholdEntry{
		{100, StorageUsage100},
		{80, StorageUsage80},
	} {
		value := storageLimit * int64(e.pct) / 100
		flagSet := flags&int(e.bit) != 0
		if before < value && after >= value && !flagSet {
			thresholds = append(thresholds, e.bit)
			break // skip lower thresholds — only the highest newly-crossed one gets emitted.
		}
		if after < value && flagSet {
			resets = append(resets, e.bit)
		}
	}
	return thresholds, resets
}

// SetNow allows tests to have the Service act as if the current time is whatever they want.
func (usage *Service) SetNow(now func() time.Time) {
	usage.nowFn = now
}

// TestSetAsOfSystemInterval allows tests to set Service asOfSystemInterval value.
func (usage *Service) TestSetAsOfSystemInterval(asOfSystemInterval time.Duration) {
	usage.asOfSystemInterval = asOfSystemInterval
}

// unlimitedThreshold this value will be used to check if the project has unlimited bw/storage/segments.
// Every limitation above this value will be considered as unlimited.
const unlimitedThreshold = 9000000000000000000 // 9EB

func unlimitedDownloads(limit *int64) bool {
	if limit == nil {
		return false
	}
	return *limit == int64(noLimits) || *limit > unlimitedThreshold
}

func unlimitedUploads(storageLimit *int64, segmentLimit *int64) bool {
	if storageLimit == nil || segmentLimit == nil {
		return false
	}
	return (*storageLimit == int64(noLimits) || *storageLimit > unlimitedThreshold) && (*segmentLimit == int64(noLimits) || *segmentLimit > unlimitedThreshold)
}
