// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package currency

import (
	"math"

	"github.com/shopspring/decimal"
	"github.com/zeebo/errs"
)

var (
	maxInt64 = decimal.NewFromInt(math.MaxInt64)
)

// MicroUnit represents 1e-6 or one millionth of a unit of currency (e.g. one
// millionth of a dollar). It is used instead of a floating point type to
// prevent rounding errors.
type MicroUnit int64

func (m MicroUnit) Decimal() decimal.Decimal {
	return decimal.New(int64(m), -6)
}

func (m MicroUnit) FloatString() string {
	return m.Decimal().StringFixed(6)
}

func MicroUnitFromFloatString(s string) (MicroUnit, error) {
	d, err := decimal.NewFromString(s)
	if err != nil {
		return 0, errs.Wrap(err)
	}
	return MicroUnitFromDecimal(d)
}

func MicroUnitFromDecimal(d decimal.Decimal) (MicroUnit, error) {
	m := d.Shift(6).Truncate(0)
	if m.GreaterThan(maxInt64) {
		return 0, errs.New("%s overflows micro-unit", d)
	}
	return MicroUnit(m.IntPart()), nil
}
