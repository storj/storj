// Copyright (C) 2021 Storj Labs, Inc.
// See LICENSE for copying information.

package metainfo_test

import (
	"context"
	"crypto/tls"
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
	"go.uber.org/zap/zaptest"

	"storj.io/common/errs2"
	"storj.io/common/macaroon"
	"storj.io/common/pb"
	"storj.io/common/rpc/rpcpeer"
	"storj.io/common/rpc/rpcstatus"
	"storj.io/common/testcontext"
	"storj.io/common/testrand"
	"storj.io/storj/private/testplanet"
	"storj.io/storj/satellite"
	"storj.io/storj/satellite/buckets"
	"storj.io/storj/satellite/console"
	"storj.io/storj/satellite/console/consoleauth"
	"storj.io/storj/satellite/metainfo"
)

type mockAPIKeys struct {
	secret []byte
}

func (m *mockAPIKeys) GetByHead(ctx context.Context, head []byte) (*console.APIKeyInfo, error) {
	return &console.APIKeyInfo{Secret: m.secret}, nil
}

var _ metainfo.APIKeys = (*mockAPIKeys)(nil)

func TestEndpoint_validateAuthN(t *testing.T) {
	ctx := testcontext.New(t)
	defer ctx.Cleanup()

	secret, err := macaroon.NewSecret()
	require.NoError(t, err)

	key, err := macaroon.NewAPIKey(secret)
	require.NoError(t, err)

	keyNoLists, err := key.Restrict(macaroon.Caveat{DisallowLists: true})
	require.NoError(t, err)

	keyNoListsNoDeletes, err := keyNoLists.Restrict(macaroon.Caveat{DisallowDeletes: true})
	require.NoError(t, err)

	endpoint := metainfo.TestingNewAPIKeysEndpoint(zaptest.NewLogger(t), &mockAPIKeys{secret: secret})

	now := time.Now()

	var canRead, canList, canDelete bool

	set1 := []metainfo.VerifyPermission{
		{
			Action: macaroon.Action{
				Op:   macaroon.ActionDelete,
				Time: now,
			},
		},
		{
			Action: macaroon.Action{
				Op:   macaroon.ActionRead,
				Time: now,
			},
			ActionPermitted: &canRead,
			Optional:        true,
		},
		{
			Action: macaroon.Action{
				Op:   macaroon.ActionList,
				Time: now,
			},
			ActionPermitted: &canList,
			Optional:        true,
		},
	}
	set2 := []metainfo.VerifyPermission{
		{
			Action: macaroon.Action{
				Op:   macaroon.ActionWrite,
				Time: now,
			},
		},
		{
			Action: macaroon.Action{
				Op:   macaroon.ActionDelete,
				Time: now,
			},
			ActionPermitted: &canDelete,
			Optional:        true,
		},
	}
	set3 := []metainfo.VerifyPermission{
		{
			Action: macaroon.Action{
				Op:   macaroon.ActionDelete,
				Time: now,
			},
		},
		{
			Action: macaroon.Action{
				Op:   macaroon.ActionRead,
				Time: now,
			},
			ActionPermitted: &canRead,
			Optional:        true,
		},
		{
			Action: macaroon.Action{
				Op:   macaroon.ActionList,
				Time: now,
			},
			ActionPermitted: &canList,
			Optional:        true,
		},
	}

	for i, tt := range [...]struct {
		key                                     *macaroon.APIKey
		permissions                             []metainfo.VerifyPermission
		wantCanRead, wantCanList, wantCanDelete bool
		wantErr                                 bool
	}{
		{
			key:     key,
			wantErr: true,
		},

		{
			key:         key,
			permissions: make([]metainfo.VerifyPermission, 2),
			wantErr:     true,
		},

		{
			key: key,
			permissions: []metainfo.VerifyPermission{
				{
					Action: macaroon.Action{
						Op:   macaroon.ActionWrite,
						Time: now,
					},
					Optional: true,
				},
				{
					Action: macaroon.Action{
						Op:   macaroon.ActionDelete,
						Time: now,
					},
					Optional: true,
				},
			},
			wantErr: true,
		},

		{
			key: key,
			permissions: []metainfo.VerifyPermission{
				{
					Action: macaroon.Action{
						Op:   macaroon.ActionProjectInfo,
						Time: now,
					},
				},
			},
		},

		{
			key:         key,
			permissions: set1,
			wantCanRead: true,
			wantCanList: true,
		},
		{
			key:         keyNoLists,
			permissions: set1,
			wantCanRead: true,
		},
		{
			key:         keyNoListsNoDeletes,
			permissions: set1,
			wantErr:     true,
		},

		{
			key:           key,
			permissions:   set2,
			wantCanDelete: true,
		},
		{
			key:           keyNoLists,
			permissions:   set2,
			wantCanDelete: true,
		},
		{
			key:         keyNoListsNoDeletes,
			permissions: set2,
		},

		{
			key:         key,
			permissions: set3,
			wantCanRead: true,
			wantCanList: true,
		},
		{
			key:         keyNoLists,
			permissions: set3,
			wantCanRead: true,
		},
		{
			key:         keyNoListsNoDeletes,
			permissions: set3,
			wantErr:     true,
		},
	} {
		canRead, canList, canDelete = false, false, false // reset state

		rawKey := tt.key.SerializeRaw()
		ctxWithKey := consoleauth.WithAPIKey(ctx, rawKey)

		_, err := endpoint.ValidateAuthN(ctxWithKey, &pb.RequestHeader{ApiKey: rawKey}, console.RateLimit, tt.permissions...)

		assert.Equal(t, err != nil, tt.wantErr, i)
		assert.Equal(t, tt.wantCanRead, canRead, i)
		assert.Equal(t, tt.wantCanList, canList, i)
		assert.Equal(t, tt.wantCanDelete, canDelete, i)
	}
}

