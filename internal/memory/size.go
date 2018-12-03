// Copyright (C) 2018 Storj Labs, Inc.
// See LICENSE for copying information.

package memory

import (
	"errors"
	"fmt"
	"strconv"
	"strings"
)

// different sizes
const (
	B Size = 1 << (10 * iota)
	KB
	MB
	GB
	TB
	PB
	EB
)

// Size implements flag.Value for collecting memory size in bytes
type Size int64

// Int returns bytes size as int
func (size Size) Int() int { return int(size) }

// Int32 returns bytes size as int32
func (size Size) Int32() int32 { return int32(size) }

// Int64 returns bytes size as int64
func (size Size) Int64() int64 { return int64(size) }

// Float64 returns bytes size as float64
func (size Size) Float64() float64 { return float64(size) }

// KB returns size in kilobytes
func (size Size) KB() float64 { return size.Float64() / KB.Float64() }

// MB returns size in megabytes
func (size Size) MB() float64 { return size.Float64() / MB.Float64() }

// GB returns size in gigabytes
func (size Size) GB() float64 { return size.Float64() / GB.Float64() }

// TB returns size in terabytes
func (size Size) TB() float64 { return size.Float64() / TB.Float64() }

// PB returns size in petabytes
func (size Size) PB() float64 { return size.Float64() / PB.Float64() }

// EB returns size in etabytes
func (size Size) EB() float64 { return size.Float64() / EB.Float64() }

// String converts size to a string
func (size Size) String() string {
	if size == 0 {
		return "0"
	}

	switch {
	case size >= EB*2/3:
		return fmt.Sprintf("%.1f EB", size.EB())
	case size >= PB*2/3:
		return fmt.Sprintf("%.1f PB", size.PB())
	case size >= TB*2/3:
		return fmt.Sprintf("%.1f TB", size.TB())
	case size >= GB*2/3:
		return fmt.Sprintf("%.1f GB", size.GB())
	case size >= MB*2/3:
		return fmt.Sprintf("%.1f MB", size.MB())
	case size >= KB*2/3:
		return fmt.Sprintf("%.1f KB", size.KB())
	}

	return strconv.Itoa(size.Int()) + " B"
}

func isLetter(b byte) bool {
	return ('a' <= b && b <= 'z') || ('A' <= b && b <= 'Z')
}

// Set updates value from string
func (size *Size) Set(s string) error {
	if s == "" {
		return errors.New("empty size")
	}

	p := len(s)
	if isLetter(s[len(s)-1]) {
		p--
		if len(s)-2 >= 0 && isLetter(s[len(s)-2]) {
			p--
		}
	}

	value, suffix := s[:p], s[p:]
	suffix = strings.ToUpper(suffix)
	if suffix == "" || suffix[len(suffix)-1] != 'B' {
		suffix += "B"
	}

	value = strings.TrimSpace(value)
	v, err := strconv.ParseFloat(value, 64)
	if err != nil {
		return err
	}

	switch suffix {
	case "EB":
		*size = Size(v * EB.Float64())
	case "PB":
		*size = Size(v * PB.Float64())
	case "TB":
		*size = Size(v * TB.Float64())
	case "GB":
		*size = Size(v * GB.Float64())
	case "MB":
		*size = Size(v * MB.Float64())
	case "KB":
		*size = Size(v * KB.Float64())
	case "B", "":
		*size = Size(v)
	default:
		return fmt.Errorf("unknown suffix %q", suffix)
	}

	return nil
}
