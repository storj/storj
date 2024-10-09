// Copyright (C) 2021 Storj Labs, Inc.
// See LICENSE for copying information.

package location

import (
	"database/sql/driver"
	"strings"

	"github.com/zeebo/errs"
)

// CountryCode stores ISO code of countries.
//
// It's encoded as a base-26. For example,
// code "QR" is encoded as `('Q'-'A')*('Z'-'A'+1) + ('R'-'A') + 1`.
// This encoding allows for smaller lookup tables for countries.
type CountryCode uint16

// ToCountryCode convert string to CountryCode.
// encoding is based on the ASCII representation of the country code.
func ToCountryCode(s string) CountryCode {
	if len(s) != 2 {
		return None
	}
	upper := strings.ToUpper(s)
	if !isUpperAsciiLetter(upper[0]) || !isUpperAsciiLetter(upper[1]) {
		return None
	}

	return CountryCode(uint16(upper[0]-'A')*asciiLetterCount + uint16(upper[1]-'A') + 1)
}

const asciiLetterCount = 'Z' - 'A' + 1
const countryCodeZZ = asciiLetterCount * asciiLetterCount
const countryCodeCount = countryCodeZZ + 1

// isUpperAsciiLetter verifies whether v is a valid character for ISO code.
func isUpperAsciiLetter(v byte) bool {
	return 'A' <= v && v <= 'Z'
}

// Equal compares two country code.
func (c CountryCode) Equal(o CountryCode) bool {
	return c == o
}

// String returns with the upper-case (two letter) ISO code of the country.
func (c CountryCode) String() string {
	if c == None {
		return ""
	}
	if int(c) < len(CountryISOCode) {
		iso := CountryISOCode[c]
		if iso != "" {
			return iso
		}
	}
	c--
	return string([]byte{byte(c/asciiLetterCount) + 'A', byte(c%asciiLetterCount) + 'A'})
}

// Value implements the driver.Valuer interface.
func (c CountryCode) Value() (driver.Value, error) {
	return c.String(), nil
}

// Scan implements the sql.Scanner interface.
func (c *CountryCode) Scan(value interface{}) error {
	if value == nil {
		*c = None
		return nil
	}

	if _, isString := value.(string); !isString {
		return errs.New("unable to scan %T into CountryCode", value)
	}

	rawValue, err := driver.String.ConvertValue(value)
	if err != nil {
		return errs.Wrap(err)
	}
	*c = ToCountryCode(rawValue.(string))
	return nil

}
