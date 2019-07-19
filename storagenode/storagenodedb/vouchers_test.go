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
)

func TestVouchers_Trivial(t *testing.T) {
	Run(t, func(t *testing.T, ctx context.Context, db *DB) {
		satelliteID := testrand.NodeID()

		{ // Ensure Put works at all
			err := db.Vouchers().Put(ctx, &pb.Voucher{
				SatelliteId: satelliteID,
				Expiration:  time.Now(),
			})
			require.NoError(t, err)
		}

		{ // Ensure NeedVoucher works at all
			_, err := db.Vouchers().NeedVoucher(ctx, satelliteID, time.Hour)
			require.NoError(t, err)
		}

		{ // Ensure GetValid works at all
			_, err := db.Vouchers().GetAll(ctx)
			require.NoError(t, err)
		}
	})
}
