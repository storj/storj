// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package accounting

import (
	"context"
	"time"

	"storj.io/common/memory"
	"storj.io/common/storj"
	"storj.io/common/uuid"
	"storj.io/storj/satellite/compensation"
	"storj.io/storj/satellite/metainfo/metabase"
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
	ID             int64
	NodeID         storj.NodeID
	StartTime      time.Time
	PutTotal       int64
	GetTotal       int64
	GetAuditTotal  int64
	GetRepairTotal int64
	PutRepairTotal int64
	AtRestTotal    float64
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

	Timestamp time.Time
}

// ProjectUsage consist of period total storage, egress
// and objects count per hour for certain Project in bytes.
type ProjectUsage struct {
	Storage     float64 `json:"storage"`
	Egress      int64   `json:"egress"`
	ObjectCount float64 `json:"objectCount"`

	Since  time.Time `json:"since"`
	Before time.Time `json:"before"`
}

// ProjectLimits contains the storage and bandwidth limits.
type ProjectLimits struct {
	Usage     *int64
	Bandwidth *int64
}

// BucketUsage consist of total bucket usage for period.
type BucketUsage struct {
	ProjectID  uuid.UUID
	BucketName string

	Storage     float64
	Egress      float64
	ObjectCount int64

	Since  time.Time
	Before time.Time
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
	BucketUsages []BucketUsage

	Search string
	Limit  uint
	Offset uint64

	PageCount   uint
	CurrentPage uint
	TotalCount  uint64
}

// BucketUsageRollup is total bucket usage info
// for certain period.
type BucketUsageRollup struct {
	ProjectID  uuid.UUID
	BucketName []byte

	RemoteStoredData float64
	InlineStoredData float64

	RemoteSegments float64
	InlineSegments float64
	ObjectCount    float64
	MetadataSize   float64

	RepairEgress float64
	GetEgress    float64
	AuditEgress  float64

	Since  time.Time
	Before time.Time
}

// StoragenodeAccounting stores information about bandwidth and storage usage for storage nodes.
//
// architecture: Database
type StoragenodeAccounting interface {
	// SaveTallies records tallies of data at rest
	SaveTallies(ctx context.Context, latestTally time.Time, nodeData map[storj.NodeID]float64) error
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
	DeleteTalliesBefore(ctx context.Context, latestRollup time.Time) error
}

// ProjectAccounting stores information about bandwidth and storage usage for projects.
//
// architecture: Database
type ProjectAccounting interface {
	// SaveTallies saves the latest project info
	SaveTallies(ctx context.Context, intervalStart time.Time, bucketTallies map[metabase.BucketLocation]*BucketTally) error
	// GetTallies retrieves all tallies
	GetTallies(ctx context.Context) ([]BucketTally, error)
	// CreateStorageTally creates a record for BucketStorageTally in the accounting DB table
	CreateStorageTally(ctx context.Context, tally BucketStorageTally) error
	// GetAllocatedBandwidthTotal returns the sum of GET bandwidth usage allocated for a projectID in the past time frame
	GetAllocatedBandwidthTotal(ctx context.Context, projectID uuid.UUID, from time.Time) (int64, error)
	// GetProjectAllocatedBandwidth returns project allocated bandwidth for the specified year and month.
	GetProjectAllocatedBandwidth(ctx context.Context, projectID uuid.UUID, year int, month time.Month) (int64, error)
	// DeleteProjectAllocatedBandwidthBefore deletes project bandwidth rollups before the given time
	DeleteProjectAllocatedBandwidthBefore(ctx context.Context, before time.Time) error

	// GetStorageTotals returns the current inline and remote storage usage for a projectID
	GetStorageTotals(ctx context.Context, projectID uuid.UUID) (int64, int64, error)
	// UpdateProjectUsageLimit updates project usage limit.
	UpdateProjectUsageLimit(ctx context.Context, projectID uuid.UUID, limit memory.Size) error
	// UpdateProjectBandwidthLimit updates project bandwidth limit.
	UpdateProjectBandwidthLimit(ctx context.Context, projectID uuid.UUID, limit memory.Size) error
	// GetProjectStorageLimit returns project storage usage limit.
	GetProjectStorageLimit(ctx context.Context, projectID uuid.UUID) (*int64, error)
	// GetProjectBandwidthLimit returns project bandwidth usage limit.
	GetProjectBandwidthLimit(ctx context.Context, projectID uuid.UUID) (*int64, error)
	// GetProjectLimits returns current project limit for both storage and bandwidth.
	GetProjectLimits(ctx context.Context, projectID uuid.UUID) (ProjectLimits, error)
	// GetProjectTotal returns project usage summary for specified period of time.
	GetProjectTotal(ctx context.Context, projectID uuid.UUID, since, before time.Time) (*ProjectUsage, error)
	// GetBucketUsageRollups returns usage rollup per each bucket for specified period of time.
	GetBucketUsageRollups(ctx context.Context, projectID uuid.UUID, since, before time.Time) ([]BucketUsageRollup, error)
	// GetBucketTotals returns per bucket usage summary for specified period of time.
	GetBucketTotals(ctx context.Context, projectID uuid.UUID, cursor BucketUsageCursor, since, before time.Time) (*BucketUsagePage, error)
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
	// GetProjectStorageUsage  returns the project's storage usage.
	GetProjectStorageUsage(ctx context.Context, projectID uuid.UUID) (totalUsed int64, err error)
	// GetProjectBandwidthUsage  returns the project's bandwidth usage.
	GetProjectBandwidthUsage(ctx context.Context, projectID uuid.UUID, now time.Time) (currentUsed int64, err error)
	// UpdateProjectBandthUsage updates the project's bandwidth usage increasing
	// it. The projectID is inserted to the increment when it doesn't exists,
	// hence this method will never return ErrKeyNotFound error's class.
	UpdateProjectBandwidthUsage(ctx context.Context, projectID uuid.UUID, increment int64, ttl time.Duration, now time.Time) error
	// AddProjectStorageUsage adds to the projects storage usage the spacedUsed.
	// The projectID is inserted to the spaceUsed when it doesn't exists, hence
	// this method will never return ErrKeyNotFound.
	AddProjectStorageUsage(ctx context.Context, projectID uuid.UUID, spaceUsed int64) error
	// GetAllProjectTotals return the total projects' storage used space.
	GetAllProjectTotals(ctx context.Context) (map[uuid.UUID]int64, error)
	// Close the client, releasing any open resources. Once it's called any other
	// method must be called.
	Close() error
}
