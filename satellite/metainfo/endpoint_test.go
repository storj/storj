// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package metainfo_test

import (
	"crypto/tls"
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"

	"storj.io/common/errs2"
	"storj.io/common/macaroon"
	"storj.io/common/pb"
	"storj.io/common/rpc/rpcpeer"
	"storj.io/common/rpc/rpcstatus"
	"storj.io/common/signing"
	"storj.io/common/storj"
	"storj.io/common/testcontext"
	"storj.io/common/testrand"
	"storj.io/storj/private/testplanet"
	"storj.io/storj/satellite"
	"storj.io/storj/satellite/console"
	"storj.io/storj/satellite/internalpb"
	"storj.io/storj/satellite/metainfo"
	"storj.io/uplink"
	"storj.io/uplink/private/metaclient"
)

var (
	randomEncryptedKey = testrand.Bytes(48)
	randomBucketName   = []byte(testrand.BucketName())
)

func TestEndpoint_NoStorageNodes(t *testing.T) {
	var (
		authUrl             = "auth.storj.io"
		publicLinksharing   = "public-link.storj.io"
		internalLinksharing = "link.storj.io"
	)

	testplanet.Run(t, testplanet.Config{
		SatelliteCount: 1, UplinkCount: 3,
		Reconfigure: testplanet.Reconfigure{
			SatelliteDBOptions: testplanet.SatelliteDBDisableCaches,
			Satellite: func(log *zap.Logger, index int, config *satellite.Config) {
				config.Metainfo.SendEdgeUrlOverrides = true
				config.Metainfo.APIKeyTailsConfig.CombinerQueueEnabled = true
				err := config.Console.PlacementEdgeURLOverrides.Set(
					fmt.Sprintf(`{
						"1": {
							"authService": "%s",
							"publicLinksharing": "%s",
							"internalLinksharing": "%s"
						}
					}`, authUrl, publicLinksharing, internalLinksharing),
				)
				require.NoError(t, err)
			},
		},
	}, func(t *testing.T, ctx *testcontext.Context, planet *testplanet.Planet) {
		t.Run("revoke access", func(t *testing.T) {
			accessIssuer := planet.Uplinks[0].Access[planet.Satellites[0].ID()]
			accessUser1, err := accessIssuer.Share(uplink.Permission{
				AllowDownload: true,
				AllowUpload:   true,
				AllowList:     true,
				AllowDelete:   true,
			})
			require.NoError(t, err)
			accessUser2, err := accessUser1.Share(uplink.Permission{
				AllowDownload: true,
				AllowUpload:   true,
				AllowList:     true,
				AllowDelete:   true,
			})
			require.NoError(t, err)

			projectUser2, err := uplink.OpenProject(ctx, accessUser2)
			require.NoError(t, err)
			defer ctx.Check(projectUser2.Close)

			// confirm that we can create a bucket
			_, err = projectUser2.CreateBucket(ctx, "bob")
			require.NoError(t, err)

			// we shouldn't be allowed to revoke ourselves or our parent
			err = projectUser2.RevokeAccess(ctx, accessUser2)
			assert.True(t, errs2.IsRPC(err, rpcstatus.PermissionDenied))
			err = projectUser2.RevokeAccess(ctx, accessUser1)
			assert.True(t, errs2.IsRPC(err, rpcstatus.PermissionDenied))

			projectIssuer, err := uplink.OpenProject(ctx, accessIssuer)
			require.NoError(t, err)
			defer ctx.Check(projectIssuer.Close)

			projectUser1, err := uplink.OpenProject(ctx, accessUser1)
			require.NoError(t, err)
			defer ctx.Check(projectUser1.Close)

			// I should be able to revoke with accessIssuer
			err = projectIssuer.RevokeAccess(ctx, accessUser1)
			require.NoError(t, err)

			// should no longer be able to create bucket with access 2 or 3
			_, err = projectUser2.CreateBucket(ctx, "bob1")
			assert.True(t, errs2.IsRPC(err, rpcstatus.PermissionDenied))
			_, err = projectUser1.CreateBucket(ctx, "bob1")
			assert.True(t, errs2.IsRPC(err, rpcstatus.PermissionDenied))
		})
		t.Run("revoke macaroon", func(t *testing.T) {
			// I want the api key for the single satellite in this test
			up := planet.Uplinks[2]
			apiKey := up.APIKey[planet.Satellites[0].ID()]

			client, err := up.DialMetainfo(ctx, planet.Satellites[0], apiKey)
			require.NoError(t, err)
			defer ctx.Check(client.Close)

			// Sanity check: it should work before revoke
			_, err = client.ListBuckets(ctx, metaclient.ListBucketsParams{
				ListOpts: metaclient.BucketListOptions{
					Cursor:    "",
					Direction: metaclient.Forward,
					Limit:     10,
				},
			})
			require.NoError(t, err)

			err = planet.Satellites[0].API.DB.Revocation().Revoke(ctx, apiKey.Tail(), []byte("apikey"))
			require.NoError(t, err)

			_, err = client.ListBuckets(ctx, metaclient.ListBucketsParams{})
			assert.True(t, errs2.IsRPC(err, rpcstatus.PermissionDenied))

			_, err = client.BeginObject(ctx, metaclient.BeginObjectParams{
				Bucket: []byte(testrand.BucketName()),
			})
			assert.True(t, errs2.IsRPC(err, rpcstatus.PermissionDenied))

			_, err = client.BeginDeleteObject(ctx, metaclient.BeginDeleteObjectParams{
				Bucket: randomBucketName,
			})
			assert.True(t, errs2.IsRPC(err, rpcstatus.PermissionDenied))

			_, err = client.ListBuckets(ctx, metaclient.ListBucketsParams{})
			assert.True(t, errs2.IsRPC(err, rpcstatus.PermissionDenied))

			_, _, err = client.ListObjects(ctx, metaclient.ListObjectsParams{
				Bucket: randomBucketName,
			})
			assert.True(t, errs2.IsRPC(err, rpcstatus.PermissionDenied))

			_, err = client.CreateBucket(ctx, metaclient.CreateBucketParams{})
			assert.True(t, errs2.IsRPC(err, rpcstatus.PermissionDenied))

			_, err = client.DeleteBucket(ctx, metaclient.DeleteBucketParams{})
			assert.True(t, errs2.IsRPC(err, rpcstatus.PermissionDenied))

			_, err = client.BeginDeleteObject(ctx, metaclient.BeginDeleteObjectParams{
				Bucket: randomBucketName,
			})
			assert.True(t, errs2.IsRPC(err, rpcstatus.PermissionDenied))

			_, err = client.GetBucket(ctx, metaclient.GetBucketParams{})
			assert.True(t, errs2.IsRPC(err, rpcstatus.PermissionDenied))

			_, err = client.GetObject(ctx, metaclient.GetObjectParams{
				Bucket: randomBucketName,
			})
			assert.True(t, errs2.IsRPC(err, rpcstatus.PermissionDenied))

			_, err = client.GetProjectInfo(ctx)
			assert.True(t, errs2.IsRPC(err, rpcstatus.PermissionDenied))

			signer := signing.SignerFromFullIdentity(planet.Satellites[0].Identity)
			satStreamID := &internalpb.StreamID{
				CreationDate: time.Now(),
			}
			signedStreamID, err := metainfo.SignStreamID(ctx, signer, satStreamID)
			require.NoError(t, err)

			encodedStreamID, err := pb.Marshal(signedStreamID)
			require.NoError(t, err)

			err = client.CommitObject(ctx, metaclient.CommitObjectParams{StreamID: encodedStreamID})
			assert.True(t, errs2.IsRPC(err, rpcstatus.PermissionDenied))

			_, err = client.BeginSegment(ctx, metaclient.BeginSegmentParams{StreamID: encodedStreamID})
			assert.True(t, errs2.IsRPC(err, rpcstatus.PermissionDenied))

			err = client.MakeInlineSegment(ctx, metaclient.MakeInlineSegmentParams{StreamID: encodedStreamID})
			assert.True(t, errs2.IsRPC(err, rpcstatus.PermissionDenied))

			_, err = client.DownloadSegmentWithRS(ctx, metaclient.DownloadSegmentParams{StreamID: encodedStreamID})
			assert.True(t, errs2.IsRPC(err, rpcstatus.PermissionDenied))

			_, err = client.ListSegments(ctx, metaclient.ListSegmentsParams{StreamID: encodedStreamID})
			assert.True(t, errs2.IsRPC(err, rpcstatus.PermissionDenied))

			// these methods needs SegmentID

			signedSegmentID, err := metainfo.SignSegmentID(ctx, signer, &internalpb.SegmentID{
				StreamId:     satStreamID,
				CreationDate: time.Now(),
			})
			require.NoError(t, err)

			encodedSegmentID, err := pb.Marshal(signedSegmentID)
			require.NoError(t, err)

			segmentID, err := storj.SegmentIDFromBytes(encodedSegmentID)
			require.NoError(t, err)

			err = client.CommitSegment(ctx, metaclient.CommitSegmentParams{SegmentID: segmentID})
			assert.True(t, errs2.IsRPC(err, rpcstatus.PermissionDenied))
		})

		t.Run("invalid api key", func(t *testing.T) {
			throwawayKey, err := macaroon.NewAPIKey([]byte("secret"))
			require.NoError(t, err)

			for _, invalidAPIKey := range []string{"", "invalid", "testKey"} {
				func() {
					client, err := planet.Uplinks[0].DialMetainfo(ctx, planet.Satellites[0], throwawayKey)
					require.NoError(t, err)
					defer ctx.Check(client.Close)

					client.SetRawAPIKey([]byte(invalidAPIKey))

					_, err = client.BeginObject(ctx, metaclient.BeginObjectParams{})
					assertInvalidArgument(t, err, false)

					_, err = client.BeginDeleteObject(ctx, metaclient.BeginDeleteObjectParams{})
					assertInvalidArgument(t, err, false)

					_, err = client.ListBuckets(ctx, metaclient.ListBucketsParams{})
					assertInvalidArgument(t, err, false)

					_, _, err = client.ListObjects(ctx, metaclient.ListObjectsParams{})
					assertInvalidArgument(t, err, false)

					_, err = client.CreateBucket(ctx, metaclient.CreateBucketParams{})
					assertInvalidArgument(t, err, false)

					_, err = client.DeleteBucket(ctx, metaclient.DeleteBucketParams{})
					assertInvalidArgument(t, err, false)

					_, err = client.BeginDeleteObject(ctx, metaclient.BeginDeleteObjectParams{})
					assertInvalidArgument(t, err, false)

					_, err = client.GetBucket(ctx, metaclient.GetBucketParams{})
					assertInvalidArgument(t, err, false)

					_, err = planet.Satellites[0].Metainfo.Endpoint.GetBucketLocation(ctx, &pb.GetBucketLocationRequest{
						Header: &pb.RequestHeader{
							ApiKey: []byte(invalidAPIKey),
						},
					})
					assertInvalidArgument(t, err, false)

					_, err = planet.Satellites[0].Metainfo.Endpoint.GetBucketVersioning(ctx, &pb.GetBucketVersioningRequest{
						Header: &pb.RequestHeader{
							ApiKey: []byte(invalidAPIKey),
						},
					})
					assertInvalidArgument(t, err, false)

					_, err = planet.Satellites[0].Metainfo.Endpoint.SetBucketVersioning(ctx, &pb.SetBucketVersioningRequest{
						Header: &pb.RequestHeader{
							ApiKey: []byte(invalidAPIKey),
						},
					})
					assertInvalidArgument(t, err, false)

					_, err = client.GetObject(ctx, metaclient.GetObjectParams{})
					assertInvalidArgument(t, err, false)

					_, err = client.GetProjectInfo(ctx)
					assertInvalidArgument(t, err, false)

					// these methods needs StreamID to do authentication

					signer := signing.SignerFromFullIdentity(planet.Satellites[0].Identity)
					satStreamID := &internalpb.StreamID{
						CreationDate: time.Now(),
					}
					signedStreamID, err := metainfo.SignStreamID(ctx, signer, satStreamID)
					require.NoError(t, err)

					encodedStreamID, err := pb.Marshal(signedStreamID)
					require.NoError(t, err)

					streamID, err := storj.StreamIDFromBytes(encodedStreamID)
					require.NoError(t, err)

					err = client.CommitObject(ctx, metaclient.CommitObjectParams{StreamID: streamID})
					assertInvalidArgument(t, err, false)

					_, err = client.BeginSegment(ctx, metaclient.BeginSegmentParams{StreamID: streamID})
					assertInvalidArgument(t, err, false)

					err = client.MakeInlineSegment(ctx, metaclient.MakeInlineSegmentParams{StreamID: streamID})
					assertInvalidArgument(t, err, false)

					_, err = client.DownloadSegmentWithRS(ctx, metaclient.DownloadSegmentParams{StreamID: streamID})
					assertInvalidArgument(t, err, false)

					_, err = client.ListSegments(ctx, metaclient.ListSegmentsParams{StreamID: streamID})
					assertInvalidArgument(t, err, false)

					// these methods needs SegmentID

					signedSegmentID, err := metainfo.SignSegmentID(ctx, signer, &internalpb.SegmentID{
						StreamId:     satStreamID,
						CreationDate: time.Now(),
					})
					require.NoError(t, err)

					encodedSegmentID, err := pb.Marshal(signedSegmentID)
					require.NoError(t, err)

					segmentID, err := storj.SegmentIDFromBytes(encodedSegmentID)
					require.NoError(t, err)

					err = client.CommitSegment(ctx, metaclient.CommitSegmentParams{SegmentID: segmentID})
					assertInvalidArgument(t, err, false)
				}()
			}
		})

		t.Run("get project info", func(t *testing.T) {
			sat := planet.Satellites[0]
			upl1 := planet.Uplinks[0]
			apiKey0 := upl1.APIKey[sat.ID()]
			apiKey1 := planet.Uplinks[1].APIKey[sat.ID()]

			project, err := sat.DB.Console().Projects().Get(ctx, planet.Uplinks[0].Projects[0].ID)
			require.NoError(t, err)

			metainfo0, err := upl1.DialMetainfo(ctx, sat, apiKey0)
			require.NoError(t, err)

			metainfo1, err := upl1.DialMetainfo(ctx, sat, apiKey1)
			require.NoError(t, err)

			info0, err := metainfo0.GetProjectInfo(ctx)
			require.NoError(t, err)
			require.NotNil(t, info0.ProjectSalt)
			require.Nil(t, info0.EdgeUrlOverrides)

			info1, err := metainfo1.GetProjectInfo(ctx)
			require.NoError(t, err)
			require.NotNil(t, info1.ProjectSalt)
			require.Nil(t, info0.EdgeUrlOverrides)

			// Different projects should have different salts
			require.NotEqual(t, info0.ProjectSalt, info1.ProjectSalt)

			err = sat.API.DB.Console().Projects().UpdateDefaultPlacement(ctx, upl1.Projects[0].ID, storj.PlacementConstraint(1))
			require.NoError(t, err)

			info0, err = metainfo0.GetProjectInfo(ctx)
			require.NoError(t, err)
			require.NotNil(t, info0.ProjectSalt)
			require.NotNil(t, info0.EdgeUrlOverrides)
			require.WithinDuration(t, info0.ProjectCreatedAt, project.CreatedAt, time.Nanosecond)
			require.Equal(t, authUrl, string(info0.EdgeUrlOverrides.AuthService))
			require.Equal(t, publicLinksharing, string(info0.EdgeUrlOverrides.PublicLinksharing))
			require.Equal(t, internalLinksharing, string(info0.EdgeUrlOverrides.PrivateLinksharing))
		})

		t.Run("check IDs", func(t *testing.T) {
			apiKey := planet.Uplinks[0].APIKey[planet.Satellites[0].ID()]
			metainfoClient, err := planet.Uplinks[0].DialMetainfo(ctx, planet.Satellites[0], apiKey)
			require.NoError(t, err)
			defer ctx.Check(metainfoClient.Close)

			{
				streamID := testrand.StreamID(256)
				err = metainfoClient.CommitObject(ctx, metaclient.CommitObjectParams{
					StreamID: streamID,
				})
				require.Error(t, err) // invalid streamID

				segmentID := testrand.SegmentID(512)
				err = metainfoClient.CommitSegment(ctx, metaclient.CommitSegmentParams{
					SegmentID: segmentID,
				})
				require.Error(t, err) // invalid segmentID
			}

			satellitePeer := signing.SignerFromFullIdentity(planet.Satellites[0].Identity)

			{ // streamID expired
				signedStreamID, err := metainfo.SignStreamID(ctx, satellitePeer, &internalpb.StreamID{
					CreationDate: time.Now().Add(-36 * time.Hour),
				})
				require.NoError(t, err)

				encodedStreamID, err := pb.Marshal(signedStreamID)
				require.NoError(t, err)

				streamID, err := storj.StreamIDFromBytes(encodedStreamID)
				require.NoError(t, err)

				err = metainfoClient.CommitObject(ctx, metaclient.CommitObjectParams{
					StreamID: streamID,
				})
				require.Error(t, err)
			}

			{ // segment id missing stream id
				signedSegmentID, err := metainfo.SignSegmentID(ctx, satellitePeer, &internalpb.SegmentID{
					CreationDate: time.Now().Add(-1 * time.Hour),
				})
				require.NoError(t, err)

				encodedSegmentID, err := pb.Marshal(signedSegmentID)
				require.NoError(t, err)

				segmentID, err := storj.SegmentIDFromBytes(encodedSegmentID)
				require.NoError(t, err)

				err = metainfoClient.CommitSegment(ctx, metaclient.CommitSegmentParams{
					SegmentID: segmentID,
				})
				require.Error(t, err)
			}

			{ // segmentID expired
				signedSegmentID, err := metainfo.SignSegmentID(ctx, satellitePeer, &internalpb.SegmentID{
					CreationDate: time.Now().Add(-36 * time.Hour),
					StreamId: &internalpb.StreamID{
						CreationDate: time.Now(),
					},
				})
				require.NoError(t, err)

				encodedSegmentID, err := pb.Marshal(signedSegmentID)
				require.NoError(t, err)

				segmentID, err := storj.SegmentIDFromBytes(encodedSegmentID)
				require.NoError(t, err)

				err = metainfoClient.CommitSegment(ctx, metaclient.CommitSegmentParams{
					SegmentID: segmentID,
				})
				require.Error(t, err)
			}
		})
	})
}

