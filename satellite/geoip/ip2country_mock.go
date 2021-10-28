// Copyright (C) 2021 Storj Labs, Inc.
// See LICENSE for copying information

package geoip

// MockIPToCountry provides a mock solution for looking up country codes in testplanet tests. This is done using the
// last byte of the ip address and mod'ing it into a country code.
type MockIPToCountry []string

func (m MockIPToCountry) Close() error {
	return nil
}

func (m MockIPToCountry) LookupISOCountryCode(address string) (string, error) {
	ip, err := addressToIP(address)
	if err != nil || ip == nil {
		return "", err
	}

	lastBlock := int(ip[len(ip)-1])
	return m[lastBlock%len(m)], nil
}

var _ IPToCountry = MockIPToCountry{}
