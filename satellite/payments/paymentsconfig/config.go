// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package paymentsconfig

import (
	"strconv"

	"storj.io/common/memory"
	"storj.io/storj/satellite/payments/stripecoinpayments"
)

// Config defines global payments config.
type Config struct {
	Provider                 string `help:"payments provider to use" default:""`
	StripeCoinPayments       stripecoinpayments.Config
	StorageTBPrice           string         `help:"price user should pay for storing TB per month" default:"10"`
	EgressTBPrice            string         `help:"price user should pay for each TB of egress" default:"45"`
	ObjectPrice              string         `help:"price user should pay for each object stored in network per month" default:"0.0000022"`
	BonusRate                int64          `help:"amount of percents that user will earn as bonus credits by depositing in STORJ tokens" default:"10"`
	CouponValue              int64          `help:"coupon value in cents" default:"275"`
	CouponDuration           CouponDuration `help:"duration a new coupon is valid in months/billing cycles" default:"2"`
	CouponProjectLimit       memory.Size    `help:"project limit to which increase to after applying the coupon, 0 B means not changing it from the default" default:"0 B"`
	MinCoinPayment           int64          `help:"minimum value of coin payments in cents before coupon is applied" default:"1000"`
	NodeEgressBandwidthPrice int64          `help:"price node receive for storing TB of egress in cents" default:"2000"`
	NodeRepairBandwidthPrice int64          `help:"price node receive for storing TB of repair in cents" default:"1000"`
	NodeAuditBandwidthPrice  int64          `help:"price node receive for storing TB of audit in cents" default:"1000"`
	NodeDiskSpacePrice       int64          `help:"price node receive for storing disk space in cents/TB" default:"150"`
	PaywallProportion        float64        `help:"proportion of users which require a balance to create projects [0-1]" default:"0"`
}

// CouponDuration is a configuration struct that keeps details about default
// promotional coupon duration.
//
// Can be used as a flag.
type CouponDuration struct {
	Enabled        bool
	BillingPeriods int64
}

// Type implements pflag.Value.
func (CouponDuration) Type() string { return "paymentsconfig.CouponDuration" }

// String is required for pflag.Value.
func (cd *CouponDuration) String() string {
	if !cd.Enabled {
		return ""
	}
	return strconv.FormatInt(cd.BillingPeriods, 10)

}

// Set sets the value from a string.
func (cd *CouponDuration) Set(s string) error {
	if s == "" {
		cd.Enabled = false
		return nil
	}

	cd.Enabled = true

	billingPeriods, err := strconv.ParseInt(s, 10, 64)
	if err != nil {
		return err
	}
	cd.BillingPeriods = billingPeriods

	return nil
}

// IntPointer returns an int64 pointer representation of the config value.
func (cd *CouponDuration) IntPointer() *int64 {
	if !cd.Enabled {
		return nil
	}
	return &cd.BillingPeriods
}