func assertInvalidArgument(t *testing.T, err error, allowed bool) {
	t.Helper()

	// If it's allowed, we allow any non-unauthenticated error because
	// some calls error after authentication checks.
	if !allowed {
		assert.True(t, errs2.IsRPC(err, rpcstatus.InvalidArgument))
	}
}

func TestAPIKeyTails(t *testing.T) {
	testplanet.Run(t, testplanet.Config{
		SatelliteCount: 1, UplinkCount: 1,
		Reconfigure: testplanet.Reconfigure{
			Satellite: func(log *zap.Logger, index int, config *satellite.Config) {
				config.Metainfo.APIKeyTailsConfig.CombinerQueueEnabled = true
			},
		},
	}, func(t *testing.T, ctx *testcontext.Context, planet *testplanet.Planet) {
		sat := planet.Satellites[0]
		upl := planet.Uplinks[0]

		secret, err := macaroon.NewSecret()
		require.NoError(t, err)
		apiKey, err := macaroon.NewAPIKey(secret)
		require.NoError(t, err)

		keyInfo, err := sat.DB.Console().APIKeys().Create(ctx, apiKey.Head(), console.APIKeyInfo{
			Name:      "test-key",
			ProjectID: upl.Projects[0].ID,
			Secret:    secret,
			Version:   macaroon.APIKeyVersionMin,
		})
		require.NoError(t, err)

		caveat0 := macaroon.Caveat{DisallowDeletes: true}
		macaroon0, err := apiKey.Restrict(caveat0)
		require.NoError(t, err)
		require.NotNil(t, macaroon0)

		caveat1 := macaroon.Caveat{DisallowWrites: true}
		macaroon1, err := macaroon0.Restrict(caveat1)
		require.NoError(t, err)
		require.NotNil(t, macaroon1)

		metainfoClient, err := upl.DialMetainfo(ctx, sat, macaroon1)
		require.NoError(t, err)
		defer ctx.Check(metainfoClient.Close)

		_, err = metainfoClient.ListBuckets(ctx, metaclient.ListBucketsParams{
			ListOpts: metaclient.BucketListOptions{
				Cursor:    "",
				Direction: metaclient.Forward,
				Limit:     10,
			},
		})
		require.NoError(t, err)

		err = sat.Metainfo.Endpoint.TestWaitForTailsCombinerWorkers(ctx)
		require.NoError(t, err)

		tailsDB := sat.DB.Console().APIKeyTails()

		tail0, err := tailsDB.GetByTail(ctx, macaroon0.Tail())
		require.NoError(t, err)
		require.NotNil(t, tail0)
		require.EqualValues(t, keyInfo.ID, tail0.RootKeyID)
		require.EqualValues(t, macaroon0.Tail(), tail0.Tail)
		require.EqualValues(t, apiKey.Tail(), tail0.ParentTail)
		require.WithinDuration(t, time.Now(), tail0.LastUsed, time.Minute)

		parsedCaveat, err := macaroon.ParseCaveat(tail0.Caveat)
		require.NoError(t, err)
		require.NotNil(t, parsedCaveat)
		require.EqualValues(t, caveat0, *parsedCaveat)

		tail1, err := tailsDB.GetByTail(ctx, macaroon1.Tail())
		require.NoError(t, err)
		require.NotNil(t, tail1)
		require.EqualValues(t, keyInfo.ID, tail1.RootKeyID)
		require.EqualValues(t, macaroon1.Tail(), tail1.Tail)
		require.EqualValues(t, macaroon0.Tail(), tail1.ParentTail)
		require.WithinDuration(t, time.Now(), tail1.LastUsed, time.Minute)

		parsedCaveat, err = macaroon.ParseCaveat(tail1.Caveat)
		require.NoError(t, err)
		require.NotNil(t, parsedCaveat)
		require.EqualValues(t, caveat1, *parsedCaveat)
	})
}

