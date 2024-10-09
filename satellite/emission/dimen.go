// Copyright (C) 2024 Storj Labs, Inc.
// See LICENSE for copying information.

package emission

import (
	"bytes"
	"fmt"

	"github.com/zeebo/errs"
)

// Unit represents a set of unit dimensions.
// Unit{byte: 0, watt: 0, hour: 1, kilogram: 0} means hour (H).
// Unit{byte: 1, watt: 0, hour: -1, kilogram: 0} means byte/hour (B/H).
// Unit{byte: -1, watt: 0, hour: -1, kilogram: 1} means kg/byte-hour (kg/B*H).
type Unit struct {
	byte     int8
	watt     int8
	hour     int8
	kilogram int8
}

// Value creates new Val from existing Unit.
func (u *Unit) Value(v float64) Val {
	return Val{Value: v, Unit: *u}
}

// Mul multiplies existing Unit by a given one.
func (u *Unit) Mul(b Unit) {
	u.byte += b.byte
	u.watt += b.watt
	u.hour += b.hour
	u.kilogram += b.kilogram
}

// Div divides existing Unit by a given one.
func (u *Unit) Div(b Unit) {
	u.byte -= b.byte
	u.watt -= b.watt
	u.hour -= b.hour
	u.kilogram -= b.kilogram
}

// String returns string representation of the Unit.
func (u *Unit) String() string {
	var num bytes.Buffer
	var div bytes.Buffer

	a := func(prefix string, v int8) {
		if v == 0 {
			return
		}

		target := &num
		if v < 0 {
			target = &div
			v = -v
		}

		switch v {
		case 1:
			target.WriteString(prefix)
		case 2:
			target.WriteString(prefix + "²")
		case 3:
			target.WriteString(prefix + "³")
		default:
			target.WriteString(fmt.Sprintf("%s^%d", prefix, v))
		}
	}

	a("B", u.byte)
	a("W", u.watt)
	a("H", u.hour)
	a("kg", u.kilogram)

	n := num.String()
	d := div.String()

	switch {
	case n == "" && d == "":
		return ""
	case d == "":
		return n
	case n == "":
		return "1/" + d
	default:
		return n + "/" + d
	}
}

// Val represents a value which consists of the numeric value itself and it's dimensions e.g. 1 W.
// It may be used to represent a really complex value e.g. 1 W / H or 0.005 W * H / B.
type Val struct {
	Value float64
	Unit  Unit
}

// Add sums two Val instances with the same dimensions.
func (a Val) Add(b Val) (Val, error) {
	if a.Unit != b.Unit {
		return Val{}, errs.New("cannot add units %v, %v", a.Unit, b.Unit)
	}
	r := a
	r.Value += b.Value
	return r, nil
}

// Sub subtracts one Val from another if they have the same dimensions.
func (a Val) Sub(b Val) (Val, error) {
	if a.Unit != b.Unit {
		return Val{}, errs.New("cannot subtract units %v, %v", a.Unit, b.Unit)
	}
	r := a
	r.Value -= b.Value
	return r, nil
}

// Mul multiplies existing Val with a given one and returns new Val.
// It adjusts both the amount and the dimensions accordingly.
func (a Val) Mul(b Val) Val {
	r := a
	r.Unit.Mul(b.Unit)
	r.Value *= b.Value
	return r
}

// Div divides one Val by a given one and returns new Val.
// It adjusts both the amount and the dimensions accordingly.
func (a Val) Div(b Val) Val {
	r := a
	r.Unit.Div(b.Unit)
	r.Value /= b.Value
	return r
}

// String returns string representation of the Val.
func (a Val) String() string {
	return fmt.Sprintf("%f[%s]", a.Value, a.Unit.String())
}
