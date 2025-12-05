// Copyright (C) 2021 Storj Labs, Inc.
// See LICENSE for copying information.

package reputation

import (
	"time"

	"storj.io/common/storj"
)

// AuditWindow contains audit count for particular time frame.
type AuditWindow struct {
	WindowStart time.Time `json:"windowStart"`
	TotalCount  int32     `json:"totalCount"`
	OnlineCount int32     `json:"onlineCount"`
}

// Audit contains audit reputation metrics.
type Audit struct {
	TotalCount      int64         `json:"totalCount"`
	SuccessCount    int64         `json:"successCount"`
	Alpha           float64       `json:"alpha"`
	Beta            float64       `json:"beta"`
	UnknownAlpha    float64       `json:"unknownAlpha"`
	UnknownBeta     float64       `json:"unknownBeta"`
	Score           float64       `json:"score"`
	SuspensionScore float64       `json:"suspensionScore"`
	History         []AuditWindow `json:"history"`
}

// Stats encapsulates node reputation data.
type Stats struct {
	NodeID               storj.NodeID `json:"nodeId"`
	NodeName             string       `json:"nodeName"`
	Audit                Audit        `json:"audit"`
	OnlineScore          float64      `json:"onlineScore"`
	DisqualifiedAt       *time.Time   `json:"disqualifiedAt"`
	SuspendedAt          *time.Time   `json:"suspendedAt"`
	OfflineSuspendedAt   *time.Time   `json:"offlineSuspendedAt"`
	OfflineUnderReviewAt *time.Time   `json:"offlineUnderReviewAt"`
	VettedAt             *time.Time   `json:"vettedAt"`
	UpdatedAt            time.Time    `json:"updatedAt"`
	JoinedAt             time.Time    `json:"joinedAt"`
}
