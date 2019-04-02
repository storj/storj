// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package satellitedb

import (
	"context"
	"time"

	"github.com/skyrings/skyring-common/tools/uuid"

	"storj.io/storj/satellite/console"
	dbx "storj.io/storj/satellite/satellitedb/dbx"
)

// usagerollups implements console.UsageRollups
type usagerollups struct {
	db dbx.Methods
}

func (*usagerollups) GetProjectTotal(ctx context.Context, projectID uuid.UUID, since, before time.Time) (*console.ProjectUsage, error) {
	return nil, nil
}

func (*usagerollups) Get(ctx context.Context, projectID uuid.UUID, bucketID []byte, before time.Time, count int) ([]console.UsageRollup, error) {
	panic("implement me")
}
