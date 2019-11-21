// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package paymentsconfig

import (
	"storj.io/storj/satellite/payments/stripecoinpayments"
)

// Config defines global payments config.
type Config struct {
	Provider           string `help:"payments provider to use" default:""`
	StripeCoinPayments stripecoinpayments.Config
	PerObjectPrice     int64 `help:"price in cents user should pay for each object storing in network" devDefault:"0" releaseDefault:"0"`
	EgressPrice        int64 `help:"price in cents user should pay for each TB of egress" devDefault:"0" releaseDefault:"0"`
	TbhPrice           int64 `help:"price in cents user should pay for storing each TB per hour" devDefault:"0" releaseDefault:"0"`
}
