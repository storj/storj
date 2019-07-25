package satellitedb_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
	"go.uber.org/zap"

	"storj.io/storj/internal/teststorj"
	"storj.io/storj/pkg/pb"
	"storj.io/storj/satellite/overlay"
	"storj.io/storj/satellite/satellitedb"
)

func TestSimple(t *testing.T) {
	db, err := satellitedb.NewInMemory(zap.L().Named("test"))
	require.NoError(t, err)

	cache := db.OverlayCache()

	ctx := context.Background()

	defaults := overlay.NodeSelectionConfig{
		AuditReputationAlpha0:  1,
		AuditReputationBeta0:   1,
		UptimeReputationAlpha0: 1,
		UptimeReputationBeta0:  1,
		UptimeReputationLambda: 1,
		UptimeReputationWeight: 1,
		UptimeReputationDQ:     1,
	}

	node := pb.Node{
		Id: teststorj.NodeIDFromString("testid"),
		Address: &pb.NodeAddress{
			Address: "127.0.0.1:100",
		},
		LastIp: "127.0.0.1",
	}

	err = cache.UpdateAddressAndUptime(ctx, &node, true, defaults)
	require.NoError(t, err)

}
