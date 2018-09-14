// Copyright (C) 2018 Storj Labs, Inc.
// See LICENSE for copying information.

package main

import (
	"errors"
	"strconv"
	"strings"
)

// Sizes implements flag.Value for collecting byte counts
type Sizes struct {
	Default []Size
	Custom  []Size
}

// Sizes returns the loaded values
func (sizes Sizes) Sizes() []Size {
	if len(sizes.Custom) > 0 {
		return sizes.Custom
	}
	return sizes.Default
}

// String converts values to a string
func (sizes Sizes) String() string {
	sz := sizes.Sizes()
	xs := make([]string, len(sz))
	for i, size := range sz {
		xs[i] = size.String()
	}
	return strings.Join(xs, " ")
}

// Set adds values from byte values
func (sizes *Sizes) Set(s string) error {
	for _, x := range strings.Fields(s) {
		var size Size
		if err := size.Set(x); err != nil {
			return err
		}
		sizes.Custom = append(sizes.Custom, size)
	}
	return nil
}

// Size represents a value of bytes
type Size struct {
	bytes int64
}

type unit struct {
	suffix string
	scale  float64
}

var units = []unit{
	{"T", 1 << 40},
	{"G", 1 << 30},
	{"M", 1 << 20},
	{"K", 1 << 10},
	{"B", 1},
	{"", 0},
}

// String converts size to a string
func (size Size) String() string {
	if size.bytes <= 0 {
		return "0"
	}

	v := float64(size.bytes)
	for _, unit := range units {
		if v >= unit.scale {
			r := strconv.FormatFloat(v/unit.scale, 'f', 1, 64)
			r = strings.TrimSuffix(r, "0")
			r = strings.TrimSuffix(r, ".")
			return r + unit.suffix
		}
	}
	return strconv.Itoa(int(size.bytes)) + "B"
}

// Set updates value from string
func (size *Size) Set(s string) error {
	if s == "" {
		return errors.New("empty size")
	}

	value, suffix := s[:len(s)-1], s[len(s)-1]
	if '0' <= suffix && suffix <= '9' {
		suffix = 'B'
		value = s
	}

	for _, unit := range units {
		if unit.suffix == string(suffix) {
			v, err := strconv.ParseFloat(value, 64)
			if err != nil {
				return err
			}
			size.bytes = int64(v * unit.scale)
			return nil
		}
	}
	return errors.New("unknown suffix " + string(suffix))
}
