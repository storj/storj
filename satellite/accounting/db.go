// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package accounting

import (
	"context"
	"fmt"
	"time"

	"storj.io/common/memory"
	"storj.io/common/storj"
	"storj.io/common/uuid"
	"storj.io/storj/satellite/compensation"
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

// ProjectUsage consist of period total storage, egress
// and objects count per hour for certain Project in bytes.
type ProjectUsage struct {
	Storage      float64 `json:"storage"`
	Egress       int64   `json:"egress"`
	SegmentCount float64 `json:"segmentCount"`
	ObjectCount  float64 `json:"objectCount"`

	Since  time.Time `json:"since"`
	Before time.Time `json:"before"`
}

// ProjectObjectsSegments consist of period total objects and segments count for certain Project.
type ProjectObjectsSegments struct {
	SegmentCount int64 `json:"segmentCount"`
	ObjectCount  int64 `json:"objectCount"`
}

// ProjectLimits contains the project limits.
type ProjectLimits struct {
	Usage     *int64
	Bandwidth *int64
	Segments  *int64

	RateLimit  *int
	BurstLimit *int
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

	Storage      float64 `json:"storage"`
	Egress       float64 `json:"egress"`
	ObjectCount  int64   `json:"objectCount"`
	SegmentCount int64   `json:"segmentCount"`

	Since  time.Time `json:"since"`
	Before time.Time `json:"before"`
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

// ToStringSlice converts rollup values to a slice of strings.
func (b *BucketUsageRollup) ToStringSlice() []string {
	return []string{
		b.ProjectID.String(),
		b.BucketName,
		fmt.Sprintf("%f", b.TotalStoredData),
		fmt.Sprintf("%f", b.TotalSegments),
		fmt.Sprintf("%f", b.ObjectCount),
		fmt.Sprintf("%f", b.MetadataSize),
		fmt.Sprintf("%f", b.RepairEgress),
		fmt.Sprintf("%f", b.GetEgress),
		fmt.Sprintf("%f", b.AuditEgress),
		b.Since.String(),
		b.Before.String(),
	}
}

// Usage contains project's usage split on segments and storage.
type Usage struct {
	Storage  int64
	Segments int64
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
	// QueryPaymentInfo queries Nodes and Accounting_Rollup on nodeID
	QueryPaymentInfo(ctx context.Context, start time.Time, end time.Time) ([]*CSVRow, error)
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
	// CreateStorageTally creates a record for BucketStorageTally in the accounting DB table
	CreateStorageTally(ctx context.Context, tally BucketStorageTally) error
	// GetNonEmptyTallyBucketsInRange returns a list of bucket locations within the given range
	// whose most recent tally does not represent empty usage.
	GetNonEmptyTallyBucketsInRange(ctx context.Context, from, to metabase.BucketLocation) ([]metabase.BucketLocation, error)
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
	// GetProjectLimits returns current project limit for both storage and bandwidth.
	GetProjectLimits(ctx context.Context, projectID uuid.UUID) (ProjectLimits, error)
	// GetProjectTotal returns project usage summary for specified period of time.
	GetProjectTotal(ctx context.Context, projectID uuid.UUID, since, before time.Time) (*ProjectUsage, error)
	// GetProjectTotalByPartner retrieves project usage for a given period categorized by partner name.
	// Unpartnered usage or usage for a partner not present in partnerNames is mapped to the empty string.
	GetProjectTotalByPartner(ctx context.Context, projectID uuid.UUID, partnerNames []string, since, before time.Time) (usages map[string]ProjectUsage, err error)
	// GetProjectObjectsSegments returns project objects and segments number.
	GetProjectObjectsSegments(ctx context.Context, projectID uuid.UUID) (ProjectObjectsSegments, error)
	// GetBucketUsageRollups returns usage rollup per each bucket for specified period of time.
	GetBucketUsageRollups(ctx context.Context, projectID uuid.UUID, since, before time.Time) ([]BucketUsageRollup, error)
	// GetSingleBucketUsageRollup returns usage rollup per single bucket for specified period of time.
	GetSingleBucketUsageRollup(ctx context.Context, projectID uuid.UUID, bucket string, since, before time.Time) (*BucketUsageRollup, error)
	// GetBucketTotals returns per bucket total usage summary since bucket creation.
	GetBucketTotals(ctx context.Context, projectID uuid.UUID, cursor BucketUsageCursor, before time.Time) (*BucketUsagePage, error)
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
	// GetProjectSegmentUsage returns the project's segment usage.
	GetProjectSegmentUsage(ctx context.Context, projectID uuid.UUID) (currentUsed int64, err error)
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
	// UpdateProjectSegmentUsage updates the project's segment usage increasing
	// it. The projectID is inserted to the increment when it doesn't exists,
	// hence this method will never return ErrKeyNotFound error's class.
	UpdateProjectSegmentUsage(ctx context.Context, projectID uuid.UUID, increment int64) error
	// AddProjectStorageUsage adds to the projects storage usage the spacedUsed.
	// The projectID is inserted to the spaceUsed when it doesn't exists, hence
	// this method will never return ErrKeyNotFound.
	AddProjectStorageUsage(ctx context.Context, projectID uuid.UUID, spaceUsed int64) error
	// AddProjectStorageUsageUpToLimit increases storage usage up to the limit.
	// If the limit is exceeded, the usage is not increased and accounting.ErrProjectLimitExceeded is returned.
	AddProjectStorageUsageUpToLimit(ctx context.Context, projectID uuid.UUID, increment int64, spaceLimit int64) error
	// GetAllProjectTotals return the total projects' storage and segments used space.
	GetAllProjectTotals(ctx context.Context) (map[uuid.UUID]Usage, error)
	// Close the client, releasing any open resources. Once it's called any other
	// method must be called.
	Close() error
}
