// Copyright (C) 2022 Storj Labs, Inc.
// See LICENSE for copying information.

package nodeevents_test

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"go.uber.org/zap"

	"storj.io/common/testcontext"
	"storj.io/storj/private/testplanet"
	"storj.io/storj/satellite"
	"storj.io/storj/satellite/nodeevents"
	"storj.io/storj/satellite/overlay"
	"storj.io/storj/storagenode"
)

type TestNotifier struct {
	notifications map[string][]nodeevents.NodeEvent
}

func (tn *TestNotifier) Notify(ctx context.Context, satellite string, events []nodeevents.NodeEvent) error {
	if len(events) == 0 {
		return nil
	}
	email := events[0].Email
	n := tn.notifications[email]
	n = append(n, events...)
	tn.notifications[email] = n
	return nil
}

func TestNodeEventsChore(t *testing.T) {
	testplanet.Run(t, testplanet.Config{
		SatelliteCount: 1, StorageNodeCount: 2, UplinkCount: 0,
		Reconfigure: testplanet.Reconfigure{
			Satellite: func(log *zap.Logger, index int, config *satellite.Config) {
				config.Overlay.SendNodeEmails = true
				config.NodeEvents.SelectionWaitPeriod = 5 * time.Minute
			},
			StorageNode: func(index int, config *storagenode.Config) {
				config.Operator.Email = "test@storj.test"
			},
		},
	}, func(t *testing.T, ctx *testcontext.Context, planet *testplanet.Planet) {
		sat := planet.Satellites[0]
		node0 := planet.StorageNodes[0]
		node1 := planet.StorageNodes[1]
		// email was reconfigured to be the same for all nodes.
		email := node0.Config.Operator.Email

		chore := sat.NodeEvents.Chore
		chore.Loop.Pause()

		tn := &TestNotifier{
			notifications: make(map[string][]nodeevents.NodeEvent),
		}
		chore.SetNotifier(tn)

		// First, test that chore does not notify because not enough time has elapsed since the oldest event of type Disqualified,
		// with this email, was inserted.
		//
		// DQ nodes. Should create a node events in nodeevents DB.
		require.NoError(t, sat.Overlay.Service.DisqualifyNode(ctx, node0.ID(), overlay.DisqualificationReasonUnknown))
		require.NoError(t, sat.Overlay.Service.DisqualifyNode(ctx, node1.ID(), overlay.DisqualificationReasonUnknown))

		// Trigger chore and check that Notifier.Notify was NOT called with the node events.
		chore.Loop.TriggerWait()

		events := tn.notifications[email]
		require.Empty(t, events)

		// Now, set nowFn on chore to 5 minutes in the future to test that chore does notify for the events.
		futureTime := func() time.Time {
			return time.Now().Add(5 * time.Minute)
		}
		chore.SetNow(futureTime)

		// Trigger chore and check that Notifier.Notify was called with the node events.
		chore.Loop.TriggerWait()

		events = tn.notifications[email]
		require.Len(t, events, 2)
		var foundEvent1, foundEvent2 bool
		for _, e := range events {
			require.Equal(t, email, e.Email)
			require.Equal(t, nodeevents.Disqualified, e.Event)
			if e.NodeID == node0.ID() {
				foundEvent1 = true
			} else if e.NodeID == node1.ID() {
				foundEvent2 = true
			}
		}
		require.True(t, foundEvent1)
		require.True(t, foundEvent2)
	})
}
