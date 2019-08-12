// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package console

import (
	"context"
	"time"

	"github.com/zeebo/errs"

	"storj.io/storj/pkg/storj"
	"storj.io/storj/storagenode/bandwidth"
)

// Bandwidth is interface for querying bandwidth from the db
type Bandwidth interface {
	// GetDaily returns slice of daily bandwidth usage for provided time range,
	// sorted in ascending order for particular satellite
	GetDaily(ctx context.Context, satelliteID storj.NodeID, from, to time.Time) ([]BandwidthUsed, error)
	// GetDailyTotal returns slice of daily bandwidth usage for provided time range,
	// sorted in ascending order
	GetDailyTotal(ctx context.Context, from, to time.Time) ([]BandwidthUsed, error)
}

// Egress stores info about storage node egress usage
type Egress struct {
	Repair int64 `json:"repair"`
	Audit  int64 `json:"audit"`
	Usage  int64 `json:"usage"`
}

// Ingress stores info about storage node ingress usage
type Ingress struct {
	Repair int64 `json:"repair"`
	Usage  int64 `json:"usage"`
}

// BandwidthInfo stores all info about storage node bandwidth usage
type BandwidthInfo struct {
	Egress    Egress  `json:"egress"`
	Ingress   Ingress `json:"ingress"`
	Used      int64   `json:"used"`
	Remaining int64   `json:"remaining"`
}

// BandwidthUsed stores bandwidth usage information
// over the period of time
type BandwidthUsed struct {
	Egress  Egress  `json:"egress"`
	Ingress Ingress `json:"ingress"`

	From, To time.Time
}

// FromUsage used to create BandwidthInfo instance from Usage object
func FromUsage(usage *bandwidth.Usage, allocatedBandwidth int64) (*BandwidthInfo, error) {
	if usage == nil {
		return nil, errs.New("usage is nil")
	}

	used := usage.Total()

	return &BandwidthInfo{
		Ingress: Ingress{
			Usage:  usage.Put,
			Repair: usage.PutRepair,
		},
		Egress: Egress{
			Repair: usage.GetRepair,
			Usage:  usage.Get,
			Audit:  usage.GetAudit,
		},
		Remaining: allocatedBandwidth - used,
		Used:      used,
	}, nil
}
