// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package paymentsconfig

import "storj.io/storj/satellite/payments/stripecoinpayments"

// Config defines global payments config.
type Config struct {
	Provider           string `help:"payments provider to use" default:""`
	StripeCoinPayments stripecoinpayments.Config
}
