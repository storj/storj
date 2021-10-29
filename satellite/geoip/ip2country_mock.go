// Copyright (C) 2021 Storj Labs, Inc.
// See LICENSE for copying information

package geoip

// MockIPToCountry provides a mock solution for looking up country codes in testplanet tests. This is done using the
// last byte of the ip address and mod'ing it into a country code.
type MockIPToCountry []string

// Close does nothing for the MockIPToCountry.
func (m MockIPToCountry) Close() error {
	return nil
}

// LookupISOCountryCode accepts an IP address.
func (m MockIPToCountry) LookupISOCountryCode(address string) (string, error) {
	if len(m) == 0 {
		return "", nil
	}

	ip, err := addressToIP(address)
	if err != nil || ip == nil {
		return "", err
	}

	lastBlock := int(ip[len(ip)-1])
	return m[lastBlock%len(m)], nil
}

var _ IPToCountry = MockIPToCountry{}
