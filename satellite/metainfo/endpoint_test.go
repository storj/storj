// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package metainfo_test

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"

	"storj.io/common/errs2"
	"storj.io/common/macaroon"
	"storj.io/common/pb"
	"storj.io/common/rpc/rpcstatus"
	"storj.io/common/signing"
	"storj.io/common/storj"
	"storj.io/common/testcontext"
	"storj.io/common/testrand"
	"storj.io/storj/private/testplanet"
	"storj.io/storj/satellite"
	"storj.io/storj/satellite/internalpb"
	"storj.io/storj/satellite/metainfo"
	"storj.io/uplink"
	"storj.io/uplink/private/metaclient"
)

var randomEncryptedKey = testrand.Bytes(48)

func TestEndpoint_NoStorageNodes(t *testing.T) {
	testplanet.Run(t, testplanet.Config{
		SatelliteCount: 1, UplinkCount: 3,
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

			_, err = client.BeginObject(ctx, metaclient.BeginObjectParams{})
			assert.True(t, errs2.IsRPC(err, rpcstatus.PermissionDenied))

			_, err = client.BeginDeleteObject(ctx, metaclient.BeginDeleteObjectParams{})
			assert.True(t, errs2.IsRPC(err, rpcstatus.PermissionDenied))

			_, err = client.ListBuckets(ctx, metaclient.ListBucketsParams{})
			assert.True(t, errs2.IsRPC(err, rpcstatus.PermissionDenied))

			_, _, err = client.ListObjects(ctx, metaclient.ListObjectsParams{})
			assert.True(t, errs2.IsRPC(err, rpcstatus.PermissionDenied))

			_, err = client.CreateBucket(ctx, metaclient.CreateBucketParams{})
			assert.True(t, errs2.IsRPC(err, rpcstatus.PermissionDenied))

			_, err = client.DeleteBucket(ctx, metaclient.DeleteBucketParams{})
			assert.True(t, errs2.IsRPC(err, rpcstatus.PermissionDenied))

			_, err = client.BeginDeleteObject(ctx, metaclient.BeginDeleteObjectParams{})
			assert.True(t, errs2.IsRPC(err, rpcstatus.PermissionDenied))

			_, err = client.GetBucket(ctx, metaclient.GetBucketParams{})
			assert.True(t, errs2.IsRPC(err, rpcstatus.PermissionDenied))

			_, err = client.GetObject(ctx, metaclient.GetObjectParams{})
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
			apiKey0 := planet.Uplinks[0].APIKey[planet.Satellites[0].ID()]
			apiKey1 := planet.Uplinks[1].APIKey[planet.Satellites[0].ID()]

			metainfo0, err := planet.Uplinks[0].DialMetainfo(ctx, planet.Satellites[0], apiKey0)
			require.NoError(t, err)

			metainfo1, err := planet.Uplinks[0].DialMetainfo(ctx, planet.Satellites[0], apiKey1)
			require.NoError(t, err)

			info0, err := metainfo0.GetProjectInfo(ctx)
			require.NoError(t, err)
			require.NotNil(t, info0.ProjectSalt)

			info1, err := metainfo1.GetProjectInfo(ctx)
			require.NoError(t, err)
			require.NotNil(t, info1.ProjectSalt)

			// Different projects should have different salts
			require.NotEqual(t, info0.ProjectSalt, info1.ProjectSalt)
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
				return ul.CreateBucket(ctx, satellite, testrand.BucketName())
			})
		}
		groupErrs := group.Wait()
		require.Len(t, groupErrs, 1)
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
				return ul.CreateBucket(ctx, satellite, testrand.BucketName())
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
				return ul.CreateBucket(ctx, satellite, testrand.BucketName())
			})
		}
		groupErrs := group.Wait()
		require.Len(t, groupErrs, 1)
	})
}

func TestRateLimit_ProjectRateLimitOverrideCachedExpired(t *testing.T) {
	testplanet.Run(t, testplanet.Config{
		SatelliteCount: 1, StorageNodeCount: 0, UplinkCount: 1,
		Reconfigure: testplanet.Reconfigure{
			Satellite: func(log *zap.Logger, index int, config *satellite.Config) {
				config.Metainfo.RateLimiter.Rate = 2
				config.Metainfo.RateLimiter.CacheExpiration = time.Second
			},
		},
	}, func(t *testing.T, ctx *testcontext.Context, planet *testplanet.Planet) {
		ul := planet.Uplinks[0]
		satellite := planet.Satellites[0]

		// TODO find a way to reset limiter before test is executed, currently
		// testplanet is doing one additional request to get access
		time.Sleep(2 * time.Second)

		projects, err := satellite.DB.Console().Projects().GetAll(ctx)
		require.NoError(t, err)
		require.Len(t, projects, 1)

		rateLimit := 3
		projects[0].RateLimit = &rateLimit

		err = satellite.DB.Console().Projects().Update(ctx, &projects[0])
		require.NoError(t, err)

		var group1 errs2.Group

		for i := 0; i <= rateLimit; i++ {
			group1.Go(func() error {
				return ul.CreateBucket(ctx, satellite, testrand.BucketName())
			})
		}
		group1Errs := group1.Wait()
		require.Len(t, group1Errs, 1)

		rateLimit = 1
		projects[0].RateLimit = &rateLimit

		err = satellite.DB.Console().Projects().Update(ctx, &projects[0])
		require.NoError(t, err)

		time.Sleep(2 * time.Second)

		var group2 errs2.Group

		for i := 0; i <= rateLimit; i++ {
			group2.Go(func() error {
				return ul.CreateBucket(ctx, satellite, testrand.BucketName())
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
				return ul.CreateBucket(ctx, satellite, testrand.BucketName())
			})
		}
		groupErrs := group.Wait()
		require.Len(t, groupErrs, 1)

		projects, err := satellite.DB.Console().Projects().GetAll(ctx)
		require.NoError(t, err)
		require.Len(t, projects, 1)

		zeroRateLimit := 0
		err = satellite.DB.Console().Projects().UpdateBurstLimit(ctx, projects[0].ID, zeroRateLimit)
		require.NoError(t, err)

		time.Sleep(1 * time.Second)

		var group2 errs2.Group
		for i := 0; i <= burstLimit; i++ {
			group2.Go(func() error {
				return ul.CreateBucket(ctx, satellite, testrand.BucketName())
			})
		}
		group2Errs := group2.Wait()
		require.Len(t, group2Errs, burstLimit+1)

	})
}
