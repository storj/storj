// Copyright (C) 2026 Storj Labs, Inc.
// See LICENSE for copying information.

package metainfo_test

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"storj.io/common/macaroon"
	"storj.io/common/pb"
	"storj.io/common/testcontext"
	"storj.io/storj/private/testplanet"
	"storj.io/storj/satellite/console"
	"storj.io/storj/satellite/entitlements"
)

func TestEndpoint_LicenseInfo(t *testing.T) {
	testplanet.Run(t, testplanet.Config{
		SatelliteCount: 1, UplinkCount: 1,
	}, func(t *testing.T, ctx *testcontext.Context, planet *testplanet.Planet) {
		sat := planet.Satellites[0]
		uplink := planet.Uplinks[0]
		publicID := uplink.Projects[0].PublicID
		apiKey := uplink.APIKey[sat.ID()]

		// Get the user who owns this API key.
		apiKeyInfo, err := sat.DB.Console().APIKeys().GetByHead(ctx, apiKey.Head())
		require.NoError(t, err)
		userID := apiKeyInfo.CreatedBy

		entSvc := sat.API.Entitlements.Service
		now := time.Now().Truncate(time.Second)

		t.Run("no licenses", func(t *testing.T) {
			response, err := sat.Metainfo.Endpoint.LicenseInfo(ctx, &pb.LicenseInfoRequest{
				Header: &pb.RequestHeader{
					ApiKey: apiKey.SerializeRaw(),
				},
			})
			require.NoError(t, err)
			require.NotNil(t, response)
			require.Empty(t, response.Licenses)
		})

		// Set up various licenses for testing.
		licenses := entitlements.AccountLicenses{
			Licenses: []entitlements.AccountLicense{
				{
					// should match all user projects/buckets
					Type:      "pro",
					ExpiresAt: now.Add(30 * 24 * time.Hour),
				},
				{
					// should match only specific bucket
					Type:       "enterprise",
					PublicID:   publicID.String(),
					BucketName: "test-bucket",
					ExpiresAt:  now.Add(60 * 24 * time.Hour),
				},
				{
					// should match all buckets in project
					Type:       "basic",
					PublicID:   publicID.String(),
					BucketName: "",
					ExpiresAt:  now.Add(90 * 24 * time.Hour),
				},
				{
					// should match all user projects/buckets (if not expired)
					Type:      "expired",
					ExpiresAt: now.Add(-24 * time.Hour),
				},
			},
		}
		require.NoError(t, entSvc.Licenses().Set(ctx, userID, licenses))

		t.Run("get all licenses without filters", func(t *testing.T) {
			response, err := sat.Metainfo.Endpoint.LicenseInfo(ctx, &pb.LicenseInfoRequest{
				Header: &pb.RequestHeader{
					ApiKey: apiKey.SerializeRaw(),
				},
			})
			require.NoError(t, err)
			require.NotNil(t, response)

			types := []string{}
			for _, lic := range response.Licenses {
				types = append(types, lic.Type)
			}
			require.ElementsMatch(t, []string{"pro", "enterprise", "basic"}, types)
		})

		t.Run("filter by license type", func(t *testing.T) {
			response, err := sat.Metainfo.Endpoint.LicenseInfo(ctx, &pb.LicenseInfoRequest{
				Header: &pb.RequestHeader{
					ApiKey: apiKey.SerializeRaw(),
				},
				Type: "enterprise",
			})
			require.NoError(t, err)
			require.NotNil(t, response)
			require.Len(t, response.Licenses, 1)
			require.Equal(t, "enterprise", response.Licenses[0].Type)
		})

		t.Run("filter by bucket name", func(t *testing.T) {
			response, err := sat.Metainfo.Endpoint.LicenseInfo(ctx, &pb.LicenseInfoRequest{
				Header: &pb.RequestHeader{
					ApiKey: apiKey.SerializeRaw(),
				},
				BucketName: "test-bucket",
			})
			require.NoError(t, err)
			require.NotNil(t, response)

			types := []string{}
			for _, lic := range response.Licenses {
				types = append(types, lic.Type)
			}
			require.ElementsMatch(t, []string{"pro", "enterprise", "basic"}, types)
		})

		t.Run("filter by both type and bucket", func(t *testing.T) {
			response, err := sat.Metainfo.Endpoint.LicenseInfo(ctx, &pb.LicenseInfoRequest{
				Header: &pb.RequestHeader{
					ApiKey: apiKey.SerializeRaw(),
				},
				Type:       "enterprise",
				BucketName: "test-bucket",
			})
			require.NoError(t, err)
			require.NotNil(t, response)
			require.Len(t, response.Licenses, 1)
			require.Equal(t, "enterprise", response.Licenses[0].Type)
		})

		t.Run("non-matching bucket returns project-wide licenses", func(t *testing.T) {
			response, err := sat.Metainfo.Endpoint.LicenseInfo(ctx, &pb.LicenseInfoRequest{
				Header: &pb.RequestHeader{
					ApiKey: apiKey.SerializeRaw(),
				},
				BucketName: "other-bucket",
			})
			require.NoError(t, err)
			require.NotNil(t, response)

			types := []string{}
			for _, lic := range response.Licenses {
				types = append(types, lic.Type)
			}
			require.ElementsMatch(t, []string{"pro", "basic"}, types)
		})

		t.Run("non-matching type returns empty", func(t *testing.T) {
			response, err := sat.Metainfo.Endpoint.LicenseInfo(ctx, &pb.LicenseInfoRequest{
				Header: &pb.RequestHeader{
					ApiKey: apiKey.SerializeRaw(),
				},
				Type: "nonexistent",
			})
			require.NoError(t, err)
			require.NotNil(t, response)
			require.Empty(t, response.Licenses)
		})

		t.Run("expired licenses excluded", func(t *testing.T) {
			response, err := sat.Metainfo.Endpoint.LicenseInfo(ctx, &pb.LicenseInfoRequest{
				Header: &pb.RequestHeader{
					ApiKey: apiKey.SerializeRaw(),
				},
				Type: "expired",
			})
			require.NoError(t, err)
			require.NotNil(t, response)
			require.Empty(t, response.Licenses)
		})

		t.Run("invalid api key", func(t *testing.T) {
			_, err := sat.Metainfo.Endpoint.LicenseInfo(ctx, &pb.LicenseInfoRequest{
				Header: &pb.RequestHeader{
					ApiKey: []byte("invalid"),
				},
			})
			require.Error(t, err)
		})
	})
}

