// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package audit_test

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
	"storj.io/storj/internal/testcontext"
	"storj.io/storj/internal/testplanet"
	"storj.io/storj/pkg/audit"
	"storj.io/storj/pkg/overlay"
	"storj.io/storj/pkg/storj"
	"storj.io/storj/satellite"
)

// TestDisqualificationTooManyFailedAudits does the following:
// * Create a failed audit report for a storagenode
// * Record the audit report several times and check that the node isn't
//	 disqualified until the audit reputation reaches the cut-off value.
func TestDisqualificationTooManyFailedAudits(t *testing.T) {
	var auditDQCutOff float64

	testplanet.Run(t, testplanet.Config{
		SatelliteCount: 1, StorageNodeCount: 1, Reconfigure: testplanet.Reconfigure{
			Satellite: func(log *zap.Logger, index int, config *satellite.Config) {
				auditDQCutOff = config.Overlay.Node.AuditReputationDQ
				// TODO: if/v3-1952-h1 this setting will go away once
				// https://github.com/storj/storj/pull/2271 be merge
				config.Audit.MaxRetriesStatDB = 1
			},
		},
	}, func(t *testing.T, ctx *testcontext.Context, planet *testplanet.Planet) {
		err := planet.Satellites[0].Audit.Service.Close()
		require.NoError(t, err)

		var (
			sat    = planet.Satellites[0]
			nodeID = planet.StorageNodes[0].ID()
			report = &audit.Report{
				Fails: storj.NodeIDList{nodeID},
			}
		)

		dossier, err := sat.Overlay.Service.Get(ctx, nodeID)
		require.NoError(t, err)

		prevReputation := calculateStorageNodeReputation(dossier)

		// Report the audit failure until the node gets disqualified due to many
		// failed audits
		for n := 0; ; n++ {
			_, err := sat.Audit.Service.Reporter.RecordAudits(ctx, report)
			require.NoError(t, err)

			dossier, err := sat.Overlay.Service.Get(ctx, nodeID)
			require.NoError(t, err)

			reputation := calculateStorageNodeReputation(dossier)
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

func calculateStorageNodeReputation(dossier *overlay.NodeDossier) float64 {
	var (
		alpha = dossier.Reputation.AuditReputationAlpha
		beta  = dossier.Reputation.AuditReputationBeta
	)

	return alpha / (alpha + beta)
}
