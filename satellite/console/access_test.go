// Copyright (C) 2026 Storj Labs, Inc.
// See LICENSE for copying information.

package console_test

import (
	"net/http"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"go.uber.org/zap"

	"storj.io/common/grant"
	"storj.io/common/macaroon"
	"storj.io/common/storj"
	"storj.io/common/testcontext"
	"storj.io/storj/private/testplanet"
	"storj.io/storj/satellite"
	"storj.io/storj/satellite/console"
	"storj.io/storj/satellite/kms"
)

func TestCreateRestrictedAccess(t *testing.T) {
	testplanet.Run(t, testplanet.Config{
		SatelliteCount: 1, StorageNodeCount: 0, UplinkCount: 2,
		Reconfigure: testplanet.Reconfigure{
			Satellite: func(log *zap.Logger, index int, config *satellite.Config) {
				config.Console.AccessCreationHttpApiEnabled = true
				config.Console.SatelliteManagedEncryptionEnabled = true
				config.Console.ManagedEncryption.PathEncryptionEnabled = true
				config.KeyManagement.KeyInfos = kms.KeyInfos{
					Values: map[int]kms.KeyInfo{
						1: {SecretVersion: "secretversion1", SecretChecksum: 12345},
					},
				}
			},
		},
	}, func(t *testing.T, ctx *testcontext.Context, planet *testplanet.Planet) {
		sat := planet.Satellites[0]
		s := sat.API.Console.Service

		owner := planet.Uplinks[0]
		ownerUser := owner.Projects[0].Owner.ID
		ownerCtx, err := sat.UserContext(ctx, ownerUser)
		require.NoError(t, err)
		user, err := s.GetUser(ownerCtx, ownerUser)
		require.NoError(t, err)

		project, err := s.GetProject(ownerCtx, owner.Projects[0].ID)
		require.NoError(t, err)

		managedProject, err := s.CreateProject(ownerCtx, console.UpsertProjectInfo{
			Name:             "managed-project",
			ManagePassphrase: true,
		})
		require.NoError(t, err)

		allPermissions := console.AccessPermissions{
			AllowDownload: true, AllowUpload: true, AllowList: true, AllowDelete: true,
		}

		t.Run("access grant with passphrase", func(t *testing.T) {
			resp, httpErr := s.PrivateGenCreateAccess(ownerCtx, user, console.CreateAccessRequest{
				ProjectID:   project.ID,
				Name:        "access-grant-1",
				Permissions: allPermissions,
				Buckets:     []string{"bucket1", "bucket2"},
				Passphrase:  "hunter2",
			})
			require.NoError(t, httpErr.Err)
			require.NotNil(t, resp)
			require.NotEmpty(t, resp.AccessGrant)

			parsed, err := grant.ParseAccess(resp.AccessGrant)
			require.NoError(t, err)
			require.Equal(t, storj.EncAESGCM, parsed.EncAccess.Store.GetDefaultPathCipher())

			m, err := macaroon.ParseMacaroon(parsed.APIKey.SerializeRaw())
			require.NoError(t, err)

			buckets := collectAllowedBuckets(t, m)
			require.ElementsMatch(t, []string{"bucket1", "bucket2"}, buckets)
		})

		t.Run("missing passphrase", func(t *testing.T) {
			_, httpErr := s.PrivateGenCreateAccess(ownerCtx, user, console.CreateAccessRequest{
				ProjectID:   project.ID,
				Name:        "no-passphrase",
				Permissions: allPermissions,
			})
			require.Error(t, httpErr.Err)
			require.Equal(t, http.StatusBadRequest, httpErr.Status)
		})

		t.Run("empty name", func(t *testing.T) {
			_, httpErr := s.PrivateGenCreateAccess(ownerCtx, user, console.CreateAccessRequest{
				ProjectID:   project.ID,
				Name:        "",
				Permissions: allPermissions,
				Passphrase:  "hunter2",
			})
			require.Error(t, httpErr.Err)
			require.Equal(t, http.StatusBadRequest, httpErr.Status)
		})

		t.Run("notAfter before notBefore", func(t *testing.T) {
			earlier := time.Now()
			later := earlier.Add(time.Hour)
			_, httpErr := s.PrivateGenCreateAccess(ownerCtx, user, console.CreateAccessRequest{
				ProjectID:   project.ID,
				Name:        "bad-time-range",
				Permissions: allPermissions,
				Passphrase:  "hunter2",
				NotBefore:   &later,
				NotAfter:    &earlier,
			})
			require.Error(t, httpErr.Err)
			require.Equal(t, http.StatusBadRequest, httpErr.Status)
		})

		t.Run("non-member", func(t *testing.T) {
			outsider := planet.Uplinks[1]
			outsiderCtx, err := sat.UserContext(ctx, outsider.Projects[0].Owner.ID)
			require.NoError(t, err)
			outsiderUser, err := s.GetUser(outsiderCtx, outsider.Projects[0].Owner.ID)
			require.NoError(t, err)

			_, httpErr := s.PrivateGenCreateAccess(outsiderCtx, outsiderUser, console.CreateAccessRequest{
				ProjectID:   project.ID,
				Name:        "outsider",
				Permissions: allPermissions,
				Passphrase:  "hunter2",
			})
			require.Error(t, httpErr.Err)
			require.Equal(t, http.StatusUnauthorized, httpErr.Status)
		})

		t.Run("duplicate name", func(t *testing.T) {
			req := console.CreateAccessRequest{
				ProjectID:   project.ID,
				Name:        "duplicate-name-access",
				Permissions: allPermissions,
				Passphrase:  "hunter2",
			}
			_, httpErr := s.PrivateGenCreateAccess(ownerCtx, user, req)
			require.NoError(t, httpErr.Err)

			_, httpErr = s.PrivateGenCreateAccess(ownerCtx, user, req)
			require.Error(t, httpErr.Err)
			require.Equal(t, http.StatusConflict, httpErr.Status)
		})

		t.Run("managed project without passphrase", func(t *testing.T) {
			resp, httpErr := s.PrivateGenCreateAccess(ownerCtx, user, console.CreateAccessRequest{
				ProjectID:   managedProject.ID,
				Name:        "managed-access",
				Permissions: allPermissions,
			})
			require.NoError(t, httpErr.Err)
			require.NotEmpty(t, resp.AccessGrant)

			parsed, err := grant.ParseAccess(resp.AccessGrant)
			require.NoError(t, err)
			require.Equal(t, storj.EncAESGCM, parsed.EncAccess.Store.GetDefaultPathCipher())
		})

		t.Run("managed project rejects custom passphrase", func(t *testing.T) {
			_, httpErr := s.PrivateGenCreateAccess(ownerCtx, user, console.CreateAccessRequest{
				ProjectID:   managedProject.ID,
				Name:        "managed-rejects-custom",
				Permissions: allPermissions,
				Passphrase:  "custom-not-allowed",
			})
			require.Error(t, httpErr.Err)
			require.Equal(t, http.StatusBadRequest, httpErr.Status)
		})

		t.Run("path encryption disabled", func(t *testing.T) {
			s.TestToggleManagedEncryptionPathEncryption(false)
			defer s.TestToggleManagedEncryptionPathEncryption(true)

			noPathEncProject, err := s.CreateProject(ownerCtx, console.UpsertProjectInfo{
				Name:             "no-path-enc",
				ManagePassphrase: true,
			})
			require.NoError(t, err)

			resp, httpErr := s.PrivateGenCreateAccess(ownerCtx, user, console.CreateAccessRequest{
				ProjectID:   noPathEncProject.ID,
				Name:        "no-path-enc-access",
				Permissions: allPermissions,
			})
			require.NoError(t, httpErr.Err)

			parsed, err := grant.ParseAccess(resp.AccessGrant)
			require.NoError(t, err)
			require.Equal(t, storj.EncNull, parsed.EncAccess.Store.GetDefaultPathCipher())
		})
	})
}

// collectAllowedBuckets returns the set of bucket names allowed by the macaroon's caveats.
func collectAllowedBuckets(t *testing.T, m *macaroon.Macaroon) []string {
	t.Helper()
	var buckets []string
	for _, cb := range m.Caveats() {
		var c macaroon.Caveat
		require.NoError(t, c.UnmarshalBinary(cb))
		for _, p := range c.AllowedPaths {
			buckets = append(buckets, string(p.Bucket))
		}
	}
	return buckets
}