func TestEndpoint_LicenseInfo_MultipleProjects(t *testing.T) {
	testplanet.Run(t, testplanet.Config{
		SatelliteCount: 1, UplinkCount: 1,
	}, func(t *testing.T, ctx *testcontext.Context, planet *testplanet.Planet) {
		sat := planet.Satellites[0]
		uplink := planet.Uplinks[0]

		// Get the owner of the first project.
		userID := uplink.Projects[0].Owner.ID

		// Create a second project for the same user.
		project2, err := sat.DB.Console().Projects().Insert(ctx, &console.Project{
			Name:        "Project 2",
			Description: "Second project",
			OwnerID:     userID,
		})
		require.NoError(t, err)

		// Create API key for project2.
		apiKey2, err := sat.CreateAPIKey(ctx, project2.ID, userID, macaroon.APIKeyVersionMin)
		require.NoError(t, err)

		public1ID := uplink.Projects[0].PublicID
		now := time.Now().Truncate(time.Second)

		// Set up licenses: one for project1, one for project2.
		licenses := entitlements.AccountLicenses{
			Licenses: []entitlements.AccountLicense{
				{
					Type:      "project1-license",
					PublicID:  public1ID.String(),
					ExpiresAt: now.Add(30 * 24 * time.Hour),
				},
				{
					Type:      "project2-license",
					PublicID:  project2.PublicID.String(),
					ExpiresAt: now.Add(30 * 24 * time.Hour),
				},
			},
		}
		require.NoError(t, sat.API.Entitlements.Service.Licenses().Set(ctx, userID, licenses))

		t.Run("project1 sees only project1 licenses", func(t *testing.T) {
			apiKey1 := uplink.APIKey[sat.ID()]
			response, err := sat.Metainfo.Endpoint.LicenseInfo(ctx, &pb.LicenseInfoRequest{
				Header: &pb.RequestHeader{
					ApiKey: apiKey1.SerializeRaw(),
				},
			})
			require.NoError(t, err)
			require.NotNil(t, response)

			require.Len(t, response.Licenses, 1)
			require.Equal(t, "project1-license", response.Licenses[0].Type)
		})

		t.Run("project2 sees only project2 licenses", func(t *testing.T) {
			response, err := sat.Metainfo.Endpoint.LicenseInfo(ctx, &pb.LicenseInfoRequest{
				Header: &pb.RequestHeader{
					ApiKey: apiKey2.SerializeRaw(),
				},
			})
			require.NoError(t, err)
			require.NotNil(t, response)

			require.Len(t, response.Licenses, 1)
			require.Equal(t, "project2-license", response.Licenses[0].Type)
		})
	})
}

func TestEndpoint_LicenseInfo_ExpiresAtFormat(t *testing.T) {
	testplanet.Run(t, testplanet.Config{
		SatelliteCount: 1, UplinkCount: 1,
	}, func(t *testing.T, ctx *testcontext.Context, planet *testplanet.Planet) {
		sat := planet.Satellites[0]
		uplink := planet.Uplinks[0]
		publicID := uplink.Projects[0].PublicID
		apiKey := uplink.APIKey[sat.ID()]

		// Get the user who owns this API key.
		apiKeyInfo, err := sat.DB.Console().APIKeys().GetByHead(ctx, apiKey.Head())
		require.NoError(t, err)
		userID := apiKeyInfo.CreatedBy

		entSvc := sat.API.Entitlements.Service
		now := time.Now().Truncate(time.Second)
		expiresAt := now.Add(30 * 24 * time.Hour)

		licenses := entitlements.AccountLicenses{
			Licenses: []entitlements.AccountLicense{
				{
					Type:      "test-license",
					PublicID:  publicID.String(),
					ExpiresAt: expiresAt,
				},
			},
		}
		require.NoError(t, entSvc.Licenses().Set(ctx, userID, licenses))

		response, err := sat.Metainfo.Endpoint.LicenseInfo(ctx, &pb.LicenseInfoRequest{
			Header: &pb.RequestHeader{
				ApiKey: apiKey.SerializeRaw(),
			},
		})
		require.NoError(t, err)
		require.NotNil(t, response)
		require.Len(t, response.Licenses, 1)

		// Verify ExpiresAt is returned as a string.
		require.Equal(t, expiresAt.String(), response.Licenses[0].ExpiresAt)
		require.Equal(t, "test-license", response.Licenses[0].Type)
	})
}
