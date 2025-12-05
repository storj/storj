// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package currency

import (
	"math"
	"strconv"

	"github.com/shopspring/decimal"
	"github.com/zeebo/errs"
)

var (
	maxInt64 = decimal.NewFromInt(math.MaxInt64)

	// Zero is a MicroUnit representing 0.
	Zero MicroUnit
)

// NewMicroUnit returns a MicroUnit with v. Much like a time.Duration, a value
// of 1 means 1e-6 or one millionth of a unit of currency.
func NewMicroUnit(v int64) MicroUnit {
	return MicroUnit{v: v}
}

// MicroUnit represents 1e-6 or one millionth of a unit of currency (e.g. one
// millionth of a dollar). It is used instead of a floating point type to
// prevent rounding errors.
type MicroUnit struct{ v int64 }

// Value returns the underlying MicroUnit value.
func (m MicroUnit) Value() int64 { return m.v }

// Decimal returns the a decimal form of the MicroUnit.
func (m MicroUnit) Decimal() decimal.Decimal {
	return decimal.New(m.v, -6)
}

// FloatString returns a string fixed to 6 decimal places.
func (m MicroUnit) FloatString() string {
	return m.Decimal().StringFixed(6)
}

// MicroUnitFromFloatString parses the string from FloatString into a MicroUnit.
func MicroUnitFromFloatString(s string) (MicroUnit, error) {
	d, err := decimal.NewFromString(s)
	if err != nil {
		return MicroUnit{}, errs.Wrap(err)
	}
	return MicroUnitFromDecimal(d)
}

// MicroUnitFromDecimal returns a MicroUnit from a decimal value and returns an
// error if there is not enough precision.
func MicroUnitFromDecimal(d decimal.Decimal) (MicroUnit, error) {
	m := d.Shift(6).Truncate(0)
	if m.GreaterThan(maxInt64) {
		return MicroUnit{}, errs.New("%s overflows micro-unit", d)
	}
	return MicroUnit{v: m.IntPart()}, nil
}

// MarshalCSV does the custom marshaling of MicroUnits.
func (m MicroUnit) MarshalCSV() (string, error) { return strconv.FormatInt(m.v, 10), nil }

// UnmarshalCSV reads the MicroUnit in CSV form.
func (m *MicroUnit) UnmarshalCSV(s string) (err error) {
	v, err := strconv.ParseInt(s, 10, 64)
	if err != nil {
		return err
	}
	m.v = v
	return nil
}