func TestEndpoint_checkRate(t *testing.T) {
	testplanet.Run(t, testplanet.Config{
		SatelliteCount: 1, StorageNodeCount: 1, UplinkCount: 2,
		Reconfigure: testplanet.Reconfigure{
			Satellite: func(log *zap.Logger, index int, config *satellite.Config) {
				// make global rate/burst limit 1
				config.Metainfo.RateLimiter.Rate = 1
				config.Metainfo.RateLimiter.CacheExpiration = time.Hour
				config.Metainfo.DownloadLimiter.Enabled = false
			},
			SatelliteDBOptions: testplanet.SatelliteDBDisableCaches,
		},
	},
		func(t *testing.T, ctx *testcontext.Context, planet *testplanet.Planet) {
			sat := planet.Satellites[0]
			endpoint := sat.Metainfo.Endpoint
			projects := sat.API.DB.Console().Projects()
			apiKeys := sat.API.DB.Console().APIKeys()

			proj, err := projects.Get(ctx, planet.Uplinks[0].Projects[0].ID)
			require.NoError(t, err)
			ownerAPIKey := planet.Uplinks[0].APIKey[sat.ID()]

			// ensure all rate limits for this project are null
			require.Nil(t, proj.RateLimit)
			require.Nil(t, proj.BurstLimit)
			require.Nil(t, proj.RateLimitHead)
			require.Nil(t, proj.BurstLimitHead)
			require.Nil(t, proj.RateLimitGet)
			require.Nil(t, proj.BurstLimitGet)
			require.Nil(t, proj.RateLimitPut)
			require.Nil(t, proj.BurstLimitPut)
			require.Nil(t, proj.RateLimitList)
			require.Nil(t, proj.BurstLimitList)
			require.Nil(t, proj.RateLimitDelete)
			require.Nil(t, proj.BurstLimitDelete)

			// ensure global rate limit is hit when project limits are nil
			listReq := &pb.ListBucketsRequest{
				Header: &pb.RequestHeader{
					ApiKey: ownerAPIKey.SerializeRaw(),
				},
				Direction: buckets.DirectionForward,
			}

			// Mock the rate limiter time to depend on the execution time
			rateLimiterTime := time.Now()
			endpoint.TestingSetRateLimiterTime(func() time.Time {
				return rateLimiterTime
			})

			_, err = endpoint.ListBuckets(ctx, listReq)
			require.NoError(t, err)
			_, err = endpoint.ListBuckets(ctx, listReq)
			require.Error(t, err)
			require.True(t, errs2.IsRPC(err, rpcstatus.ResourceExhausted))

			rate := int64(1)
			burstProject := int64(2)
			burstHead := int64(3)
			burstGet := int64(4)
			burstList := int64(5)
			burstDelete := int64(6)
			burstPut := int64(7)

			// switch project so that cached rate limiter isn't used for next test stage
			peerctx := rpcpeer.NewContext(ctx, &rpcpeer.Peer{
				State: tls.ConnectionState{
					PeerCertificates: planet.Uplinks[1].Identity.Chain(),
				}})
			proj, err = projects.Get(ctx, planet.Uplinks[1].Projects[0].ID)
			require.NoError(t, err)
			ownerAPIKey = planet.Uplinks[1].APIKey[sat.ID()]
			listReq.Header.ApiKey = ownerAPIKey.SerializeRaw()

			// set project "rate limit" and "burst limit"
			err = projects.UpdateLimitsGeneric(ctx, proj.ID, []console.Limit{
				{Kind: console.RateLimit, Value: &rate},
				{Kind: console.BurstLimit, Value: &burstProject},
			})
			require.NoError(t, err)
			keyInfo, err := apiKeys.GetByHead(ctx, ownerAPIKey.Head())
			require.NoError(t, err)
			require.NotNil(t, keyInfo.ProjectRateLimit)
			require.EqualValues(t, *keyInfo.ProjectRateLimit, rate)
			require.NotNil(t, keyInfo.ProjectBurstLimit)
			require.EqualValues(t, *keyInfo.ProjectBurstLimit, burstProject)

			// verify project limits are hit instead of global limits
			for i := int64(0); i < burstProject; i++ {
				_, err = endpoint.ListBuckets(ctx, listReq)
				require.NoError(t, err)
			}
			_, err = endpoint.ListBuckets(ctx, listReq)
			require.Error(t, err)
			require.True(t, errs2.IsRPC(err, rpcstatus.ResourceExhausted))

			// set op-specific rate limit values
			err = projects.UpdateLimitsGeneric(ctx, proj.ID, []console.Limit{
				{Kind: console.RateLimitHead, Value: &rate},
				{Kind: console.BurstLimitHead, Value: &burstHead},
				{Kind: console.RateLimitGet, Value: &rate},
				{Kind: console.BurstLimitGet, Value: &burstGet},
				{Kind: console.RateLimitPut, Value: &rate},
				{Kind: console.BurstLimitPut, Value: &burstPut},
				{Kind: console.RateLimitList, Value: &rate},
				{Kind: console.BurstLimitList, Value: &burstList},
				{Kind: console.RateLimitDelete, Value: &rate},
				{Kind: console.BurstLimitDelete, Value: &burstDelete},
			})
			require.NoError(t, err)
			keyInfo, err = apiKeys.GetByHead(ctx, ownerAPIKey.Head())
			require.NoError(t, err)
			require.NotNil(t, keyInfo.ProjectRateLimitHead)
			require.EqualValues(t, *keyInfo.ProjectRateLimitHead, rate)
			require.NotNil(t, keyInfo.ProjectBurstLimitHead)
			require.EqualValues(t, *keyInfo.ProjectBurstLimitHead, burstHead)
			require.NotNil(t, keyInfo.ProjectRateLimitGet)
			require.EqualValues(t, *keyInfo.ProjectRateLimitGet, rate)
			require.NotNil(t, keyInfo.ProjectBurstLimitGet)
			require.EqualValues(t, *keyInfo.ProjectBurstLimitGet, burstGet)
			require.NotNil(t, keyInfo.ProjectRateLimitPut)
			require.EqualValues(t, *keyInfo.ProjectRateLimitPut, rate)
			require.NotNil(t, keyInfo.ProjectBurstLimitPut)
			require.EqualValues(t, *keyInfo.ProjectBurstLimitPut, burstPut)
			require.NotNil(t, keyInfo.ProjectRateLimitList)
			require.EqualValues(t, *keyInfo.ProjectRateLimitList, rate)
			require.NotNil(t, keyInfo.ProjectBurstLimitList)
			require.EqualValues(t, *keyInfo.ProjectBurstLimitList, burstList)
			require.NotNil(t, keyInfo.ProjectRateLimitDelete)
			require.EqualValues(t, *keyInfo.ProjectRateLimitDelete, rate)
			require.NotNil(t, keyInfo.ProjectBurstLimitDelete)
			require.EqualValues(t, *keyInfo.ProjectBurstLimitDelete, burstDelete)

			// verify put rate limit
			for i := int64(0); i < burstPut; i++ {
				bucketName := fmt.Sprintf("bucket%d", i)
				_, err = endpoint.CreateBucket(ctx, &pb.CreateBucketRequest{
					Header: &pb.RequestHeader{ApiKey: ownerAPIKey.SerializeRaw()},
					Name:   []byte(bucketName),
				})
				require.NoError(t, err)
			}
			_, err = endpoint.CreateBucket(ctx, &pb.CreateBucketRequest{
				Header: &pb.RequestHeader{ApiKey: ownerAPIKey.SerializeRaw()},
				Name:   []byte("failbucket"),
			})
			require.Error(t, err)
			require.True(t, errs2.IsRPC(err, rpcstatus.ResourceExhausted))

			// verify head rate limit
			for i := int64(0); i < burstHead; i++ {
				bucketName := fmt.Sprintf("bucket%d", i)
				_, err = endpoint.GetBucket(ctx, &pb.GetBucketRequest{
					Header: &pb.RequestHeader{ApiKey: ownerAPIKey.SerializeRaw()},
					Name:   []byte(bucketName),
				})
				require.NoError(t, err)
			}
			_, err = endpoint.GetBucket(ctx, &pb.GetBucketRequest{
				Header: &pb.RequestHeader{ApiKey: ownerAPIKey.SerializeRaw()},
				Name:   []byte("bucket0"),
			})
			require.Error(t, err)
			require.True(t, errs2.IsRPC(err, rpcstatus.ResourceExhausted))

			// verify get rate limit
			dlRequest := &pb.ObjectDownloadRequest{
				Header:             &pb.RequestHeader{ApiKey: ownerAPIKey.SerializeRaw()},
				Bucket:             []byte("badbucket"),
				EncryptedObjectKey: []byte("badobjectkey"),
			}
			for i := int64(0); i < burstGet; i++ {
				// no objects are uploaded, so we always expect "not found"
				// since we are testing the rate limiter only, we simply need to verify we get "resource exhausted" after all the allowed requests
				_, err = endpoint.DownloadObject(peerctx, dlRequest)
				require.Error(t, err)
				require.True(t, errs2.IsRPC(err, rpcstatus.NotFound))
			}
			_, err = endpoint.DownloadObject(peerctx, dlRequest)
			require.Error(t, err)
			require.True(t, errs2.IsRPC(err, rpcstatus.ResourceExhausted))

			// verify list rate limit
			for i := int64(0); i < burstList; i++ {
				_, err = endpoint.ListBuckets(ctx, listReq)
				require.NoError(t, err)
			}
			_, err = endpoint.ListBuckets(ctx, listReq)
			require.Error(t, err)
			require.True(t, errs2.IsRPC(err, rpcstatus.ResourceExhausted))

			// verify delete rate limit
			for i := int64(0); i < burstDelete; i++ {
				bucketName := fmt.Sprintf("bucket%d", i)
				_, err = endpoint.DeleteBucket(ctx, &pb.DeleteBucketRequest{
					Header: &pb.RequestHeader{ApiKey: ownerAPIKey.SerializeRaw()},
					Name:   []byte(bucketName),
				})
				require.NoError(t, err)
			}
			_, err = endpoint.DeleteBucket(ctx, &pb.DeleteBucketRequest{
				Header: &pb.RequestHeader{ApiKey: ownerAPIKey.SerializeRaw()},
				Name:   []byte(fmt.Sprintf("bucket%d", burstPut-1)),
			})
			require.Error(t, err)
			require.True(t, errs2.IsRPC(err, rpcstatus.ResourceExhausted))
		})
}

