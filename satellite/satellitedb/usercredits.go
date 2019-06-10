package satellitedb

import (
	"context"

	dbx "storj.io/storj/satellite/satellitedb/dbx"
)

type usercredits struct {
	db dbx.Methods
}

func (c *usercredits) TotalReferredCountByUserID(ctx context.Context)
