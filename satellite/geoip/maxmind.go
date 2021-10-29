// Copyright (C) 2021 Storj Labs, Inc.
// See LICENSE for copying information

package geoip

import (
	"github.com/oschwald/maxminddb-golang"

	"storj.io/common/storj/location"
)

// OpenMaxmindDB will use the provided filepath to open the target maxmind database.
func OpenMaxmindDB(filepath string) (*MaxmindDB, error) {
	geoIP, err := maxminddb.Open(filepath)
	if err != nil {
		return nil, err
	}

	return &MaxmindDB{
		db: geoIP,
	}, nil
}

type ipInfo struct {
	Country struct {
		IsoCode string `maxminddb:"iso_code"`
	} `maxminddb:"country"`
}

// MaxmindDB provides access to GeoIP data via the maxmind geoip databases.
type MaxmindDB struct {
	db *maxminddb.Reader
}

var _ IPToCountry = &MaxmindDB{}

// Close will disconnect the underlying connection to the database.
func (m *MaxmindDB) Close() error {
	return m.db.Close()
}

// LookupISOCountryCode accepts an IP address.
func (m *MaxmindDB) LookupISOCountryCode(address string) (location.CountryCode, error) {
	ip, err := addressToIP(address)
	if err != nil || ip == nil {
		return location.CountryCode(0), err
	}

	info := &ipInfo{}
	err = m.db.Lookup(ip, info)
	if err != nil {
		return location.CountryCode(0), err
	}

	return location.ToCountryCode(info.Country.IsoCode), nil
}