func TestAuditableAPIKeyValidation(t *testing.T) {
	testplanet.Run(t, testplanet.Config{
		SatelliteCount: 1, UplinkCount: 1,
		Reconfigure: testplanet.Reconfigure{
			Satellite: func(log *zap.Logger, index int, config *satellite.Config) {
				config.Metainfo.APIKeyTailsConfig.CombinerQueueEnabled = true
			},
		},
	}, func(t *testing.T, ctx *testcontext.Context, planet *testplanet.Planet) {
		sat := planet.Satellites[0]
		upl := planet.Uplinks[0]

		t.Run("auditable key with unregistered tail should be rejected", func(t *testing.T) {
			secret, err := macaroon.NewSecret()
			require.NoError(t, err)
			apiKey, err := macaroon.NewAPIKey(secret)
			require.NoError(t, err)

			_, err = sat.DB.Console().APIKeys().Create(ctx, apiKey.Head(), console.APIKeyInfo{
				Name:      "auditable-key",
				ProjectID: upl.Projects[0].ID,
				Secret:    secret,
				Version:   macaroon.APIKeyVersionAuditable,
			})
			require.NoError(t, err)

			caveat := macaroon.Caveat{DisallowDeletes: true}
			restrictedKey, err := apiKey.Restrict(caveat)
			require.NoError(t, err)

			metainfoClient, err := upl.DialMetainfo(ctx, sat, restrictedKey)
			require.NoError(t, err)
			defer ctx.Check(metainfoClient.Close)

			_, err = metainfoClient.ListBuckets(ctx, metaclient.ListBucketsParams{
				ListOpts: metaclient.BucketListOptions{
					Cursor:    "",
					Direction: metaclient.Forward,
					Limit:     10,
				},
			})
			require.Error(t, err)
			require.True(t, errs2.IsRPC(err, rpcstatus.PermissionDenied))
		})

		t.Run("auditable key with registered tail should work", func(t *testing.T) {
			secret, err := macaroon.NewSecret()
			require.NoError(t, err)
			apiKey, err := macaroon.NewAPIKey(secret)
			require.NoError(t, err)

			keyInfo, err := sat.DB.Console().APIKeys().Create(ctx, apiKey.Head(), console.APIKeyInfo{
				Name:      "auditable-key-registered",
				ProjectID: upl.Projects[0].ID,
				Secret:    secret,
				Version:   macaroon.APIKeyVersionAuditable,
			})
			require.NoError(t, err)

			caveat := macaroon.Caveat{DisallowDeletes: true}
			restrictedKey, err := apiKey.Restrict(caveat)
			require.NoError(t, err)

			mac, err := macaroon.ParseMacaroon(restrictedKey.SerializeRaw())
			require.NoError(t, err)
			tails := mac.Tails(keyInfo.Secret)
			caveats := mac.Caveats()

			restrictedTail := &console.APIKeyTail{
				RootKeyID:  keyInfo.ID,
				Tail:       tails[1],
				ParentTail: tails[0],
				Caveat:     caveats[0],
				LastUsed:   time.Now(),
			}

			_, err = sat.DB.Console().APIKeyTails().Upsert(ctx, restrictedTail)
			require.NoError(t, err)

			metainfoClient, err := upl.DialMetainfo(ctx, sat, restrictedKey)
			require.NoError(t, err)
			defer ctx.Check(metainfoClient.Close)

			_, err = metainfoClient.ListBuckets(ctx, metaclient.ListBucketsParams{
				ListOpts: metaclient.BucketListOptions{
					Cursor:    "",
					Direction: metaclient.Forward,
					Limit:     10,
				},
			})
			require.NoError(t, err)
		})
	})
}

