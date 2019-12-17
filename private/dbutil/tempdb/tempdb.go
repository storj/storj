// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package tempdb

import (
	"strings"

	"github.com/zeebo/errs"

	"storj.io/storj/private/dbutil"
	"storj.io/storj/private/dbutil/cockroachutil"
	"storj.io/storj/private/dbutil/pgutil"
)

// OpenUnique opens a temporary, uniquely named database (or isolated database schema)
// for scratch work. When closed, this database or schema will be cleaned up and destroyed.
func OpenUnique(connURL string, namePrefix string) (*dbutil.TempDatabase, error) {
	if strings.HasPrefix(connURL, "postgres://") || strings.HasPrefix(connURL, "postgresql://") {
		return pgutil.OpenUnique(connURL, namePrefix)
	}
	if strings.HasPrefix(connURL, "cockroach://") {
		return cockroachutil.OpenUnique(connURL, namePrefix)
	}
	return nil, errs.New("OpenUnique does not yet support the db type for %q", connURL)
}
