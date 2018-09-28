// Copyright (C) 2018 Storj Labs, Inc.
// See LICENSE for copying information.

package memory

import (
	"errors"
	"strconv"
	"strings"
)

// Size implements flag.Value for collecting memory size
type Size struct {
	Bytes int64
}

// KB returns size in kilobytes
func (size Size) KB() float64 { return float64(size.Bytes) / KB }

// MB returns size in megabytes
func (size Size) MB() float64 { return float64(size.Bytes) / MB }

// GB returns size in gigabytes
func (size Size) GB() float64 { return float64(size.Bytes) / GB }

// TB returns size in terabytes
func (size Size) TB() float64 { return float64(size.Bytes) / TB }

// String converts size to a string
func (size Size) String() string {
	if size.Bytes <= 0 {
		return "0"
	}

	v := float64(size.Bytes)
	for _, unit := range Units {
		if v >= unit.Scale {
			r := strconv.FormatFloat(v/unit.Scale, 'f', 1, 64)
			r = strings.TrimSuffix(r, "0")
			r = strings.TrimSuffix(r, ".")
			return r + unit.Suffix
		}
	}
	return strconv.Itoa(int(size.Bytes)) + "B"
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
			size.Bytes = int64(v * unit.Scale)
			return nil
		}
	}
	return errors.New("unknown suffix " + string(suffix))
}
