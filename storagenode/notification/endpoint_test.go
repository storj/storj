// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package notification_test

import (
	"testing"

	"github.com/stretchr/testify/require"

	"storj.io/storj/pkg/pb"
	"storj.io/storj/private/testcontext"
	"storj.io/storj/private/testplanet"
)

func TestProcessNotification(t *testing.T) {
	ctx := testcontext.New(t)
	defer ctx.Cleanup()

	totalSatelliteCount := 3
	planet, err := testplanet.New(t, totalSatelliteCount, 1, 1)
	require.NoError(t, err)
	defer ctx.Check(planet.Shutdown)

	planet.Start(ctx)
	storagenode := planet.StorageNodes[0]
	_, err = storagenode.Notification.Endpoint.ProcessNotification(ctx, &pb.NotificationMessage{
		NodeId:   storagenode.ID(),
		Loglevel: pb.LogLevel_INFO,
		Message:  []byte("test"),
		Address:  storagenode.Addr(),
	})
	require.NoError(t, err)
}
