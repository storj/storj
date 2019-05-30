// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package vouchers_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"

	"storj.io/storj/internal/testcontext"
	"storj.io/storj/internal/testplanet"
	"storj.io/storj/pkg/overlay"
	"storj.io/storj/pkg/pb"
	"storj.io/storj/satellite"
	"storj.io/storj/storagenode"
)

func TestVouchers(t *testing.T) {
	testplanet.Run(t, testplanet.Config{
		SatelliteCount: 1, StorageNodeCount: 2, UplinkCount: 0,
		Reconfigure: testplanet.Reconfigure{
			Satellite: func(log *zap.Logger, index int, config *satellite.Config) {
				config.Overlay.Node.AuditCount = 1
			},
		},
	}, func(t *testing.T, ctx *testcontext.Context, planet *testplanet.Planet) {
		tests := []struct {
			node        *storagenode.Peer
			reputable   bool
			expectedErr string
		}{
			{
				node:      planet.StorageNodes[0],
				reputable: true,
			},
			{
				node:        planet.StorageNodes[1],
				reputable:   false,
				expectedErr: "rpc error: code = Unknown desc = vouchers error: Request rejected. Node not reputable",
			},
		}

		satellite := planet.Satellites[0].Local().Node

		for _, tt := range tests {
			if tt.reputable {
				_, err := planet.Satellites[0].DB.OverlayCache().UpdateStats(ctx, &overlay.UpdateRequest{
					NodeID:       tt.node.ID(),
					IsUp:         true,
					AuditSuccess: true,
				})
				require.NoError(t, err)
			}

			conn, err := tt.node.Transport.DialNode(ctx, &satellite)
			require.NoError(t, err)

			client := pb.NewVouchersClient(conn)

			voucher, err := client.Request(ctx, &pb.VoucherRequest{})
			if tt.reputable {
				assert.NoError(t, err)
				assert.NotNil(t, voucher)
				assert.Equal(t, tt.node.ID(), voucher.StorageNodeId)
			} else {
				assert.Equal(t, tt.expectedErr, err.Error())
				assert.Nil(t, voucher)
			}
		}
	})
}
