// Copyright (C) 2021 Storj Labs, Inc.
// See LICENSE for copying information

package geoip

import (
	"github.com/oschwald/maxminddb-golang"

	"storj.io/storj/shared/location"
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
	Country            country `maxminddb:"country"`
	RepresentedCountry country `maxminddb:"represented_country"`
}

type country struct {
	IsoCode string `maxminddb:"iso_code"`
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

	return toCountryCode(info), nil
}

func toCountryCode(info *ipInfo) location.CountryCode {
	// it's a tricky situation when represented_country is returned (like an embassy or military base).
	// we have only 1-2 such nodes. it's more safe to exclude them from geofencing.
	if info.RepresentedCountry.IsoCode != "" && info.RepresentedCountry.IsoCode != info.Country.IsoCode {
		return location.None
	}
	return location.ToCountryCode(info.Country.IsoCode)
}
