// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package accounting

import (
	"context"
	"time"

	"storj.io/common/memory"
	"storj.io/common/storj"
	"storj.io/common/uuid"
	"storj.io/storj/satellite/buckets"
	"storj.io/storj/satellite/compensation"
	"storj.io/storj/satellite/entitlements"
	"storj.io/storj/satellite/metabase"
	"storj.io/storj/satellite/orders"
)

// RollupStats is a convenience alias.
type RollupStats map[time.Time]map[storj.NodeID]*Rollup

// StoragenodeStorageTally mirrors dbx.StoragenodeStorageTally, allowing us to use that struct without leaking dbx.
type StoragenodeStorageTally struct {
	ID              int64
	NodeID          storj.NodeID
	IntervalEndTime time.Time
	DataTotal       float64
}

// StoragenodeBandwidthRollup mirrors dbx.StoragenodeBandwidthRollup, allowing us to use the struct without leaking dbx.
type StoragenodeBandwidthRollup struct {
	NodeID        storj.NodeID
	IntervalStart time.Time
	Action        uint
	Settled       uint64
}

// Rollup mirrors dbx.AccountingRollup, allowing us to use that struct without leaking dbx.
type Rollup struct {
	ID              int64
	NodeID          storj.NodeID
	StartTime       time.Time
	PutTotal        int64
	GetTotal        int64
	GetAuditTotal   int64
	GetRepairTotal  int64
	PutRepairTotal  int64
	AtRestTotal     float64
	IntervalEndTime time.Time
}

// BucketInfo holds information about a bucket.
type BucketInfo struct {
	Name      string
	UserAgent []byte
	Placement *storj.PlacementConstraint
}

// StorageNodePeriodUsage represents a statement for a node for a compensation period.
type StorageNodePeriodUsage struct {
	NodeID         storj.NodeID
	AtRestTotal    float64
	GetTotal       int64
	PutTotal       int64
	GetRepairTotal int64
	PutRepairTotal int64
	GetAuditTotal  int64
}

// StorageNodeUsage is node at rest space usage over a period of time.
type StorageNodeUsage struct {
	NodeID      storj.NodeID
	StorageUsed float64

	Timestamp       time.Time
	IntervalEndTime time.Time
}

// NodePaymentInfo contains data for a node payment information.
type NodePaymentInfo struct {
	NodeID storj.NodeID

	AtRestTotal    float64
	GetRepairTotal int64
	PutRepairTotal int64
	GetAuditTotal  int64
	PutTotal       int64
	GetTotal       int64
}

// ProjectUsage consist of period total storage, egress
// and objects count per hour for certain Project in bytes.
type ProjectUsage struct {
	Storage float64 `json:"storage"`
	// IncludedEgress is the amount of egress included as free in the tier/product.
	// It should be calculated if the product/tier has egress overage mode enabled.
	IncludedEgress int64   `json:"includedEgress"`
	Egress         int64   `json:"egress"`
	SegmentCount   float64 `json:"segmentCount"`
	ObjectCount    float64 `json:"objectCount"`

	Since  time.Time `json:"since"`
	Before time.Time `json:"before"`
}

// Clone creates a copy of ProjectUsage.
func (pu *ProjectUsage) Clone() (usage ProjectUsage) {
	usage.Storage = pu.Storage
	usage.IncludedEgress = pu.IncludedEgress
	usage.Egress = pu.Egress
	usage.SegmentCount = pu.SegmentCount
	usage.ObjectCount = pu.ObjectCount
	usage.Since = pu.Since
	usage.Before = pu.Before

	return usage
}

// ProjectObjectsSegments consist of period total objects and segments count for certain Project.
type ProjectObjectsSegments struct {
	SegmentCount int64 `json:"segmentCount"`
	ObjectCount  int64 `json:"objectCount"`
}

// ProjectLimits contains the project limits.
type ProjectLimits struct {
	ProjectID        uuid.UUID
	Usage            *int64
	UserSetUsage     *int64
	Bandwidth        *int64
	UserSetBandwidth *int64
	Segments         *int64

	RateLimit        *int
	BurstLimit       *int
	RateLimitHead    *int
	BurstLimitHead   *int
	RateLimitGet     *int
	BurstLimitGet    *int
	RateLimitPut     *int
	BurstLimitPut    *int
	RateLimitList    *int
	BurstLimitList   *int
	RateLimitDelete  *int
	BurstLimitDelete *int
}

