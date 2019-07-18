// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package storagenodedb

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"storj.io/storj/internal/testrand"
	"storj.io/storj/pkg/pb"
	"storj.io/storj/storagenode/orders"
)

func TestOrders_Trivial(t *testing.T) {
	Run(t, func(t *testing.T, ctx context.Context, db *DB) {
		satelliteID, serial := testrand.NodeID(), testrand.SerialNumber()

		{ // Ensure Enqueue works at all
			err := db.Orders().Enqueue(ctx, &orders.Info{
				Order: &pb.Order{},
				Limit: &pb.OrderLimit{
					SatelliteId:     satelliteID,
					SerialNumber:    serial,
					OrderExpiration: time.Now(),
				},
			})
			require.NoError(t, err)
		}

		{ // Ensure ListUnsent works at all
			_, err := db.Orders().ListUnsent(ctx, 1)
			require.NoError(t, err)
		}

		{ // Ensure ListUnsentBySatellite works at all
			_, err := db.Orders().ListUnsentBySatellite(ctx)
			require.NoError(t, err)
		}

		{ // Ensure Archive works at all
			err := db.Orders().Archive(ctx, satelliteID, serial, orders.StatusAccepted)
			require.NoError(t, err)
		}

		{ // Ensure ListArchived works at all
			_, err := db.Orders().ListArchived(ctx, 1)
			require.NoError(t, err)
		}
	})
}