func TestRateLimit(t *testing.T) {
	rateLimit := 2
	testplanet.Run(t, testplanet.Config{
		SatelliteCount: 1, StorageNodeCount: 0, UplinkCount: 1,
		Reconfigure: testplanet.Reconfigure{
			Satellite: func(log *zap.Logger, index int, config *satellite.Config) {
				config.Metainfo.RateLimiter.Rate = float64(rateLimit)
				config.Metainfo.RateLimiter.CacheExpiration = 500 * time.Millisecond
			},
		},
	}, func(t *testing.T, ctx *testcontext.Context, planet *testplanet.Planet) {
		ul := planet.Uplinks[0]
		satellite := planet.Satellites[0]

		// TODO find a way to reset limiter before test is executed, currently
		// testplanet is doing one additional request to get access
		time.Sleep(1 * time.Second)

		var group errs2.Group
		for i := 0; i <= rateLimit; i++ {
			group.Go(func() error {
				_, err := ul.ListBuckets(ctx, satellite)
				return err
			})
		}
		groupErrs := group.Wait()
		require.Len(t, groupErrs, 1)
	})
}

func TestDisableRateLimit(t *testing.T) {
	t.Skip("flaky")
	testplanet.Run(t, testplanet.Config{
		SatelliteCount: 1, StorageNodeCount: 1, UplinkCount: 1,
		Reconfigure: testplanet.Reconfigure{
			Satellite: func(log *zap.Logger, index int, config *satellite.Config) {
				config.Metainfo.RateLimiter.Rate = float64(2)
			},
		},
	}, func(t *testing.T, ctx *testcontext.Context, planet *testplanet.Planet) {
		// TODO find a way to reset limiter before test is executed, currently
		// testplanet is doing one additional request to get access
		time.Sleep(1 * time.Second)

		apiKey := planet.Uplinks[0].APIKey[planet.Satellites[0].ID()]
		satellite := planet.Satellites[0]

		require.NoError(t, planet.Uplinks[0].TestingCreateBucket(ctx, planet.Satellites[0], "test-bucket"))

		peerctx := rpcpeer.NewContext(ctx, &rpcpeer.Peer{
			State: tls.ConnectionState{
				PeerCertificates: planet.Uplinks[0].Identity.Chain(),
			}})

		start := time.Now()
		beginObjResponse, err := satellite.Metainfo.Endpoint.BeginObject(peerctx, &pb.BeginObjectRequest{
			Header: &pb.RequestHeader{
				ApiKey: apiKey.SerializeRaw(),
			},
			Bucket:             []byte("test-bucket"),
			EncryptedObjectKey: []byte("test-object"),
			EncryptionParameters: &pb.EncryptionParameters{
				CipherSuite: pb.CipherSuite_ENC_AESGCM,
			},
		})
		require.NoError(t, err)

		var group errs2.Group
		for i := 0; i <= 5; i++ {
			group.Go(func() error {
				_, err := satellite.Metainfo.Endpoint.BeginSegment(peerctx, &pb.BeginSegmentRequest{
					Header: &pb.RequestHeader{
						ApiKey: apiKey.SerializeRaw(),
					},
					StreamId: beginObjResponse.StreamId,
					Position: &pb.SegmentPosition{},
				})
				return err
			})

		}
		groupErrs := group.Wait()
		require.Empty(t, groupErrs)
		// check that test didn't take enough time to pass one way or another
		require.WithinDuration(t, start, time.Now(), 2*time.Second)
	})
}

