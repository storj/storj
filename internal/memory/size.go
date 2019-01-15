// Copyright (C) 2018 Storj Labs, Inc.
// See LICENSE for copying information.

package memory

import (
	"errors"
	"fmt"
	"strconv"
	"strings"
)

// base 2
const (
	B BinarySize = 1 << (10 * iota)
	KiB
	MiB
	GiB
	TiB
	PiB
	EiB
)

// base 10
const (
	KB DecimalSize = 1e3
	MB DecimalSize = 1e6
	GB DecimalSize = 1e9
	TB DecimalSize = 1e12
	PB DecimalSize = 1e15
	EB DecimalSize = 1e18
)

// Size implements flag.Value for collecting memory size in bytes
// type Size int64

// BinarySize implements ..
type BinarySize int64

// DecimalSize implements ..
type DecimalSize int64

// Int returns bytes binary size as int
func (binSize BinarySize) Int() int { return int(binSize) }

// Int32 returns bytes binary size as int32
func (binSize BinarySize) Int32() int32 { return int32(binSize) }

// Int64 returns bytes binary size as int64
func (binSize BinarySize) Int64() int64 { return int64(binSize) }

// Float64 returns bytes binary size as float64
func (binSize BinarySize) Float64() float64 { return float64(binSize) }

// KiB returns binary size in kibibytes
func (binSize BinarySize) KiB() float64 { return binSize.Float64() / KiB.Float64() }

// MiB returns size in mebibytes
func (binSize BinarySize) MiB() float64 { return binSize.Float64() / MB.Float64() }

// GiB returns size in gibibytes
func (binSize BinarySize) GiB() float64 { return binSize.Float64() / GB.Float64() }

// TiB returns size in tebibytes
func (binSize BinarySize) TiB() float64 { return binSize.Float64() / TB.Float64() }

// PiB returns size in pebibytes
func (binSize BinarySize) PiB() float64 { return binSize.Float64() / PB.Float64() }

// EiB returns size in exbibytes
func (binSize BinarySize) EiB() float64 { return binSize.Float64() / EB.Float64() }

// DecimalSize methods

// Int returns bytes size as int
func (decSize DecimalSize) Int() int { return int(decSize) }

// Int32 returns bytes size as int32
func (decSize DecimalSize) Int32() int32 { return int32(decSize) }

// Int64 returns bytes size as int64
func (decSize DecimalSize) Int64() int64 { return int64(decSize) }

// Float64 returns bytes size as float64
func (decSize DecimalSize) Float64() float64 { return float64(decSize) }

// KB returns size in kilobytes
func (decSize DecimalSize) KB() float64 { return decSize.Float64() / KB.Float64() }

// MB returns size in megabytes
func (decSize DecimalSize) MB() float64 { return decSize.Float64() / MB.Float64() }

// GB returns size in gigabytes
func (decSize DecimalSize) GB() float64 { return decSize.Float64() / GB.Float64() }

// TB returns size in terabytes
func (decSize DecimalSize) TB() float64 { return decSize.Float64() / TB.Float64() }

// PB returns size in petabytes
func (decSize DecimalSize) PB() float64 { return decSize.Float64() / PB.Float64() }

// EB returns size in etabytes
func (decSize DecimalSize) EB() float64 { return decSize.Float64() / EB.Float64() }

// String converts decimal size to a string
func (decSize DecimalSize) String() string {
	if decSize == 0 {
		return "0"
	}

	switch {
	case decSize >= EB*2/3:
		return fmt.Sprintf("%.1f EB", decSize.EB())
	case decSize >= PB*2/3:
		return fmt.Sprintf("%.1f PB", decSize.PB())
	case decSize >= TB*2/3:
		return fmt.Sprintf("%.1f TB", decSize.TB())
	case decSize >= GB*2/3:
		return fmt.Sprintf("%.1f GB", decSize.GB())
	case decSize >= MB*2/3:
		return fmt.Sprintf("%.1f MB", decSize.MB())
	case decSize >= KB*2/3:
		return fmt.Sprintf("%.1f KB", decSize.KB())
	}

	return strconv.Itoa(decSize.Int()) + " B"
}

func isLetter(b byte) bool {
	return ('a' <= b && b <= 'z') || ('A' <= b && b <= 'Z')
}

// Set updates value from string
func (binSize *BinarySize) Set(s string) error {
	if s == "" {
		return errors.New("empty size")
	}

	p := len(s)
	for isLetter(s[p-1]) {
		p--

		if p < 0 {
			return errors.New("p out of bounds")
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
	case "EB", "EIB":
		*binSize = BinarySize(v * EiB.Float64())
	case "PB", "PIB":
		*binSize = BinarySize(v * PiB.Float64())
	case "TB", "TIB":
		*binSize = BinarySize(v * TiB.Float64())
	case "GB", "GIB":
		*binSize = BinarySize(v * GiB.Float64())
	case "MB", "MIB":
		*binSize = BinarySize(v * MiB.Float64())
	case "KB", "KIB":
		*binSize = BinarySize(v * KiB.Float64())
	case "B", "":
		*binSize = BinarySize(v)
	default:
		return fmt.Errorf("unknown suffix %q", suffix)
	}

	return nil
}

// Set updates value from string
func (decSize *DecimalSize) Set(s string) error {
	if s == "" {
		return errors.New("empty size")
	}

	p := len(s)
	for isLetter(s[p-1]) {
		p--

		if p < 0 {
			return errors.New("p out of bounds")
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
	case "EB", "EIB":
		*decSize = DecimalSize(v * EB.Float64())
	case "PB", "PIB":
		*decSize = DecimalSize(v * PB.Float64())
	case "TB", "TIB":
		*decSize = DecimalSize(v * TB.Float64())
	case "GB", "GIB":
		*decSize = DecimalSize(v * GB.Float64())
	case "MB", "MIB":
		*decSize = DecimalSize(v * MB.Float64())
	case "KB", "KIB":
		*decSize = DecimalSize(v * KB.Float64())
	case "B", "":
		*decSize = DecimalSize(v)
	default:
		return fmt.Errorf("unknown suffix %q", suffix)
	}

	return nil
}

// Type implements pflag.Value
func (BinarySize) Type() string { return "memory.Size" }

// Type implements pflag.Value
func (DecimalSize) Type() string { return "memory.Size" }
