// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information

package geoip

import "net"

// MockIPToCountry provides a mock solution for looking up country codes in testplanet tests. This is done using the
// last byte of the ip address and mod'ing it into a country code.
type MockIPToCountry []string

func (m MockIPToCountry) Close() error {
	return nil
}

func (m MockIPToCountry) LookupISOCountryCode(address string) (string, error) {
	host, _, err := net.SplitHostPort(address)
	if err != nil {
		return "", err
	}

	ip := net.ParseIP(host)
	lastBlock := int(ip[len(ip) - 1])

	// mod or div?
	return m[lastBlock % len(m)], nil
}

var _ IPToCountry = MockIPToCountry{}
