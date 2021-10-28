// Copyright (C) 2021 Storj Labs, Inc.
// See LICENSE for copying information

package geoip

import (
	"github.com/oschwald/maxminddb-golang"
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

func (m *MaxmindDB) Close() error {
	return m.db.Close()
}

func (m *MaxmindDB) LookupISOCountryCode(address string) (string, error) {
	ip, err := addressToIP(address)
	if err != nil || ip == nil {
		return "", err
	}

	info := &ipInfo{}
	err = m.db.Lookup(ip, info)
	if err != nil {
		return "", err
	}

	return info.Country.IsoCode, nil
}

var _ IPToCountry = &MaxmindDB{}
