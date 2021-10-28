// Copyright (C) 2021 Storj Labs, Inc.
// See LICENSE for copying information

package geoip

import (
	"net"

	"github.com/oschwald/maxminddb-golang"
)

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
	host, _, err := net.SplitHostPort(address)
	if err != nil {
		return "", err
	}

	info := &ipInfo{}
	err = m.db.Lookup(net.ParseIP(host), info)
	if err != nil {
		return "", err
	}

	return info.Country.IsoCode, nil
}

var _ IPToCountry = &MaxmindDB{}
