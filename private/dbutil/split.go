// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package dbutil

import (
	"fmt"
	"strings"
)

// SplitConnStr returns the driver and DSN portions of a URL, along with the db implementation.
func SplitConnStr(s string) (driver string, source string, implementation Implementation, err error) {
	// consider https://github.com/xo/dburl if this ends up lacking
	parts := strings.SplitN(s, "://", 2)
	if len(parts) != 2 {
		return "", "", Unknown, fmt.Errorf("could not parse DB URL %s", s)
	}
	driver = parts[0]
	source = parts[1]
	implementation = ImplementationForScheme(parts[0])

	switch implementation {
	case Postgres:
		source = s // postgres wants full URLS for its DSN
		driver = "pgx"
	case Cockroach:
		source = s // cockroach wants full URLS for its DSN
		driver = "pgxcockroach"
	}
	return driver, source, implementation, nil
}
