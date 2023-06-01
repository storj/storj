// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package payments

import (
	"github.com/shopspring/decimal"

	"storj.io/common/uuid"
	"storj.io/storj/satellite/accounting"
)

// ProjectCharge contains project usage and how much it will cost at the end of the month.
type ProjectCharge struct {
	accounting.ProjectUsage

	// StorageMBMonthCents is how many cents we should pay for storing MB*months.
	StorageMBMonthCents int64 `json:"storagePrice"`
	// EgressMBCents is how many cents we should pay for megabytes of egress.
	EgressMBCents int64 `json:"egressPrice"`
	// SegmentMonthCents is how many cents we should pay for objects count.
	SegmentMonthCents int64 `json:"segmentPrice"`
}

// ProjectChargesResponse represents a collection of project usage charges grouped by project ID and partner name.
// It is implemented as a map of project public IDs to a nested map of partner names to ProjectCharge structs.
//
// The values of the inner map are ProjectCharge structs which contain information about the charges associated
// with a particular project-partner combination.
type ProjectChargesResponse map[uuid.UUID]map[string]ProjectCharge

// ProjectUsagePriceModel represents price model for project usage.
type ProjectUsagePriceModel struct {
	StorageMBMonthCents decimal.Decimal `json:"storageMBMonthCents"`
	EgressMBCents       decimal.Decimal `json:"egressMBCents"`
	SegmentMonthCents   decimal.Decimal `json:"segmentMonthCents"`
	EgressDiscountRatio float64         `json:"egressDiscountRatio"`
}
