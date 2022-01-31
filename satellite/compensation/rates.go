// Copyright (C) 2020 Storj Labs, Inc.
// See LICENSE for copying information.

package compensation

import (
	"github.com/shopspring/decimal"
	"github.com/spf13/pflag"
)

// Rates configures the payment rates for network operations.
type Rates struct {
	AtRestGBHours Rate // For data at rest in dollars per gigabyte-hour.
	GetTB         Rate // For data the node has sent for reads in dollars per terabyte.
	PutTB         Rate // For data the node has received for writes in dollars per terabyte.
	GetRepairTB   Rate // For data the node has sent for repairs in dollars per terabyte.
	PutRepairTB   Rate // For data the node has received for repairs in dollars per terabyte.
	GetAuditTB    Rate // For data the node has sent for audits in dollars per terabyte.
}

// Rate is a wrapper type around a decimal.Decimal.
type Rate decimal.Decimal

var _ pflag.Value = (*Rate)(nil)

// RateFromString parses the string form of the rate into a Rate.
func RateFromString(value string) (Rate, error) {
	r, err := decimal.NewFromString(value)
	if err != nil {
		return Rate{}, err
	}
	return Rate(r), nil
}

// String returns the string form of the Rate.
func (rate Rate) String() string {
	return decimal.Decimal(rate).String()
}

// Set updates the Rate to be equal to the parsed string.
func (rate *Rate) Set(s string) error {
	r, err := decimal.NewFromString(s)
	if err != nil {
		return err
	}
	*rate = Rate(r)
	return nil
}

// Type returns a unique string representing the type of the Rate.
func (rate Rate) Type() string {
	return "rate"
}

// RequireRateFromString parses the Rate from the string or panics.
func RequireRateFromString(s string) Rate {
	return Rate(decimal.RequireFromString(s))
}
