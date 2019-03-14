// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package bandwidth

import (
	"context"
	"time"

	"storj.io/storj/pkg/pb"
	"storj.io/storj/pkg/storj"
)

// DB contains information about bandwidth usage.
type DB interface {
	Add(ctx context.Context, satelliteID storj.NodeID, action pb.Action, amount int64, created time.Time) error
	Summary(ctx context.Context, from, to time.Time) (*Usage, error)
	SummaryBySatellite(ctx context.Context, from, to time.Time) (map[storj.NodeID]*Usage, error)
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

// Include adds specified action to the appropriate field.
func (usage *Usage) Include(action pb.Action, amount int64) {
	switch action {
	case pb.Action_INVALID:
		usage.Invalid += amount
	case pb.Action_PUT:
		usage.Put += amount
	case pb.Action_GET:
		usage.Get += amount
	case pb.Action_GET_AUDIT:
		usage.GetAudit += amount
	case pb.Action_GET_REPAIR:
		usage.GetRepair += amount
	case pb.Action_PUT_REPAIR:
		usage.PutRepair += amount
	case pb.Action_DELETE:
		usage.Delete += amount
	default:
		usage.Unknown += amount
	}
}

// TotalUsedBandwidth sums all type of bandwidth
func (usage *Usage) TotalUsedBandwidth() int64 {
	return usage.Get + usage.GetAudit + usage.GetRepair + usage.Put + usage.PutRepair +
		usage.Delete + +usage.Invalid + usage.Unknown
}
