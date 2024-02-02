// Copyright (C) 2024 Storj Labs, Inc.
// See LICENSE for copying information.

package emission

import (
	"fmt"
	"slices"
	"sort"
	"strings"

	"github.com/zeebo/errs"
)

// Q is a Val constructor function without any dimension.
var Q = ValMaker("")

// Val represents a value which consists of the numeric value itself and it's dimensions e.g. 1 kW.
// It may be used to represent a really complex value e.g. 1 kW / H or 0.005 W * H / GB.
type Val struct {
	Amount float64
	Num    []string
	Denom  []string
}

// ValMaker creates new Val constructor function by given string representation of the unit e.g. kg.
// By providing amount value to a constructor function we create a value instance.
// kg := ValMaker("kg") - kg is a constructor function here.
// kg(1) is a 1 kilogram Val.
func ValMaker(unit string) func(val float64) *Val {
	if unit == "" {
		return func(val float64) *Val {
			return &Val{Amount: val}
		}
	}
	return func(val float64) *Val {
		return &Val{Amount: val, Num: []string{unit}}
	}
}

// Maker creates a new Val constructor function from already existing Val.
// This is used to handle dimension factor differences.
// B := ValMaker("B") - B is a constructor function here.
// B(1) returns 1 byte Val.
// KB := B(1000).Maker() returns a construction function for a KB value.
// KB(1) returns 1 kilobyte Val but under the hood it's still 1000 B value.
func (v *Val) Maker() func(val float64) *Val {
	return func(val float64) *Val {
		return v.Mul(Q(val))
	}
}

// Mul multiplies existing Val with a given one and returns new Val.
// It adjusts both the amount and the dimensions accordingly.
// Q := ValMaker("") - Q is a constructor function which has no dimension.
// Q(0.005) returns 0.005 Val.
// Q(0.005).Mul(W(1)) means 0.005 * 1 W = 0.005 W.
// Q(0.005).Mul(W(1)).Mul(H(1)) means 0.005 * 1 W * 1 H = 0.005 W * 1 H = 0.005 W * H.
func (v *Val) Mul(rhs *Val) *Val {
	rv := &Val{Amount: v.Amount * rhs.Amount}
	rv.Num = append(rv.Num, v.Num...)
	rv.Num = append(rv.Num, rhs.Num...)
	rv.Denom = append(rv.Denom, v.Denom...)
	rv.Denom = append(rv.Denom, rhs.Denom...)
	rv.simplify()
	return rv
}

// Div divides one Val by a given one and returns new Val.
// It adjusts both the amount and the dimensions accordingly.
// Q := ValMaker("") - Q is a constructor function which has no dimension.
// Q(0.005) returns 0.005 Val.
// Q(0.005).Mul(W(1)) means 0.005 * 1 W = 0.005 W.
// Q(0.005).Mul(W(1)).Div(H(1)) means 0.005 * 1 W / 1 H = 0.005 W / 1 H = 0.005 W / H.
func (v *Val) Div(rhs *Val) *Val {
	rv := &Val{Amount: v.Amount / rhs.Amount}
	rv.Num = append(rv.Num, v.Num...)
	rv.Num = append(rv.Num, rhs.Denom...)
	rv.Denom = append(rv.Denom, v.Denom...)
	rv.Denom = append(rv.Denom, rhs.Num...)
	rv.simplify()
	return rv
}

// Add sums two Val instances with the same dimensions.
func (v *Val) Add(rhs *Val) (*Val, error) {
	v.simplify()
	rhs.simplify()
	if !slices.Equal(v.Num, rhs.Num) {
		return nil, errs.New(fmt.Sprintf("cannot add units %s, %s", v, rhs))
	}
	if !slices.Equal(v.Denom, rhs.Denom) {
		return nil, errs.New(fmt.Sprintf("cannot add units %s, %s", v, rhs))
	}
	return &Val{
		Amount: v.Amount + rhs.Amount,
		Num:    slices.Clone(v.Num),
		Denom:  slices.Clone(v.Denom),
	}, nil
}

// Sub subtracts one Val from another if they have the same dimensions.
func (v *Val) Sub(rhs *Val) (*Val, error) {
	v.simplify()
	rhs.simplify()
	if !slices.Equal(v.Num, rhs.Num) {
		return nil, errs.New(fmt.Sprintf("cannot subtract units %s, %s", v, rhs))
	}
	if !slices.Equal(v.Denom, rhs.Denom) {
		return nil, errs.New(fmt.Sprintf("cannot subtract units %s, %s", v, rhs))
	}
	return &Val{
		Amount: v.Amount - rhs.Amount,
		Num:    slices.Clone(v.Num),
		Denom:  slices.Clone(v.Denom),
	}, nil
}

// InUnits converts a Val into the specified units, if possible.
func (v *Val) InUnits(units *Val) (float64, error) {
	x := v.Div(units)
	if len(x.Num) != 0 {
		return 0, errs.New(fmt.Sprintf("cannot convert %s to units %s", v, units))
	}
	if len(x.Denom) != 0 {
		return 0, errs.New(fmt.Sprintf("cannot convert %s to units %s", v, units))
	}
	return x.Amount, nil
}

// String returns string representation of the Val.
// Q := ValMaker("") - Q is a constructor function which has no dimension.
// Q(0.005).String is just 0.005.
// Q(0.005).Mul(W(1)).Mul(H(1)).Div(MB(1)).String() is 0.005 W * H / MB.
// KB(1).String() returns 1000 B because KB Val was created from a B Val.
func (v *Val) String() string {
	var b strings.Builder
	fmt.Fprintf(&b, "%v", v.Amount)
	if len(v.Num) > 0 {
		b.WriteByte(' ')
		for i, num := range v.Num {
			if i != 0 {
				b.WriteByte('*')
			}
			b.WriteString(num)
		}
	}
	if len(v.Denom) > 0 {
		b.WriteString("/")
		for i, num := range v.Denom {
			if i != 0 {
				b.WriteByte('/')
			}
			b.WriteString(num)
		}
	}
	return b.String()
}

func (v *Val) simplify() {
	counts := map[string]int{}
	for _, num := range v.Num {
		counts[num]++
	}
	for _, denom := range v.Denom {
		counts[denom]--
	}
	v.Num = v.Num[:0]
	v.Denom = v.Denom[:0]
	for name, count := range counts {
		for count > 0 {
			v.Num = append(v.Num, name)
			count--
		}
		for count < 0 {
			v.Denom = append(v.Denom, name)
			count++
		}
	}
	sort.Strings(v.Num)
	sort.Strings(v.Denom)
}
