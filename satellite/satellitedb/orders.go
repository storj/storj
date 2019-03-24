// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package satellitedb

import (
	"context"

	"storj.io/storj/pkg/pb"
	dbx "storj.io/storj/satellite/satellitedb/dbx"
)

type ordersDB struct {
	db *dbx.DB
}

func (db *ordersDB) SaveOrder(ctx context.Context, orderLimit *pb.OrderLimit2, order *pb.Order2) error {
	return nil
}
