// Copyright (C) 2021 Storj Labs, Inc.
// See LICENSE for copying information

package geoip

import (
	"io"
	"net"
	"strings"
)

// IPToCountry defines an abstraction for resolving the ISO country code given the string representation of an IP address.
type IPToCountry interface {
	io.Closer
	LookupISOCountryCode(address string) (string, error)
}

func addressToIP(address string) (net.IP, error) {
	host, _, err := net.SplitHostPort(address)
	if err != nil && strings.HasSuffix(err.Error(), "missing port in address") {
		host = address

		// trim for IPv6
		host = strings.TrimPrefix(host, "[")
		host = strings.TrimSuffix(host, "]")
	} else if err != nil {
		return nil, err
	}

	ip := net.ParseIP(host)
	if len(ip) == 0 {
		return nil, nil
	}

	return ip, nil
}