func TestRateLimit_Disabled(t *testing.T) {
	rateLimit := 2
	testplanet.Run(t, testplanet.Config{
		SatelliteCount: 1, StorageNodeCount: 0, UplinkCount: 1,
		Reconfigure: testplanet.Reconfigure{
			Satellite: func(log *zap.Logger, index int, config *satellite.Config) {
				config.Metainfo.RateLimiter.Enabled = false
				config.Metainfo.RateLimiter.Rate = float64(rateLimit)
			},
		},
	}, func(t *testing.T, ctx *testcontext.Context, planet *testplanet.Planet) {
		ul := planet.Uplinks[0]
		satellite := planet.Satellites[0]

		var group errs2.Group
		for i := 0; i <= rateLimit; i++ {
			group.Go(func() error {
				return ul.TestingCreateBucket(ctx, satellite, testrand.BucketName())
			})
		}
		groupErrs := group.Wait()
		require.Len(t, groupErrs, 0)
	})
}

func TestRateLimit_ProjectRateLimitOverride(t *testing.T) {
	testplanet.Run(t, testplanet.Config{
		SatelliteCount: 1, StorageNodeCount: 1, UplinkCount: 1,
		Reconfigure: testplanet.Reconfigure{
			Satellite: func(log *zap.Logger, index int, config *satellite.Config) {
				config.Metainfo.RateLimiter.Rate = 2
				config.Metainfo.RateLimiter.CacheExpiration = 500 * time.Millisecond
			},
		},
	}, func(t *testing.T, ctx *testcontext.Context, planet *testplanet.Planet) {
		ul := planet.Uplinks[0]
		satellite := planet.Satellites[0]

		// TODO find a way to reset limiter before test is executed, currently
		// testplanet is doing one additional request to get access
		time.Sleep(1 * time.Second)

		projects, err := satellite.DB.Console().Projects().GetAll(ctx)
		require.NoError(t, err)
		require.Len(t, projects, 1)

		rateLimit := 3
		projects[0].RateLimit = &rateLimit

		err = satellite.DB.Console().Projects().Update(ctx, &projects[0])
		require.NoError(t, err)

		var group errs2.Group
		for i := 0; i <= rateLimit; i++ {
			group.Go(func() error {
				_, err := ul.ListBuckets(ctx, satellite)
				return err
			})
		}
		groupErrs := group.Wait()
		require.Len(t, groupErrs, 1)
	})
}

