// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package paymentsconfig

import (
	"storj.io/common/memory"
	"storj.io/storj/satellite/payments/stripecoinpayments"
)

// Config defines global payments config.
type Config struct {
	Provider           string `help:"payments provider to use" default:""`
	StripeCoinPayments stripecoinpayments.Config
	StorageTBPrice     string      `help:"price user should pay for storing TB per month" default:"10"`
	EgressTBPrice      string      `help:"price user should pay for each TB of egress" default:"45"`
	ObjectPrice        string      `help:"price user should pay for each object stored in network per month" default:"0.0000022"`
	BonusRate          int64       `help:"amount of percents that user will earn as bonus credits by depositing in STORJ tokens" default:"10"`
	CouponValue        int64       `help:"coupon value in cents" default:"5500"`
	CouponDuration     int64       `help:"duration a new coupon is valid in months/billing cycles" default:"2"`
	CouponProjectLimit memory.Size `help:"project limit to which increase to after applying the coupon" default:"1TB"`
	MinCoinPayment     int64       `help:"minimum value of coin payments in cents before coupon is applied" default:"5000"`
}
