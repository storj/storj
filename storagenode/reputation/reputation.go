// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package reputation

import (
	"context"
	"time"

	"storj.io/common/pb"
	"storj.io/common/storj"
)

// DB works with reputation database.
//
// architecture: Database
type DB interface {
	// Store inserts or updates reputation stats into the DB
	Store(ctx context.Context, stats Stats) error
	// Get retrieves stats for specific satellite
	Get(ctx context.Context, satelliteID storj.NodeID) (*Stats, error)
	// All retrieves all stats from DB
	All(ctx context.Context) ([]Stats, error)
	// Delete removes stats for specific satellite
	Delete(ctx context.Context, satelliteID storj.NodeID) error
}

// Stats consist of reputation metrics.
type Stats struct {
	SatelliteID storj.NodeID

	Audit       Metric
	OnlineScore float64

	DisqualifiedAt       *time.Time
	SuspendedAt          *time.Time
	OfflineSuspendedAt   *time.Time
	OfflineUnderReviewAt *time.Time
	VettedAt             *time.Time
	AuditHistory         *pb.AuditHistory

	UpdatedAt time.Time
	JoinedAt  time.Time
}

// Metric encapsulates storagenode reputation metrics.
type Metric struct {
	TotalCount   int64 `json:"totalCount"`
	SuccessCount int64 `json:"successCount"`

	Alpha        float64 `json:"alpha"`
	Beta         float64 `json:"beta"`
	UnknownAlpha float64 `json:"unknownAlpha"`
	UnknownBeta  float64 `json:"unknownBeta"`
	Score        float64 `json:"score"`
	UnknownScore float64 `json:"unknownScore"`
}

// AuditHistory encapsulates storagenode audit history.
type AuditHistory struct {
	Score   float64              `json:"score"`
	Windows []AuditHistoryWindow `json:"windows"`
}

// AuditHistoryWindow encapsulates storagenode audit history window.
type AuditHistoryWindow struct {
	WindowStart time.Time `json:"windowStart"`
	TotalCount  int32     `json:"totalCount"`
	OnlineCount int32     `json:"onlineCount"`
}

// GetAuditHistoryFromPB creates the AuditHistory json struct from a protobuf.
func GetAuditHistoryFromPB(auditHistoryPB *pb.AuditHistory) AuditHistory {
	ah := AuditHistory{}
	if auditHistoryPB == nil {
		return ah
	}
	ah.Score = auditHistoryPB.Score
	for _, window := range auditHistoryPB.Windows {
		ah.Windows = append(ah.Windows, AuditHistoryWindow{
			WindowStart: window.WindowStart,
			TotalCount:  window.TotalCount,
			OnlineCount: window.OnlineCount,
		})
	}
	return ah
}
