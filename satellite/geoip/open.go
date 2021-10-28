// Copyright (C) 2021 Storj Labs, Inc.
// See LICENSE for copying information

package geoip

import "github.com/oschwald/maxminddb-golang"

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
