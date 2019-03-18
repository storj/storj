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
	Add(ctx context.Context, satelliteID storj.NodeID, action pb.PieceAction, amount int64, created time.Time) error
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
