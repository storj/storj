// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package notification_test

import (
	"testing"

	"github.com/stretchr/testify/require"

	"storj.io/storj/internal/testcontext"
	"storj.io/storj/internal/testplanet"
	"storj.io/storj/pkg/pb"
)

func TestProcessNotification(t *testing.T) {
	ctx := testcontext.New(t)
	defer ctx.Cleanup()

	totalSatelliteCount := 3
	planet, err := testplanet.New(t, totalSatelliteCount, 1, 1)
	require.NoError(t, err)
	defer ctx.Check(planet.Shutdown)

	planet.Start(ctx)
	satellite := planet.Satellites[0]
	storagenode := planet.StorageNodes[0]

	err = satellite.Notification.Service.ProcessNotification(&pb.NotificationMessage{
		NodeId:   storagenode.ID(),
		Loglevel: pb.LogLevel_WARN,
		Message:  []byte("test message"),
		Address:  storagenode.Addr(),
	})
	require.NoError(t, err)
}