func TestRateLimit_ProjectRateLimitOverrideCachedExpired(t *testing.T) {
	testplanet.Run(t, testplanet.Config{
		SatelliteCount: 1, UplinkCount: 1,
		Reconfigure: testplanet.Reconfigure{
			Satellite: func(log *zap.Logger, index int, config *satellite.Config) {
				config.Metainfo.RateLimiter.Rate = 2
				config.Metainfo.RateLimiter.CacheExpiration = 2 * time.Second
			},
			SatelliteDBOptions: testplanet.SatelliteDBDisableCaches,
		},
	}, func(t *testing.T, ctx *testcontext.Context, planet *testplanet.Planet) {
		apiKey := planet.Uplinks[0].APIKey[planet.Satellites[0].ID()]

		satellite := planet.Satellites[0]

		projects, err := satellite.DB.Console().Projects().GetAll(ctx)
		require.NoError(t, err)
		require.Len(t, projects, 1)

		rateLimit := 3
		projects[0].RateLimit = &rateLimit

		err = satellite.DB.Console().Projects().Update(ctx, &projects[0])
		require.NoError(t, err)

		limiter := satellite.Metainfo.Endpoint.TestingGetLimiterCache()
		limiter.Reset()

		listBuckets := func() error {
			_, err := satellite.Metainfo.Endpoint.ListBuckets(ctx, &pb.ListBucketsRequest{
				Header: &pb.RequestHeader{
					ApiKey: apiKey.SerializeRaw(),
				},
				Direction: pb.ListDirection_AFTER,
			})
			return err
		}

		var group1 errs2.Group
		for range rateLimit + 1 {
			group1.Go(func() error {
				return listBuckets()
			})
		}
		group1Errs := group1.Wait()
		require.Len(t, group1Errs, 1)

		rateLimit = 1
		projects[0].RateLimit = &rateLimit

		err = satellite.DB.Console().Projects().Update(ctx, &projects[0])
		require.NoError(t, err)

		time.Sleep(4 * time.Second)

		var group2 errs2.Group
		for range rateLimit + 1 {
			group2.Go(func() error {
				return listBuckets()
			})
		}
		group2Errs := group2.Wait()
		require.Len(t, group2Errs, 1)
	})
}

