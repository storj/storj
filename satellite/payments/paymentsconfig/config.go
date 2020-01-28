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
	StorageTBPrice     string `help:"price user should pay for storing TB per month" default:"10"`
	EgressTBPrice      string `help:"price user should pay for each TB of egress" default:"45"`
	ObjectPrice        string `help:"price user should pay for each object stored in network per month" default:"0.0000022"`
}
