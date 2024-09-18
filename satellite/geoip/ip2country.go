// Copyright (C) 2021 Storj Labs, Inc.
// See LICENSE for copying information

package geoip

import (
	"net"

	"github.com/zeebo/errs"

	"storj.io/storj/shared/location"
)

// IPToCountry defines an abstraction for resolving the ISO country code given the string representation of an IP address.
type IPToCountry interface {
	Close() error
	LookupISOCountryCode(address string) (location.CountryCode, error)
}

func addressToIP(address string) (net.IP, error) {
	host, _, err := net.SplitHostPort(address)
	if err != nil {
		return nil, errs.Wrap(err)
	}

	ip := net.ParseIP(host)
	if len(ip) == 0 {
		return nil, nil
	}

	return ip, nil
}
