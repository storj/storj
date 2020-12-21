// Copyright (C) 2020 Storj Labs, Inc.
// See LICENSE for copying information.

package satellitedb

import (
	"time"

	"storj.io/storj/private/dbutil"
)

type asOfSystemTimeClause struct {
	interval       time.Duration
	implementation dbutil.Implementation
}

func (aost asOfSystemTimeClause) getClause() (asOf string) {
	if aost.implementation == dbutil.Cockroach && aost.interval < 0 {
		asOf = " AS OF SYSTEM TIME '" + aost.interval.String() + "' "
	}

	return asOf
}
