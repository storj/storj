// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package console

import (
	"context"
	"time"

	"storj.io/storj/pkg/storj"
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
	Used      float64 `json:"used"`
	Available float64 `json:"available"`
}

// BandwidthUsed stores bandwidth usage information
// over the period of time
type BandwidthUsed struct {
	Egress  Egress  `json:"egress"`
	Ingress Ingress `json:"ingress"`

	From time.Time `json:"from"`
	To   time.Time `json:"to"`
}
