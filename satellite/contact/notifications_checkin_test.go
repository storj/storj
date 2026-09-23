// Copyright (C) 2026 Storj Labs, Inc.
// See LICENSE for copying information.

package contact_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"

	"storj.io/common/identity"
	"storj.io/common/pb"
	"storj.io/common/storj"
	"storj.io/storj/private/server"
	"storj.io/storj/satellite"
	"storj.io/storj/satellite/contact"
	"storj.io/storj/shared/mudplanet"
	"storj.io/storj/shared/mudplanet/satellitetest"
	"storj.io/storj/shared/mudplanet/sntest"
)

// TestCheckInNotifications verifies that the notifications defined in the satellite
// configuration are delivered to the storage nodes in the check-in response. Only the
// satellite is started: the test itself acts as the checking in node.
func TestCheckInNotifications(t *testing.T) {
	configured := []contact.Notification{
		{
			Key: "maintenance", Title: "Scheduled maintenance",
			Message: "The satellite will be unavailable for roughly 2 hours.",
			Link:    "https://forum.storj.io/t/scheduled-maintenance/1", LinkLabel: "Forum thread",
		},
		{Key: "policy", Title: "Policy update", Message: "See the forum for details."},
	}

	mudplanet.Run(t, satellitetest.WithDB(
		mudplanet.NewComponent("satellite", satellitetest.Satellite,
			mudplanet.WithRunning[*satellite.EndpointRegistration](),
			mudplanet.WithConfig(func(cfg *contact.Config) {
				cfg.Notifications = configured
				// The check-in below is sent from localhost.
				cfg.AllowPrivateIP = true
			}),
		),
	), func(t *testing.T, ctx context.Context, run mudplanet.RuntimeEnvironment) {
		srv := mudplanet.FindFirst[*server.Server](t, run, "satellite", 0)
		satID := mudplanet.FindFirst[*identity.FullIdentity](t, run, "satellite", 0)

		dialer, err := sntest.CreateRPCDialer()
		require.NoError(t, err)

		conn, err := dialer.DialNodeURL(ctx, storj.NodeURL{
			ID:      satID.ID,
			Address: srv.Addr().String(),
		})
		require.NoError(t, err)
		defer func() { require.NoError(t, conn.Close()) }()

		// The satellite pings the node back and fails, as this test doesn't serve the
		// contact endpoint. That is reported in the response instead of failing the
		// check-in, so the notifications are still delivered.
		resp, err := pb.NewDRPCNodeClient(conn).CheckIn(ctx, &pb.CheckInRequest{
			Address: srv.Addr().String(),
			Version: &pb.NodeVersion{Version: "v0.0.0"},
		})
		require.NoError(t, err)

		require.NotNil(t, resp.Notifications)
		received := resp.Notifications.Notifications
		require.Len(t, received, len(configured))
		for i, expected := range configured {
			require.Equal(t, expected.Key, received[i].Key)
			require.Equal(t, expected.Title, received[i].Title)
			require.Equal(t, expected.Message, received[i].Message)
			require.Equal(t, expected.Link, received[i].Link)
			require.Equal(t, expected.LinkLabel, received[i].LinkLabel)
		}
	})
}