// ProjectDailyUsage holds project daily usage.
type ProjectDailyUsage struct {
	StorageUsage            []ProjectUsageByDay `json:"storageUsage"`
	AllocatedBandwidthUsage []ProjectUsageByDay `json:"allocatedBandwidthUsage"`
	SettledBandwidthUsage   []ProjectUsageByDay `json:"settledBandwidthUsage"`
}

// ProjectUsageByDay holds project daily usage.
type ProjectUsageByDay struct {
	Date  time.Time `json:"date"`
	Value int64     `json:"value"`
}

// BucketUsage consist of total bucket usage for period.
type BucketUsage struct {
	ProjectID  uuid.UUID `json:"projectID"`
	BucketName string    `json:"bucketName"`
	UserAgent  []byte    `json:"-"`

	DefaultPlacement storj.PlacementConstraint `json:"defaultPlacement"`
	Location         string                    `json:"location"`

	Versioning            buckets.Versioning  `json:"versioning"`
	ObjectLockEnabled     bool                `json:"objectLockEnabled"`
	DefaultRetentionMode  storj.RetentionMode `json:"defaultRetentionMode"`
	DefaultRetentionDays  *int                `json:"defaultRetentionDays"`
	DefaultRetentionYears *int                `json:"defaultRetentionYears"`

	Storage      float64 `json:"storage"`
	Egress       float64 `json:"egress"`
	ObjectCount  int64   `json:"objectCount"`
	SegmentCount int64   `json:"segmentCount"`

	CreatorEmail string `json:"creatorEmail"`

	Since     time.Time `json:"since"`
	Before    time.Time `json:"before"`
	CreatedAt time.Time `json:"createdAt"`
}

// BucketUsageCursor holds info for bucket usage
// cursor pagination.
type BucketUsageCursor struct {
	Search string
	Limit  uint
	Page   uint
}

// BucketUsagePage represents bucket usage page result.
type BucketUsagePage struct {
	BucketUsages []BucketUsage `json:"bucketUsages"`

	Search string `json:"search"`
	Limit  uint   `json:"limit"`
	Offset uint64 `json:"offset"`

	PageCount   uint   `json:"pageCount"`
	CurrentPage uint   `json:"currentPage"`
	TotalCount  uint64 `json:"totalCount"`
}

// BucketUsageRollup is total bucket usage info
// for certain period.
type BucketUsageRollup struct {
	ProjectID  uuid.UUID `json:"projectID"`
	BucketName string    `json:"bucketName"`

	Placement storj.PlacementConstraint `json:"-"`
	UserAgent []byte                    `json:"-"`

	TotalStoredData float64 `json:"totalStoredData"`

	TotalSegments float64 `json:"totalSegments"`
	ObjectCount   float64 `json:"objectCount"`
	MetadataSize  float64 `json:"metadataSize"`

	RepairEgress float64 `json:"repairEgress"`
	GetEgress    float64 `json:"getEgress"`
	AuditEgress  float64 `json:"auditEgress"`

	Since  time.Time `json:"since"`
	Before time.Time `json:"before"`
}

// ProjectReportItem is total bucket usage info with project details for certain period.
type ProjectReportItem struct {
	ProjectID       uuid.UUID
	ProjectPublicID uuid.UUID
	ProjectName     string
	ProductName     string

	Placement storj.PlacementConstraint
	UserAgent []byte

	BucketName        string
	StorageSKU        string
	Storage           float64
	StorageTbMonth    float64
	EgressSKU         string
	Egress            float64
	EgressTb          float64
	SegmentSKU        string
	SegmentCount      float64
	SegmentCountMonth float64
	ObjectCount       float64

	// Costs in cents
	StorageCost float64
	EgressCost  float64
	SegmentCost float64
	TotalCost   float64

	Since  time.Time `json:"since"`
	Before time.Time `json:"before"`
}

// Usage contains project's usage split on segments and storage.
type Usage struct {
	Storage  int64
	Segments int64
}

// BucketLocationWithEntitlements represents a bucket location with its placement and project entitlements.
type BucketLocationWithEntitlements struct {
	Location         metabase.BucketLocation
	Placement        storj.PlacementConstraint
	ProjectFeatures  entitlements.ProjectFeatures
	HasPreviousTally bool
}

