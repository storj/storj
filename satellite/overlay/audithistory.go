// Copyright (C) 2020 Storj Labs, Inc.
// See LICENSE for copying information.

package overlay

import (
	"context"
	"time"

	"storj.io/common/pb"
	"storj.io/common/storj"
)

// AuditHistoryDB implements the database for audit history.
//
// architecture: Database
type AuditHistoryDB interface {
	// UpdateAuditHistory updates a node's audit history with an online or offline audit.
	UpdateAuditHistory(ctx context.Context, nodeID storj.NodeID, auditTime time.Time, online bool, config AuditHistoryConfig) (*UpdateAuditHistoryResponse, error)
	// GetAuditHistory gets a node's audit history.
	GetAuditHistory(ctx context.Context, nodeID storj.NodeID) (*AuditHistory, error)
}

// AuditHistory represents a node's audit history for the most recent tracking period.
type AuditHistory struct {
	Score   float64
	Windows []*AuditWindow
}

// UpdateAuditHistoryResponse contains information returned by UpdateAuditHistory.
type UpdateAuditHistoryResponse struct {
	NewScore           float64
	TrackingPeriodFull bool
}

// AuditWindow represents the number of online and total audits a node received for a specific time period.
type AuditWindow struct {
	WindowStart time.Time
	TotalCount  int32
	OnlineCount int32
}

// UpdateAuditHistory updates a node's audit history with an online or offline audit.
func (service *Service) UpdateAuditHistory(ctx context.Context, nodeID storj.NodeID, auditTime time.Time, online bool) (res *UpdateAuditHistoryResponse, err error) {
	defer mon.Task()(&ctx)(&err)

	return service.db.UpdateAuditHistory(ctx, nodeID, auditTime, online, service.config.AuditHistory)
}

// GetAuditHistory gets a node's audit history.
func (service *Service) GetAuditHistory(ctx context.Context, nodeID storj.NodeID) (auditHistory *AuditHistory, err error) {
	defer mon.Task()(&ctx)(&err)

	return service.db.GetAuditHistory(ctx, nodeID)
}

// AuditHistoryToPB converts an overlay.AuditHistory to a pb.AuditHistory.
func AuditHistoryToPB(auditHistory *AuditHistory) (historyPB *pb.AuditHistory) {
	if auditHistory == nil {
		return nil
	}
	historyPB = &pb.AuditHistory{
		Score:   auditHistory.Score,
		Windows: make([]*pb.AuditWindow, len(auditHistory.Windows)),
	}
	for i, window := range auditHistory.Windows {
		historyPB.Windows[i] = &pb.AuditWindow{
			TotalCount:  window.TotalCount,
			OnlineCount: window.OnlineCount,
			WindowStart: window.WindowStart,
		}
	}
	return historyPB
}
