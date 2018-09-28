// Copyright (C) 2018 Storj Labs, Inc.
// See LICENSE for copying information.

package memory

import (
	"errors"
	"strconv"
	"strings"
)

// Size implements flag.Value for collecting memory size in bytes
type Size int64

// Int returns bytes size as int
func (size Size) Int() int { return int(size) }

// Int64 returns bytes size as int64
func (size Size) Int64() int64 { return int64(size) }

// Float64 returns bytes size as float64
func (size Size) Float64() float64 { return float64(size) }

// KB returns size in kilobytes
func (size Size) KB() float64 { return size.Float64() / KB }

// MB returns size in megabytes
func (size Size) MB() float64 { return size.Float64() / MB }

// GB returns size in gigabytes
func (size Size) GB() float64 { return size.Float64() / GB }

// TB returns size in terabytes
func (size Size) TB() float64 { return size.Float64() / TB }

// String converts size to a string
func (size Size) String() string {
	if size == 0 {
		return "0"
	}

	for _, unit := range Units {
		if size >= unit.Scale {
			sizef := size.Float64() / unit.Scale.Float64()
			r := strconv.FormatFloat(sizef, 'f', 1, 64)
			r = strings.TrimSuffix(r, "0")
			r = strings.TrimSuffix(r, ".")
			return r + unit.Suffix
		}
	}

	return strconv.Itoa(size.Int()) + "B"
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

	for _, unit := range Units {
		if unit.Suffix == string(suffix) {
			v, err := strconv.ParseFloat(value, 64)
			if err != nil {
				return err
			}

			*size = Size(v * unit.Scale.Float64())
			return nil
		}
	}
	return errors.New("unknown suffix " + string(suffix))
}
