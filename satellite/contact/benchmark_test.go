// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package contact_test

import (
	"context"
	"os"
	"testing"
	"time"

	"storj.io/storj/internal/testcontext"
	"storj.io/storj/pkg/pb"
	"storj.io/storj/pkg/storj"
	"storj.io/storj/satellite"
	"storj.io/storj/satellite/overlay"
	"storj.io/storj/satellite/satellitedb/satellitedbtest"
)

func BenchmarkUpdateCheckIn(b *testing.B) {
	postgresSetup := os.Getenv("STORJ_POSTGRES_TEST")
	if postgresSetup == "" {
		b.Fatal("postgres must be configured with env var: STORJ_SIM_POSTGRES")
		return
	}
	satellitedbtest.Bench(b, func(b *testing.B, db satellite.DB) {
		ctx := testcontext.New(b)
		defer ctx.Cleanup()
		benchmarkOld(ctx, b, db)
		benchmarkNew(ctx, b, db)
	})
}

var node = overlay.NodeCheckInInfo{
	NodeID: storj.NodeID{1},
	Address: &pb.NodeAddress{
		Address: "1.2.4.4",
	},
	IsUp: true,
	Capacity: &pb.NodeCapacity{
		FreeBandwidth: int64(1234),
		FreeDisk:      int64(5678),
	},
	Operator: &pb.NodeOperator{
		Email:  "test@email.com",
		Wallet: "0x123",
	},
	Version: &pb.NodeVersion{
		Version:    "v0.0.0",
		CommitHash: "",
		Timestamp:  time.Time{},
		Release:    false,
	},
}
var config = overlay.NodeSelectionConfig{
	UptimeReputationLambda: 0.99,
	UptimeReputationWeight: 1.0,
	UptimeReputationDQ:     0,
}

func benchmarkOld(ctx context.Context, b *testing.B, db satellite.DB) {
	b.Run("old", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			value := pb.Node{
				Id: node.NodeID,
				Address: &pb.NodeAddress{
					Transport: pb.NodeTransport_TCP_TLS_GRPC,
					Address:   node.Address.GetAddress(),
				},
			}
			err := db.OverlayCache().UpdateAddress(ctx, &value, config)
			if err != nil {
				b.Fatal(err)
				return
			}

			_, err = db.OverlayCache().UpdateUptime(ctx, node.NodeID, node.IsUp, config.UptimeReputationLambda, config.UptimeReputationWeight, config.UptimeReputationDQ)
			if err != nil {
				b.Fatal(err)
				return
			}

			pbInfo := pb.InfoResponse{
				Operator: node.Operator,
				Capacity: node.Capacity,
				Type:     pb.NodeType_STORAGE,
			}
			_, err = db.OverlayCache().UpdateNodeInfo(ctx, node.NodeID, &pbInfo)
			if err != nil {
				b.Fatal(err)
				return
			}

		}
	})
	return
}

func benchmarkNew(ctx context.Context, b *testing.B, db satellite.DB) {
	b.Run("new", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			node.NodeID = storj.NodeID{2}
			err := db.OverlayCache().UpdateCheckIn(ctx, node, config)
			if err != nil {
				b.Fatal(err)
				return
			}

		}
	})
}
