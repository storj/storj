// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information

package satellitedb

import (
	"storj.io/storj/satellite/marketing"
	dbx "storj.io/storj/satellite/satellitedb/dbx"
)

// MarketingDB contains access to different satellite databases
type MarketingDB struct {
	db *dbx.DB

	methods dbx.Methods
}

// Offers returns access to offers table
func (db *MarketingDB) Offers() marketing.Offers {
	return &offers{db.db}
}