func TestEndpoint_checkUserStatus(t *testing.T) {
	testplanet.Run(t, testplanet.Config{
		SatelliteCount: 1, StorageNodeCount: 1, UplinkCount: 2,
		Reconfigure: testplanet.Reconfigure{
			Satellite: func(log *zap.Logger, index int, config *satellite.Config) {
				config.Metainfo.UserInfoValidation.Enabled = true
				config.Metainfo.UserInfoValidation.CacheCapacity = 0
			},
		},
	},
		func(t *testing.T, ctx *testcontext.Context, planet *testplanet.Planet) {
			sat := planet.Satellites[0]
			endpoint := sat.Metainfo.Endpoint
			users := sat.API.DB.Console().Users()

			user, err := users.GetByEmailAndTenant(ctx, planet.Uplinks[0].User[sat.ID()].Email, nil)
			require.NoError(t, err)
			require.Equal(t, console.Active, user.Status)

			ownerAPIKey := planet.Uplinks[0].APIKey[sat.ID()]

			listReq := &pb.ListBucketsRequest{
				Header: &pb.RequestHeader{
					ApiKey: ownerAPIKey.SerializeRaw(),
				},
				Direction: buckets.DirectionForward,
			}
			_, err = endpoint.ListBuckets(ctx, listReq)
			require.NoError(t, err)

			// update user status to inactive
			inactive := console.Inactive
			err = users.Update(ctx, user.ID, console.UpdateUserRequest{Status: &inactive})
			require.NoError(t, err)

			// inactive user is denied access
			_, err = endpoint.ListBuckets(ctx, listReq)
			require.Error(t, err)
			require.True(t, errs2.IsRPC(err, rpcstatus.PermissionDenied))
		})
}

