// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package operator

import (
	"github.com/zeebo/errs"

	"storj.io/storj/storagenode/bandwidth"
)

// BandwidthInfo stores all info about storage node bandwidth usage
type BandwidthInfo struct {
	Egress    Egress  `json:"egress"`
	Ingress   Ingress `json:"ingress"`
	Used      int64   `json:"used"`
	Remaining int64   `json:"remaining"`
}

// FromUsage used to create BandwidthInfo instance from Usage object
func FromUsage(usage *bandwidth.Usage, avaiableBandwidth int64) (*BandwidthInfo, error) {
	if usage == nil {
		return nil, errs.New("usage is nil")
	}

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
		Remaining: avaiableBandwidth,
		Used:      usage.Total(),
	}, nil
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