// StoragenodeAccounting stores information about bandwidth and storage usage for storage nodes.
//
// architecture: Database
type StoragenodeAccounting interface {
	// SaveTallies records tallies of data at rest
	SaveTallies(ctx context.Context, latestTally time.Time, nodes []storj.NodeID, tallies []float64) error
	// GetTallies retrieves all tallies
	GetTallies(ctx context.Context) ([]*StoragenodeStorageTally, error)
	// GetTalliesSince retrieves all tallies since latestRollup
	GetTalliesSince(ctx context.Context, latestRollup time.Time) ([]*StoragenodeStorageTally, error)
	// GetBandwidthSince retrieves all bandwidth rollup entires since latestRollup
	GetBandwidthSince(ctx context.Context, latestRollup time.Time, cb func(context.Context, *StoragenodeBandwidthRollup) error) error
	// SaveRollup records tally and bandwidth rollup aggregations to the database
	SaveRollup(ctx context.Context, latestTally time.Time, stats RollupStats) error
	// LastTimestamp records and returns the latest last tallied time.
	LastTimestamp(ctx context.Context, timestampType string) (time.Time, error)
	// QueryPaymentInfo queries accounting information and different usage.
	QueryPaymentInfo(ctx context.Context, start time.Time, end time.Time) ([]NodePaymentInfo, error)
	// QueryStorageNodePeriodUsage returns accounting statements for nodes for a given compensation period
	QueryStorageNodePeriodUsage(ctx context.Context, period compensation.Period) ([]StorageNodePeriodUsage, error)
	// QueryStorageNodeUsage returns slice of StorageNodeUsage for given period
	QueryStorageNodeUsage(ctx context.Context, nodeID storj.NodeID, start time.Time, end time.Time) ([]StorageNodeUsage, error)
	// DeleteTalliesBefore deletes all tallies prior to some time
	DeleteTalliesBefore(ctx context.Context, latestRollup time.Time, batchSize int) error
	// ArchiveRollupsBefore archives rollups older than a given time and returns num storagenode and bucket bandwidth rollups archived.
	ArchiveRollupsBefore(ctx context.Context, before time.Time, batchSize int) (numArchivedNodeBW int, err error)
	// GetRollupsSince retrieves all archived bandwidth rollup records since a given time. A hard limit batch size is used for results.
	GetRollupsSince(ctx context.Context, since time.Time) ([]StoragenodeBandwidthRollup, error)
	// GetArchivedRollupsSince retrieves all archived bandwidth rollup records since a given time. A hard limit batch size is used for results.
	GetArchivedRollupsSince(ctx context.Context, since time.Time) ([]StoragenodeBandwidthRollup, error)
}

