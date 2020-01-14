// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package bandwidth

import (
	"context"
	"time"

	"storj.io/common/pb"
	"storj.io/common/storj"
)

// DB contains information about bandwidth usage.
//
// architecture: Database
type DB interface {
	Add(ctx context.Context, satelliteID storj.NodeID, action pb.PieceAction, amount int64, created time.Time) error
	// MonthSummary returns summary of the current months bandwidth usages.
	MonthSummary(ctx context.Context) (int64, error)
	Rollup(ctx context.Context) (err error)
	// Summary returns summary of bandwidth usages.
	Summary(ctx context.Context, from, to time.Time) (*Usage, error)
	// EgressSummary returns summary of egress bandwidth usages.
	EgressSummary(ctx context.Context, from, to time.Time) (*Usage, error)
	// IngressSummary returns summary of ingress bandwidth usages.
	IngressSummary(ctx context.Context, from, to time.Time) (*Usage, error)
	// SatelliteSummary returns aggregated bandwidth usage for a particular satellite.
	SatelliteSummary(ctx context.Context, satelliteID storj.NodeID, from, to time.Time) (*Usage, error)
	// SatelliteEgressSummary returns egress bandwidth usage for a particular satellite.
	SatelliteEgressSummary(ctx context.Context, satelliteID storj.NodeID, from, to time.Time) (*Usage, error)
	// SatelliteIngressSummary returns ingress bandwidth usage for a particular satellite.
	SatelliteIngressSummary(ctx context.Context, satelliteID storj.NodeID, from, to time.Time) (*Usage, error)
	SummaryBySatellite(ctx context.Context, from, to time.Time) (map[storj.NodeID]*Usage, error)
	// GetDailyRollups returns slice of daily bandwidth usage rollups for provided time range,
	// sorted in ascending order.
	GetDailyRollups(ctx context.Context, from, to time.Time) ([]UsageRollup, error)
	// GetDailySatelliteRollups returns slice of daily bandwidth usage for provided time range,
	// sorted in ascending order for a particular satellite.
	GetDailySatelliteRollups(ctx context.Context, satelliteID storj.NodeID, from, to time.Time) ([]UsageRollup, error)
}

// Usage contains bandwidth usage information based on the type
type Usage struct {
	Invalid int64
	Unknown int64

	Put       int64
	Get       int64
	GetAudit  int64
	GetRepair int64
	PutRepair int64
	Delete    int64
}

// Egress stores info about storage node egress usage.
type Egress struct {
	Repair int64 `json:"repair"`
	Audit  int64 `json:"audit"`
	Usage  int64 `json:"usage"`
}

// Ingress stores info about storage node ingress usage.
type Ingress struct {
	Repair int64 `json:"repair"`
	Usage  int64 `json:"usage"`
}

// UsageRollup contains rolluped bandwidth usage.
type UsageRollup struct {
	Egress        Egress    `json:"egress"`
	Ingress       Ingress   `json:"ingress"`
	Delete        int64     `json:"delete"`
	IntervalStart time.Time `json:"intervalStart"`
}

// Include adds specified action to the appropriate field.
func (usage *Usage) Include(action pb.PieceAction, amount int64) {
	switch action {
	case pb.PieceAction_INVALID:
		usage.Invalid += amount
	case pb.PieceAction_PUT:
		usage.Put += amount
	case pb.PieceAction_GET:
		usage.Get += amount
	case pb.PieceAction_GET_AUDIT:
		usage.GetAudit += amount
	case pb.PieceAction_GET_REPAIR:
		usage.GetRepair += amount
	case pb.PieceAction_PUT_REPAIR:
		usage.PutRepair += amount
	case pb.PieceAction_DELETE:
		usage.Delete += amount
	default:
		usage.Unknown += amount
	}
}

// Add adds another usage to this one.
func (usage *Usage) Add(b *Usage) {
	usage.Invalid += b.Invalid
	usage.Unknown += b.Unknown
	usage.Put += b.Put
	usage.Get += b.Get
	usage.GetAudit += b.GetAudit
	usage.GetRepair += b.GetRepair
	usage.PutRepair += b.PutRepair
	usage.Delete += b.Delete
}

// Total sums all type of bandwidths
func (usage *Usage) Total() int64 {
	return usage.Invalid +
		usage.Unknown +
		usage.Put +
		usage.Get +
		usage.GetAudit +
		usage.GetRepair +
		usage.PutRepair +
		usage.Delete
}

// TotalMonthlySummary returns total bandwidth usage for current month
func TotalMonthlySummary(ctx context.Context, db DB) (*Usage, error) {
	return db.Summary(ctx, getBeginningOfMonth(), time.Now())
}

func getBeginningOfMonth() time.Time {
	t := time.Now()
	y, m, _ := t.Date()
	return time.Date(y, m, 1, 0, 0, 0, 0, time.Now().Location())
}
