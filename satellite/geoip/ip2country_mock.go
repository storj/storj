// Copyright (C) 2021 Storj Labs, Inc.
// See LICENSE for copying information

package geoip

import "storj.io/storj/shared/location"

// MockIPToCountry provides a mock solution for looking up country codes in testplanet tests. This is done using the
// last byte of the ip address and mod'ing it into a country code.
type MockIPToCountry []location.CountryCode

// NewMockIPToCountry creates a mock IPToCountry based on predefined country list.
func NewMockIPToCountry(countries []string) MockIPToCountry {
	result := MockIPToCountry{}
	for _, country := range countries {
		result = append(result, location.ToCountryCode(country))
	}
	return result
}

// Close does nothing for the MockIPToCountry.
func (m MockIPToCountry) Close() error {
	return nil
}

// LookupISOCountryCode accepts an IP address.
func (m MockIPToCountry) LookupISOCountryCode(address string) (location.CountryCode, error) {
	if len(m) == 0 {
		return location.CountryCode(0), nil
	}

	ip, err := addressToIP(address)
	if err != nil || ip == nil {
		return location.CountryCode(0), err
	}

	lastBlock := int(ip[len(ip)-1])
	return m[lastBlock%len(m)], nil
}

var _ IPToCountry = MockIPToCountry{}
