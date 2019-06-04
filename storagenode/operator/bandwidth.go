// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package operator

import "storj.io/storj/storagenode/bandwidth"

// BandwidthInfo stores all info about storage node bandwidth usage
type BandwidthInfo struct {
	Egress    Egress
	Ingress   Ingress
	Used      int64
	Remaining int64
}

// FromUsage used to create BandwidthInfo instance from Usage object
func FromUsage(usage *bandwidth.Usage, avaiableBandwidth int64) *BandwidthInfo {
	// TODO: used is not calculated
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
		Used:      0,
	}
}

// Egress stores info about storage node egress usage
type Egress struct {
	Repair int64
	Audit  int64
	Usage  int64
}

// Ingress stores info about storage node ingress usage
type Ingress struct {
	Repair int64
	Usage  int64
}
