// Copyright (C) 2021 Storj Labs, Inc.
// See LICENSE for copying information

package geoip

import (
	"net"

	"github.com/oschwald/maxminddb-golang"
	"github.com/zeebo/errs"

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

// LookupLocationByIP returns location information for the provided IP address,
// including whether it resides in a sanctioned country or subdivision.
func (m *MaxmindDB) LookupLocationByIP(ip net.IP) (LocationInfo, error) {
	result := struct {
		Country struct {
			ISOCode string `maxminddb:"iso_code"`
		} `maxminddb:"country"`
		RegisteredCountry struct {
			ISOCode string `maxminddb:"iso_code"`
		} `maxminddb:"registered_country"`
		Location struct {
			Latitude  float64 `maxminddb:"latitude"`
			Longitude float64 `maxminddb:"longitude"`
		} `maxminddb:"location"`
		Subdivisions []struct {
			GeonameID uint64 `maxminddb:"geoname_id"`
		} `maxminddb:"subdivisions"`
	}{}
	if err := m.db.Lookup(ip, &result); err != nil {
		return LocationInfo{}, errs.Wrap(err)
	}

	// The result should have a country code at a mininum, but just in case.
	if result.Country.ISOCode == "" {
		return LocationInfo{}, errs.New("failed to look up IP %q in MaxMind DB", ip)
	}

	sanctioned := false

	if IsCountrySanctioned(result.Country.ISOCode) || IsCountrySanctioned(result.RegisteredCountry.ISOCode) {
		sanctioned = true
	}
	for _, sub := range result.Subdivisions {
		if IsGeonameIDSanctioned(sub.GeonameID) {
			sanctioned = true
		}
	}

	return LocationInfo{
		Sanctioned:        sanctioned,
		Country:           result.Country.ISOCode,
		RegisteredCountry: result.RegisteredCountry.ISOCode,
		Latitude:          result.Location.Latitude,
		Longitude:         result.Location.Longitude,
	}, nil
}

// LocationInfo describes the location and sanction status of an IP address.
type LocationInfo struct {
	Sanctioned          bool
	Country             string
	RegisteredCountry   string
	Latitude, Longitude float64
}

func toCountryCode(info *ipInfo) location.CountryCode {
	// it's a tricky situation when represented_country is returned (like an embassy or military base).
	// we have only 1-2 such nodes. it's more safe to exclude them from geofencing.
	if info.RepresentedCountry.IsoCode != "" && info.RepresentedCountry.IsoCode != info.Country.IsoCode {
		return location.None
	}
	return location.ToCountryCode(info.Country.IsoCode)
}

// IsCountrySanctioned returns true if the country (by alpha-2 or alpha-3 country
// code) is sanctioned for purposes of best-effort conformance to the Office of
// Foreign Assets Code.
func IsCountrySanctioned(country string) bool {
	switch country {
	case "CU", "CUB": // Cuba
	case "IR", "IRN": // Iran
	case "KP", "PRK": // North Korea
	case "SD", "SDN": // Sudan
	case "SY", "SYR": // Syria
	default:
		return false
	}
	return true
}

// IsGeonameIDSanctioned returns true if the uint64 geonames.org id is sanctioned
// for purposes of best-effort conformance to the Office of Foreign Assets Code.
// It is assumed that IsGeonameIDSanctioned is only called after considering
// IsCountrySanctioned, as IsGeonameIDSanctioned does not currently include
// sanctioned countries.
func IsGeonameIDSanctioned(geonameID uint64) bool {
	switch geonameID {
	case 703883, // https://www.geonames.org/703883/autonomous-republic-of-crimea.html
		694422, // https://www.geonames.org/694422/sebastopol-city.html
		709716, // https://www.geonames.org/709716/donetska-oblast.html
		702657: // https://www.geonames.org/702657/luhanska-oblast.html
	default:
		return false
	}
	return true
}