// ProjectAccounting stores information about bandwidth and storage usage for projects.
//
// architecture: Database
type ProjectAccounting interface {
	// SaveTallies saves the latest project info
	SaveTallies(ctx context.Context, intervalStart time.Time, bucketTallies map[metabase.BucketLocation]*BucketTally) error
	// GetTallies retrieves all tallies ordered by interval start desc
	GetTallies(ctx context.Context) ([]BucketTally, error)
	// DeleteTalliesBefore deletes tallies with an interval start before the given time
	DeleteTalliesBefore(ctx context.Context, before time.Time) (int64, error)

	// CreateStorageTally creates a record for BucketStorageTally in the accounting DB table
	CreateStorageTally(ctx context.Context, tally BucketStorageTally) error

	// GetPreviouslyNonEmptyTallyBucketsInRange returns a list of bucket locations within the given range
	// whose most recent tally does not represent empty usage.
	GetPreviouslyNonEmptyTallyBucketsInRange(ctx context.Context, from, to metabase.BucketLocation, asOfSystemInterval time.Duration) ([]metabase.BucketLocation, error)
	// GetBucketsWithEntitlementsInRange returns all bucket locations within the given range along with their placement and entitlements.
	// The HasPreviousTally field indicates whether each bucket had a non-empty tally in the past.
	GetBucketsWithEntitlementsInRange(ctx context.Context, from, to metabase.BucketLocation, projectScopePrefix string) ([]BucketLocationWithEntitlements, error)
	// GetProjectSettledBandwidthTotal returns the sum of GET bandwidth usage settled for a projectID in the past time frame.
	GetProjectSettledBandwidthTotal(ctx context.Context, projectID uuid.UUID, from time.Time) (_ int64, err error)
	// GetProjectBandwidth returns project allocated bandwidth for the specified year, month and day.
	GetProjectBandwidth(ctx context.Context, projectID uuid.UUID, year int, month time.Month, day int, asOfSystemInterval time.Duration) (int64, error)
	// GetProjectSettledBandwidth returns the used settled bandwidth for the specified year and month.
	GetProjectSettledBandwidth(ctx context.Context, projectID uuid.UUID, year int, month time.Month, asOfSystemInterval time.Duration) (int64, error)
	// GetProjectDailyBandwidth returns bandwidth (allocated and settled) for the specified day.
	GetProjectDailyBandwidth(ctx context.Context, projectID uuid.UUID, year int, month time.Month, day int) (int64, int64, int64, error)
	// DeleteProjectBandwidthBefore deletes project bandwidth rollups before the given time
	DeleteProjectBandwidthBefore(ctx context.Context, before time.Time) error
	// GetProjectDailyUsageByDateRange returns daily allocated, settled bandwidth and storage usage for the specified date range.
	GetProjectDailyUsageByDateRange(ctx context.Context, projectID uuid.UUID, from, to time.Time, crdbInterval time.Duration) (*ProjectDailyUsage, error)

	// UpdateProjectUsageLimit updates project usage limit.
	UpdateProjectUsageLimit(ctx context.Context, projectID uuid.UUID, limit memory.Size) error
	// UpdateProjectBandwidthLimit updates project bandwidth limit.
	UpdateProjectBandwidthLimit(ctx context.Context, projectID uuid.UUID, limit memory.Size) error
	// UpdateProjectSegmentLimit updates project segment limit.
	UpdateProjectSegmentLimit(ctx context.Context, projectID uuid.UUID, limit int64) error
	// GetProjectStorageLimit returns project storage usage limit.
	GetProjectStorageLimit(ctx context.Context, projectID uuid.UUID) (*int64, error)
	// GetProjectBandwidthLimit returns project bandwidth usage limit.
	GetProjectBandwidthLimit(ctx context.Context, projectID uuid.UUID) (*int64, error)
	// GetProjectSegmentLimit returns the segment limit for a project ID.
	GetProjectSegmentLimit(ctx context.Context, projectID uuid.UUID) (_ *int64, err error)
	// GetProjectLimits returns all project limits including user specified usage and bandwidth limits.
	GetProjectLimits(ctx context.Context, projectID uuid.UUID) (ProjectLimits, error)
	// GetProjectTotal returns project usage summary for specified period of time.
	GetProjectTotal(ctx context.Context, projectID uuid.UUID, since, before time.Time) (*ProjectUsage, error)
	// GetProjectTotalByPartnerAndPlacement retrieves project usage for a given period categorized by partner name and placement constraint.
	// Unpartnered usage or usage for a partner not present in partnerNames is mapped to the empty string.
	GetProjectTotalByPartnerAndPlacement(ctx context.Context, projectID uuid.UUID, partnerNames []string, since, before time.Time, aggregate bool) (usages map[string]ProjectUsage, err error)
	// GetProjectObjectsSegments returns project objects and segments number.
	GetProjectObjectsSegments(ctx context.Context, projectID uuid.UUID) (ProjectObjectsSegments, error)
	// GetBucketsSinceAndBefore lists distinct bucket names for a project within a specific timeframe.
	// If withInfo is true, it also retrieves bucket information such as placement and user agent.
	// Exposed to be tested.
	GetBucketsSinceAndBefore(ctx context.Context, projectID uuid.UUID, since, before time.Time, withInfo bool) (buckets []BucketInfo, err error)
	// GetBucketUsageRollups returns usage rollup per each bucket for specified period of time.
	// If withInfo is true, it includes the placement and user agent of the bucket.
	GetBucketUsageRollups(ctx context.Context, projectID uuid.UUID, since, before time.Time, withInfo bool) ([]BucketUsageRollup, error)
	// GetSingleBucketUsageRollup returns usage rollup per single bucket for specified period of time.
	GetSingleBucketUsageRollup(ctx context.Context, projectID uuid.UUID, bucket string, since, before time.Time) (*BucketUsageRollup, error)
	// GetSingleBucketTotals returns single bucket total usage summary since bucket creation.
	GetSingleBucketTotals(ctx context.Context, projectID uuid.UUID, bucketName string, before time.Time) (usage *BucketUsage, err error)
	// GetBucketTotals returns per bucket total usage summary since bucket creation.
	GetBucketTotals(ctx context.Context, projectID uuid.UUID, cursor BucketUsageCursor, since, before time.Time) (*BucketUsagePage, error)
	// ArchiveRollupsBefore archives rollups older than a given time and returns number of bucket bandwidth rollups archived.
	ArchiveRollupsBefore(ctx context.Context, before time.Time, batchSize int) (numArchivedBucketBW int, err error)
	// GetRollupsSince retrieves all archived bandwidth rollup records since a given time. A hard limit batch size is used for results.
	GetRollupsSince(ctx context.Context, since time.Time) ([]orders.BucketBandwidthRollup, error)
	// GetArchivedRollupsSince retrieves all archived bandwidth rollup records since a given time. A hard limit batch size is used for results.
	GetArchivedRollupsSince(ctx context.Context, since time.Time) ([]orders.BucketBandwidthRollup, error)
}