func TestRateLimit_ExceededBurstLimit(t *testing.T) {
	burstLimit := 2
	testplanet.Run(t, testplanet.Config{
		SatelliteCount: 1, StorageNodeCount: 1, UplinkCount: 1,
		Reconfigure: testplanet.Reconfigure{
			Satellite: func(log *zap.Logger, index int, config *satellite.Config) {
				config.Metainfo.RateLimiter.Rate = float64(burstLimit)
				config.Metainfo.RateLimiter.CacheExpiration = 500 * time.Millisecond
			},
			SatelliteDBOptions: testplanet.SatelliteDBDisableCaches,
		},
	}, func(t *testing.T, ctx *testcontext.Context, planet *testplanet.Planet) {
		ul := planet.Uplinks[0]
		satellite := planet.Satellites[0]

		// TODO find a way to reset limiter before test is executed, currently
		// testplanet is doing one additional request to get access
		time.Sleep(1 * time.Second)

		var group errs2.Group
		for i := 0; i <= burstLimit; i++ {
			group.Go(func() error {
				_, err := ul.ListBuckets(ctx, satellite)
				return err
			})
		}
		groupErrs := group.Wait()
		require.Len(t, groupErrs, 1)

		projects, err := satellite.DB.Console().Projects().GetAll(ctx)
		require.NoError(t, err)
		require.Len(t, projects, 1)

		zeroRateLimit := 0
		err = satellite.DB.Console().Projects().UpdateBurstLimit(ctx, projects[0].ID, &zeroRateLimit)
		require.NoError(t, err)

		time.Sleep(1 * time.Second)

		var group2 errs2.Group
		for i := 0; i <= burstLimit; i++ {
			group2.Go(func() error {
				_, err := ul.ListBuckets(ctx, satellite)
				return err
			})
		}
		group2Errs := group2.Wait()
		require.Len(t, group2Errs, burstLimit+1)

	})
}