func TestEndpoint_ValidateAuthAny(t *testing.T) {
	ctx := testcontext.New(t)
	defer ctx.Cleanup()

	secret, err := macaroon.NewSecret()
	require.NoError(t, err)

	endpoint := metainfo.TestingNewAPIKeysEndpoint(zaptest.NewLogger(t), &mockAPIKeys{secret: secret})

	now := time.Now()

	var canList, canDelete bool
	path := testrand.Bytes(16)
	optionalPerms := []metainfo.VerifyPermission{
		{
			Action: macaroon.Action{
				Op:            macaroon.ActionList,
				Time:          now,
				EncryptedPath: path,
			},
			Optional:        true,
			ActionPermitted: &canList,
		}, {
			Action: macaroon.Action{
				Op:            macaroon.ActionDelete,
				Time:          now,
				EncryptedPath: path,
			},
			Optional:        true,
			ActionPermitted: &canDelete,
		},
	}

	perms := []metainfo.VerifyPermission{
		{
			Action: macaroon.Action{
				Op:            macaroon.ActionRead,
				Time:          now,
				EncryptedPath: path,
			},
		}, {
			Action: macaroon.Action{
				Op:            macaroon.ActionWrite,
				Time:          now,
				EncryptedPath: path,
			},
		},
	}
	perms = append(perms, optionalPerms...)

	// expect error if no required actions are provided
	key, err := macaroon.NewAPIKey(secret)
	require.NoError(t, err)

	_, err = endpoint.ValidateAuthAny(ctx, &pb.RequestHeader{ApiKey: key.SerializeRaw()}, console.RateLimit)
	require.Error(t, err)

	_, err = endpoint.ValidateAuthAny(ctx, &pb.RequestHeader{ApiKey: key.SerializeRaw()}, console.RateLimit, optionalPerms...)
	require.Error(t, err)
	require.False(t, canList)
	require.False(t, canDelete)

	// expect error if no required actions are permitted
	keyNone, err := key.Restrict(macaroon.Caveat{
		DisallowWrites: true,
		DisallowLists:  true,
		DisallowReads:  true,
	})
	require.NoError(t, err)

	_, err = endpoint.ValidateAuthAny(ctx, &pb.RequestHeader{ApiKey: keyNone.SerializeRaw()}, console.RateLimit, perms...)
	require.Error(t, err)
	require.False(t, canList)
	require.False(t, canDelete)

	keyOnlyOptional, err := key.Restrict(macaroon.Caveat{
		DisallowReads:  true,
		DisallowWrites: true,
	})
	require.NoError(t, err)

	_, err = endpoint.ValidateAuthAny(ctx, &pb.RequestHeader{ApiKey: keyOnlyOptional.SerializeRaw()}, console.RateLimit, perms...)
	require.Error(t, err)
	require.False(t, canList)
	require.False(t, canDelete)

	// expect success if any required action is permitted even if no optional action is permitted
	keyNoOptional, err := key.Restrict(macaroon.Caveat{
		DisallowLists:   true,
		DisallowDeletes: true,
	})
	require.NoError(t, err)

	_, err = endpoint.ValidateAuthAny(ctx, &pb.RequestHeader{ApiKey: keyNoOptional.SerializeRaw()}, console.RateLimit, perms...)
	require.NoError(t, err)
	require.False(t, canList)
	require.False(t, canDelete)

	// expect success if at least one required action is permitted
	keyPartial, err := key.Restrict(macaroon.Caveat{
		DisallowReads: true,
		DisallowLists: true,
	})
	require.NoError(t, err)

	_, err = endpoint.ValidateAuthAny(ctx, &pb.RequestHeader{ApiKey: keyPartial.SerializeRaw()}, console.RateLimit, perms...)
	require.NoError(t, err)
	require.False(t, canList)
	require.True(t, canDelete)
}
