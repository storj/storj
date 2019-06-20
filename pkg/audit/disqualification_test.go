// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package audit_test

import (
	"crypto/rand"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
	"storj.io/storj/internal/memory"
	"storj.io/storj/internal/testcontext"
	"storj.io/storj/internal/testplanet"
	"storj.io/storj/pkg/audit"
	"storj.io/storj/satellite"
)

// TestDisqualificationTooManyFailedAudits does the following:
// * Uploads random data
// * Select one stripe
// * Delete the piece from one of the storage nodes to simulate a missing piece
// * Create and use a verifier to audit such stripe and get a report
// * Verify that the report contains an audit failure of node whose piece has
//   has been deleted.
// * Record the audit report several times and check that the node gets
//   disqualified.
func TestDisqualificationTooManyFailedAudits(t *testing.T) {
	var auditDQCutOff float64

	testplanet.Run(t, testplanet.Config{
		SatelliteCount: 1, StorageNodeCount: 4, UplinkCount: 1, Reconfigure: testplanet.Reconfigure{
			Satellite: func(log *zap.Logger, index int, config *satellite.Config) {
				auditDQCutOff = config.Overlay.Node.AuditReputationDQ
				config.Audit.MaxRetriesStatDB = 1
			},
		},
	}, func(t *testing.T, ctx *testcontext.Context, planet *testplanet.Planet) {
		err := planet.Satellites[0].Audit.Service.Close()
		require.NoError(t, err)

		testData := make([]byte, 8*memory.KiB)
		_, err = rand.Read(testData)
		require.NoError(t, err)

		var (
			ul  = planet.Uplinks[0]
			sat = planet.Satellites[0]
		)
		err = ul.Upload(ctx, sat, "testbucket", "test/path", testData)
		require.NoError(t, err)

		cursor := audit.NewCursor(sat.Metainfo.Service)
		stripe, _, err := cursor.NextStripe(ctx)
		require.NoError(t, err)

		// Get the id from a node
		nodeID := stripe.Segment.GetRemote().GetRemotePieces()[0].NodeId
		{ // delete the piece from the selected node
			pieceID := stripe.Segment.GetRemote().RootPieceId.Derive(nodeID)
			node := getStorageNode(planet, nodeID)
			err = node.Storage2.Store.Delete(ctx, planet.Satellites[0].ID(), pieceID)
			require.NoError(t, err)
		}

		verifier := audit.NewVerifier(
			zap.L(),
			sat.Metainfo.Service,
			sat.Transport,
			sat.Overlay.Service,
			sat.DB.Containment(),
			sat.Orders.Service,
			sat.Identity,
			128*memory.B,
			5*time.Second,
		)

		report, err := verifier.Verify(ctx, stripe, nil)
		require.NoError(t, err)
		require.Len(t, report.Offlines, 0)
		require.Len(t, report.PendingAudits, 0)
		require.Len(t, report.Fails, 1)
		require.Equal(t, nodeID, report.Fails[0])

		dossier, err := sat.Overlay.Service.Get(ctx, nodeID)
		require.NoError(t, err)

		var prevReputation float64
		{
			alpha := dossier.Reputation.AuditReputationAlpha
			beta := dossier.Reputation.AuditReputationBeta
			prevReputation = alpha / (alpha + beta)
		}

		// Report the audit failure until the node gets disqualified due to many
		// failed audits
		for n := 0; ; n++ {
			_, err := sat.Audit.Service.Reporter.RecordAudits(ctx, report)
			require.NoError(t, err)

			dossier, err := sat.Overlay.Service.Get(ctx, nodeID)
			require.NoError(t, err)

			var (
				alpha = dossier.Reputation.AuditReputationAlpha
				beta  = dossier.Reputation.AuditReputationBeta
			)

			reputation := alpha / (alpha + beta)
			require.Truef(t, prevReputation >= reputation,
				"(%d) expected reputation to remain or decrease (previous >= current): %f >= %f",
				n, prevReputation, reputation,
			)

			if reputation <= auditDQCutOff || reputation == prevReputation {
				require.NotNilf(t, dossier.Disqualified,
					"Disqualified (%d) - cut-off: %f, prev. reputation: %f, current reputation: %f",
					n, auditDQCutOff, prevReputation, reputation,
				)

				require.True(t, time.Now().Sub(*dossier.Disqualified) >= 0,
					"Disqualified should be in the past",
				)

				break
			}

			require.Nil(t, dossier.Disqualified, "Disqualified")
			prevReputation = reputation
		}
	})
}