// Cache stores live information about project storage which has not yet been synced to ProjectAccounting.
//
// All the implementations must follow the convention of returning errors of one
// of the classes defined in this package.
//
// All the methods return:
//
// ErrInvalidArgument: an implementation may return if some parameter contain a
// value which isn't accepted, nonetheless, not all the implementations impose
// the same constraints on them.
//
// ErrSystemOrNetError: any method will return this if there is an error with
// the underlining system or the network.
//
// ErrKeyNotFound: returned when a key is not found.
//
// ErrUnexpectedValue: returned when a key or value stored in the underlying
// system isn't of the expected format or type according the business domain.
//
// architecture: Database
type Cache interface {
	// GetProjectStorageUsage returns the project's storage usage.
	GetProjectStorageUsage(ctx context.Context, projectID uuid.UUID) (totalUsed int64, err error)
	// GetProjectBandwidthUsage returns the project's bandwidth usage.
	GetProjectBandwidthUsage(ctx context.Context, projectID uuid.UUID, now time.Time) (currentUsed int64, err error)
	// GetProjectStorageAndSegmentUsage returns the project's storage and segment usage.
	GetProjectStorageAndSegmentUsage(ctx context.Context, projectID uuid.UUID) (storage, segment int64, err error)
	// AddProjectSegmentUsageUpToLimit increases segment usage up to the limit.
	// If the limit is exceeded, the usage is not increased and accounting.ErrProjectLimitExceeded is returned.
	AddProjectSegmentUsageUpToLimit(ctx context.Context, projectID uuid.UUID, increment int64, segmentLimit int64) error
	// InsertProjectBandwidthUsage inserts a project bandwidth usage if it
	// doesn't exist. It returns true if it's inserted, otherwise false.
	InsertProjectBandwidthUsage(ctx context.Context, projectID uuid.UUID, value int64, ttl time.Duration, now time.Time) (inserted bool, _ error)
	// UpdateProjectBandwidthUsage updates the project's bandwidth usage increasing
	// it. The projectID is inserted to the increment when it doesn't exists,
	// hence this method will never return ErrKeyNotFound error's class.
	UpdateProjectBandwidthUsage(ctx context.Context, projectID uuid.UUID, increment int64, ttl time.Duration, now time.Time) error
	// AddProjectStorageUsageUpToLimit increases storage usage up to the limit.
	// If the limit is exceeded, the usage is not increased and accounting.ErrProjectLimitExceeded is returned.
	AddProjectStorageUsageUpToLimit(ctx context.Context, projectID uuid.UUID, increment int64, spaceLimit int64) error
	// UpdateProjectStorageAndSegmentUsage updates the project's storage and segment usage by increasing it.
	UpdateProjectStorageAndSegmentUsage(ctx context.Context, projectID uuid.UUID, storageIncrement, segmentIncrement int64) (err error)
	// GetAllProjectTotals return the total projects' storage and segments used space.
	GetAllProjectTotals(ctx context.Context) (map[uuid.UUID]Usage, error)
	// Close the client, releasing any open resources. Once it's called any other
	// method must be called.
	Close() error
}
