// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package tempdb

import (
	"context"
	"strings"

	"github.com/zeebo/errs"
	"go.uber.org/zap"

	"storj.io/storj/shared/dbutil"
	"storj.io/storj/shared/dbutil/cockroachutil"
	"storj.io/storj/shared/dbutil/pgutil"
	"storj.io/storj/shared/dbutil/spannerutil"
)

// OpenUnique opens a temporary, uniquely named database (or isolated database schema)
// for scratch work. When closed, this database or schema will be cleaned up and destroyed.
func OpenUnique(ctx context.Context, log *zap.Logger, connURL string, namePrefix string, spannerExtraStatements []string) (*dbutil.TempDatabase, error) {
	if strings.HasPrefix(connURL, "postgres://") || strings.HasPrefix(connURL, "postgresql://") {
		return pgutil.OpenUnique(ctx, connURL, namePrefix)
	}
	if strings.HasPrefix(connURL, "cockroach://") {
		return cockroachutil.OpenUnique(ctx, connURL, namePrefix)
	}
	if strings.HasPrefix(connURL, "spanner://") {
		return spannerutil.OpenUnique(ctx, log, connURL, namePrefix, spannerExtraStatements)
	}
	return nil, errs.New("OpenUnique does not yet support the db type for %q", connURL)
}
