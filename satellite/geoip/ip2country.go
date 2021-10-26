// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information

package geoip

import "io"

// IPToCountry defines an abstraction for resolving the ISO country code given the string representation of an IP address.
type IPToCountry interface {
	io.Closer
	LookupISOCountryCode(address string) (string, error)
}
