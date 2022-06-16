// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package paymentsconfig

import (
	"storj.io/storj/satellite/payments/billing"
	"storj.io/storj/satellite/payments/storjscan"
	"storj.io/storj/satellite/payments/stripecoinpayments"
)

// Config defines global payments config.
type Config struct {
	Provider                 string `help:"payments provider to use" default:""`
	BillingConfig            billing.Config
	StripeCoinPayments       stripecoinpayments.Config
	Storjscan                storjscan.Config
	StorageTBPrice           string `help:"price user should pay for storing TB per month" default:"4" testDefault:"10"`
	EgressTBPrice            string `help:"price user should pay for each TB of egress" default:"7" testDefault:"45"`
	SegmentPrice             string `help:"price user should pay for each segment stored in network per month" default:"0.0000088" testDefault:"0.0000022"`
	BonusRate                int64  `help:"amount of percents that user will earn as bonus credits by depositing in STORJ tokens" default:"10"`
	NodeEgressBandwidthPrice int64  `help:"price node receive for storing TB of egress in cents" default:"2000"`
	NodeRepairBandwidthPrice int64  `help:"price node receive for storing TB of repair in cents" default:"1000"`
	NodeAuditBandwidthPrice  int64  `help:"price node receive for storing TB of audit in cents" default:"1000"`
	NodeDiskSpacePrice       int64  `help:"price node receive for storing disk space in cents/TB" default:"150"`
}

// PricingValues holds pricing model for satellite.
type PricingValues struct {
	StorageTBPrice string
	EgressTBPrice  string
	SegmentPrice   string
}
