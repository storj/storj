// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package metainfo_test

import (
	"errors"
	"fmt"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"go.uber.org/zap"

	"storj.io/common/errs2"
	"storj.io/common/macaroon"
	"storj.io/common/memory"
	"storj.io/common/pb"
	"storj.io/common/rpc/rpcstatus"
	"storj.io/common/rpc/rpctest"
	"storj.io/common/storj"
	"storj.io/common/testcontext"
	"storj.io/common/testrand"
	"storj.io/storj/private/testplanet"
	"storj.io/storj/satellite"
	"storj.io/storj/satellite/buckets"
	"storj.io/storj/satellite/console"
	"storj.io/storj/satellite/entitlements"
	"storj.io/storj/satellite/metabase"
	"storj.io/storj/satellite/nodeselection"
	"storj.io/uplink"
	"storj.io/uplink/private/metaclient"
)

func TestBucketExistenceCheck(t *testing.T) {
	testplanet.Run(t, testplanet.Config{
		SatelliteCount: 1, UplinkCount: 1,
	}, func(t *testing.T, ctx *testcontext.Context, planet *testplanet.Planet) {
		apiKey := planet.Uplinks[0].APIKey[planet.Satellites[0].ID()]

		metainfoClient, err := planet.Uplinks[0].DialMetainfo(ctx, planet.Satellites[0], apiKey)
		require.NoError(t, err)
		defer ctx.Check(metainfoClient.Close)

		// test object methods for bucket existence check
		_, err = metainfoClient.BeginObject(ctx, metaclient.BeginObjectParams{
			Bucket:             []byte("non-existing-bucket"),
			EncryptedObjectKey: []byte("encrypted-path"),
		})
		require.Error(t, err)
		require.True(t, errs2.IsRPC(err, rpcstatus.NotFound))
		require.Equal(t, buckets.ErrBucketNotFound.New("%s", "non-existing-bucket").Error(), errors.Unwrap(err).Error())

		_, _, err = metainfoClient.ListObjects(ctx, metaclient.ListObjectsParams{
			Bucket: []byte("non-existing-bucket"),
		})
		require.Error(t, err)
		require.True(t, errs2.IsRPC(err, rpcstatus.NotFound))
		require.Equal(t, buckets.ErrBucketNotFound.New("%s", "non-existing-bucket").Error(), errors.Unwrap(err).Error())
	})
}

func TestMaxOutBuckets(t *testing.T) {
	testplanet.Run(t, testplanet.Config{
		SatelliteCount: 1, UplinkCount: 1,
	}, func(t *testing.T, ctx *testcontext.Context, planet *testplanet.Planet) {
		limit := planet.Satellites[0].Config.Metainfo.ProjectLimits.MaxBuckets
		for i := 1; i <= limit; i++ {
			name := "test" + strconv.Itoa(i)
			err := planet.Uplinks[0].TestingCreateBucket(ctx, planet.Satellites[0], name)
			require.NoError(t, err)
		}
		err := planet.Uplinks[0].CreateBucket(ctx, planet.Satellites[0], fmt.Sprintf("test%d", limit+1))
		require.Error(t, err)
		require.Contains(t, err.Error(), fmt.Sprintf("number of allocated buckets (%d) exceeded", limit))
	})
}

func TestBucketNameValidation(t *testing.T) {
	testplanet.Run(t, testplanet.Config{
		SatelliteCount: 1, UplinkCount: 1,
	}, func(t *testing.T, ctx *testcontext.Context, planet *testplanet.Planet) {
		apiKey := planet.Uplinks[0].APIKey[planet.Satellites[0].ID()]

		metainfoClient, err := planet.Uplinks[0].DialMetainfo(ctx, planet.Satellites[0], apiKey)
		require.NoError(t, err)
		defer ctx.Check(metainfoClient.Close)

		validNames := []string{
			"tes", "testbucket",
			"test-bucket", "testbucket9",
			"9testbucket", "a.b",
			"test.bucket", "test-one.bucket-one",
			"test.bucket.one",
			"testbucket-63-0123456789012345678901234567890123456789012345abc",
		}
		for _, name := range validNames {
			_, err = metainfoClient.CreateBucket(ctx, metaclient.CreateBucketParams{
				Name: []byte(name),
			})
			require.NoError(t, err, "bucket name: %v", name)

			_, err = metainfoClient.BeginObject(ctx, metaclient.BeginObjectParams{
				Bucket:             []byte(name),
				EncryptedObjectKey: []byte("123"),
				ExpiresAt:          time.Now().Add(16 * 24 * time.Hour),
				EncryptionParameters: storj.EncryptionParameters{
					CipherSuite: storj.EncAESGCM,
					BlockSize:   256,
				},
			})
			require.NoError(t, err, "bucket name: %v", name)
		}

		invalidNames := []string{
			"", "t", "te", "-testbucket",
			"testbucket-", "-testbucket-",
			"a.b.", "test.bucket-.one",
			"test.-bucket.one", "1.2.3.4",
			"192.168.1.234", "testBUCKET",
			"test/bucket",
			"testbucket-64-0123456789012345678901234567890123456789012345abcd",
			"test\\", "test%",
		}
		for _, name := range invalidNames {

			_, err = metainfoClient.CreateBucket(ctx, metaclient.CreateBucketParams{
				Name: []byte(name),
			})
			require.Error(t, err, "bucket name: %v", name)
			require.True(t, errs2.IsRPC(err, rpcstatus.InvalidArgument))
		}

		invalidNames = []string{
			"", "t", "te",
			"testbucket-64-0123456789012345678901234567890123456789012345abcd",
		}
		for _, name := range invalidNames {
			// BeginObject validates only bucket name length
			_, err = metainfoClient.BeginObject(ctx, metaclient.BeginObjectParams{
				Bucket:             []byte(name),
				EncryptedObjectKey: []byte("123"),
			})
			require.Error(t, err, "bucket name: %v", name)
			require.True(t, errs2.IsRPC(err, rpcstatus.InvalidArgument))
		}
	})
}

func TestBucketEmptinessBeforeDelete(t *testing.T) {
	testplanet.Run(t, testplanet.Config{
		SatelliteCount: 1, UplinkCount: 1,
	}, func(t *testing.T, ctx *testcontext.Context, planet *testplanet.Planet) {
		for i := 0; i < 5; i++ {
			err := planet.Uplinks[0].Upload(ctx, planet.Satellites[0], "test-bucket", "object-key"+strconv.Itoa(i), testrand.Bytes(memory.KiB))
			require.NoError(t, err)
		}

		for i := 0; i < 5; i++ {
			err := planet.Uplinks[0].DeleteBucket(ctx, planet.Satellites[0], "test-bucket")
			require.Error(t, err)
			require.True(t, errors.Is(err, uplink.ErrBucketNotEmpty))

			err = planet.Uplinks[0].DeleteObject(ctx, planet.Satellites[0], "test-bucket", "object-key"+strconv.Itoa(i))
			require.NoError(t, err)
		}

		err := planet.Uplinks[0].DeleteBucket(ctx, planet.Satellites[0], "test-bucket")
		require.NoError(t, err)
	})
}

func TestDeleteBucket(t *testing.T) {
	testplanet.Run(t, testplanet.Config{
		Reconfigure: testplanet.Reconfigure{
			Satellite: testplanet.Combine(
				testplanet.ReconfigureRS(2, 2, 4, 4),
				testplanet.MaxSegmentSize(13*memory.KiB),
				func(log *zap.Logger, index int, config *satellite.Config) {
					config.Metainfo.ObjectLockEnabled = true
					config.Metainfo.UseBucketLevelObjectVersioning = true
				},
			),
			Uplink: func(log *zap.Logger, index int, config *testplanet.UplinkConfig) {
				config.APIKeyVersion = macaroon.APIKeyVersionObjectLock
			},
		},
		SatelliteCount: 1, StorageNodeCount: 4, UplinkCount: 1,
	}, func(t *testing.T, ctx *testcontext.Context, planet *testplanet.Planet) {
		ownerAPIKey := planet.Uplinks[0].APIKey[planet.Satellites[0].ID()]
		sat := planet.Satellites[0]
		uplnk := planet.Uplinks[0]
		project := uplnk.Projects[0]
		endpoint := sat.API.Metainfo.Endpoint

		expectedObjects := map[string][]byte{
			"single-segment-object":        testrand.Bytes(10 * memory.KiB),
			"multi-segment-object":         testrand.Bytes(50 * memory.KiB),
			"remote-segment-inline-object": testrand.Bytes(1 * memory.KiB),
		}

		uploadObjects := func(t *testing.T, bucketName metabase.BucketName) {
			require.NoError(t, uplnk.CreateBucket(ctx, sat, bucketName.String()))
			for name, bytes := range expectedObjects {
				require.NoError(t, uplnk.Upload(ctx, sat, bucketName.String(), name, bytes))
			}
		}

		requireBucketDeleted := func(t *testing.T, bucketName metabase.BucketName) {
			_, err := sat.DB.Buckets().GetBucket(ctx, []byte(bucketName), project.ID)
			require.True(t, buckets.ErrBucketNotFound.Has(err))

			objects, err := sat.API.Metainfo.Metabase.ListObjects(ctx, metabase.ListObjects{
				ProjectID:  project.ID,
				BucketName: bucketName,
				Limit:      1,
			})
			require.NoError(t, err)
			require.Empty(t, objects.Objects)
		}

		requireBucketNotDeleted := func(t *testing.T, bucketName metabase.BucketName) {
			_, err := sat.DB.Buckets().GetBucket(ctx, []byte(bucketName), project.ID)
			require.NoError(t, err)

			objects, err := sat.API.Metainfo.Metabase.ListObjects(ctx, metabase.ListObjects{
				ProjectID:  project.ID,
				BucketName: bucketName,
				Limit:      len(expectedObjects),
			})
			require.NoError(t, err)
			require.Len(t, objects.Objects, len(expectedObjects))
		}

		t.Run("Delete bucket as owner", func(t *testing.T) {
			bucketName := metabase.BucketName(testrand.BucketName())
			uploadObjects(t, bucketName)

			delResp, err := endpoint.DeleteBucket(ctx, &pb.BucketDeleteRequest{
				Header: &pb.RequestHeader{
					ApiKey: ownerAPIKey.SerializeRaw(),
				},
				Name:      []byte(bucketName),
				DeleteAll: true,
			})
			require.NoError(t, err)
			require.NotNil(t, delResp)
			require.EqualValues(t, len(expectedObjects), delResp.DeletedObjectsCount)

			requireBucketDeleted(t, bucketName)
		})

		t.Run("Delete bucket with Object Lock enabled as owner", func(t *testing.T) {
			bucketName := metabase.BucketName(testrand.BucketName())
			_, err := sat.DB.Buckets().CreateBucket(ctx, buckets.Bucket{
				ProjectID: project.ID,
				Name:      bucketName.String(),
				ObjectLock: buckets.ObjectLockSettings{
					Enabled: true,
				},
			})
			require.NoError(t, err)

			uploadObjects(t, bucketName)

			delResp, err := endpoint.DeleteBucket(ctx, &pb.BucketDeleteRequest{
				Header: &pb.RequestHeader{
					ApiKey: ownerAPIKey.SerializeRaw(),
				},
				Name:      []byte(bucketName),
				DeleteAll: true,
			})
			rpctest.AssertCode(t, err, rpcstatus.PermissionDenied)
			require.Nil(t, delResp)

			requireBucketNotDeleted(t, bucketName)

			objs, err := sat.Admin.MetabaseDB.ListObjects(ctx, metabase.ListObjects{
				ProjectID:  project.ID,
				BucketName: bucketName,
				Limit:      len(expectedObjects),
			})
			require.NoError(t, err)
			for _, obj := range objs.Objects {
				_, err := sat.Admin.MetabaseDB.DeleteObjectLastCommitted(ctx, metabase.DeleteObjectLastCommitted{
					ObjectLocation: metabase.ObjectLocation{
						ProjectID:  project.ID,
						BucketName: bucketName,
						ObjectKey:  obj.ObjectKey,
					},
				})
				require.NoError(t, err)
			}

			_, err = endpoint.DeleteBucket(ctx, &pb.BucketDeleteRequest{
				Header: &pb.RequestHeader{
					ApiKey: ownerAPIKey.SerializeRaw(),
				},
				Name: []byte(bucketName),
			})
			require.NoError(t, err)

			requireBucketDeleted(t, bucketName)
		})

		t.Run("Delete bucket with Object Lock enabled as owner with BypassGovernanceRetention", func(t *testing.T) {
			bucketName := metabase.BucketName(testrand.BucketName())
			_, err := sat.DB.Buckets().CreateBucket(ctx, buckets.Bucket{
				ProjectID: project.ID,
				Name:      bucketName.String(),
				ObjectLock: buckets.ObjectLockSettings{
					Enabled: true,
				},
			})
			require.NoError(t, err)

			uploadObjects(t, bucketName)

			// User should not be able to delete without BypassGovernanceRetention
			// permission.
			noBypass, err := ownerAPIKey.Restrict(macaroon.Caveat{
				DisallowBypassGovernanceRetention: true,
			})
			require.NoError(t, err)
			delResp, err := endpoint.DeleteBucket(ctx, &pb.BucketDeleteRequest{
				Header: &pb.RequestHeader{
					ApiKey: noBypass.SerializeRaw(),
				},
				Name:                      []byte(bucketName),
				DeleteAll:                 true,
				BypassGovernanceRetention: true,
			})
			rpctest.AssertCode(t, err, rpcstatus.PermissionDenied)
			require.Nil(t, delResp)

			// When user has the specific permission, then it's fine.
			delResp, err = endpoint.DeleteBucket(ctx, &pb.BucketDeleteRequest{
				Header: &pb.RequestHeader{
					ApiKey: ownerAPIKey.SerializeRaw(),
				},
				Name:                      []byte(bucketName),
				DeleteAll:                 true,
				BypassGovernanceRetention: true,
			})
			require.NoError(t, err)
			require.EqualValues(t, 3, delResp.DeletedObjectsCount)

			requireBucketDeleted(t, bucketName)
		})

		t.Run("Delete bucket as member", func(t *testing.T) {
			bucketName := metabase.BucketName(testrand.BucketName())
			uploadObjects(t, bucketName)

			member, err := sat.AddUser(ctx, console.CreateUser{
				FullName: "Member User",
				Email:    "member@example.com",
			}, 1)
			require.NoError(t, err)
			require.NotNil(t, member)

			memberCtx, err := sat.UserContext(ctx, member.ID)
			require.NoError(t, err)

			_, err = sat.DB.Console().ProjectMembers().Insert(ctx, member.ID, project.ID, console.RoleMember)
			require.NoError(t, err)

			memberKeyInfo, memberKey, err := sat.API.Console.Service.CreateAPIKey(memberCtx, project.ID, "member key", macaroon.APIKeyVersionMin)
			require.NoError(t, err)
			require.NotNil(t, memberKey)
			require.NotNil(t, memberKeyInfo)

			delResp, err := endpoint.DeleteBucket(ctx, &pb.BucketDeleteRequest{
				Header: &pb.RequestHeader{
					ApiKey: memberKey.SerializeRaw(),
				},
				Name:      []byte(bucketName),
				DeleteAll: true,
			})
			rpctest.AssertCode(t, err, rpcstatus.PermissionDenied)
			require.Nil(t, delResp)

			requireBucketNotDeleted(t, bucketName)
		})

		t.Run("Delete bucket as admin", func(t *testing.T) {
			bucketName := metabase.BucketName(testrand.BucketName())
			uploadObjects(t, bucketName)

			admin, err := sat.AddUser(ctx, console.CreateUser{
				FullName: "Admin User",
				Email:    "admin@example.com",
			}, 1)
			require.NoError(t, err)
			require.NotNil(t, admin)

			adminCtx, err := sat.UserContext(ctx, admin.ID)
			require.NoError(t, err)

			_, err = sat.DB.Console().ProjectMembers().Insert(ctx, admin.ID, project.ID, console.RoleAdmin)
			require.NoError(t, err)

			adminKeyInfo, adminKey, err := sat.API.Console.Service.CreateAPIKey(adminCtx, project.ID, "admin key", macaroon.APIKeyVersionMin)
			require.NoError(t, err)
			require.NotNil(t, adminKey)
			require.NotNil(t, adminKeyInfo)

			delResp, err := endpoint.DeleteBucket(ctx, &pb.BucketDeleteRequest{
				Header: &pb.RequestHeader{
					ApiKey: adminKey.SerializeRaw(),
				},
				Name:      []byte(bucketName),
				DeleteAll: true,
			})
			require.NoError(t, err)
			require.NotNil(t, delResp)
			require.EqualValues(t, len(expectedObjects), delResp.DeletedObjectsCount)

			requireBucketDeleted(t, bucketName)
		})

		t.Run("Ensure attribution after bucket delete", func(t *testing.T) {
			bucketName := metabase.BucketName(testrand.BucketName())
			uploadObjects(t, bucketName)

			nameBytes := []byte(bucketName)

			attrDB := sat.DB.Attribution()

			attr, err := attrDB.Get(ctx, project.ID, nameBytes)
			require.NoError(t, err)
			require.NotNil(t, attr)

			require.NoError(t, attrDB.TestDelete(ctx, project.ID, nameBytes))

			delResp, err := endpoint.DeleteBucket(ctx, &pb.BucketDeleteRequest{
				Header: &pb.RequestHeader{
					ApiKey: ownerAPIKey.SerializeRaw(),
				},
				Name:      []byte(bucketName),
				DeleteAll: true,
			})
			require.NoError(t, err)
			require.NotNil(t, delResp)
			require.EqualValues(t, len(expectedObjects), delResp.DeletedObjectsCount)

			requireBucketDeleted(t, bucketName)

			attr, err = attrDB.Get(ctx, project.ID, nameBytes)
			require.NoError(t, err)
			require.NotNil(t, attr)
			require.Equal(t, bucketName.String(), string(attr.BucketName))
			require.Equal(t, project.ID, attr.ProjectID)
		})
	})
}

func TestListBucketsWithAttribution(t *testing.T) {
	testplanet.Run(t, testplanet.Config{
		SatelliteCount: 1, StorageNodeCount: 4, UplinkCount: 1,
	}, func(t *testing.T, ctx *testcontext.Context, planet *testplanet.Planet) {
		satellite := planet.Satellites[0]
		apiKey := planet.Uplinks[0].APIKey[planet.Satellites[0].ID()]

		type testCase struct {
			UserAgent string
			Bucket    string
		}

		var testCases = []testCase{
			{
				Bucket: "bucket-without-user-agent",
			},
			{
				UserAgent: "storj",
				Bucket:    "bucket-with-user-agent",
			},
		}

		bucketExists := func(tc testCase, buckets *pb.BucketListResponse) bool {
			for _, bucket := range buckets.Items {
				if string(bucket.Name) == tc.Bucket {
					require.EqualValues(t, tc.UserAgent, string(bucket.UserAgent))
					return true
				}
			}
			t.Fatalf("bucket was not found in results:%s", tc.Bucket)
			return false
		}

		for _, tc := range testCases {
			config := uplink.Config{
				UserAgent: tc.UserAgent,
			}

			project, err := config.OpenProject(ctx, planet.Uplinks[0].Access[satellite.ID()])
			require.NoError(t, err)

			_, err = project.CreateBucket(ctx, tc.Bucket)
			require.NoError(t, err)

			buckets, err := satellite.Metainfo.Endpoint.ListBuckets(ctx, &pb.BucketListRequest{
				Header: &pb.RequestHeader{
					ApiKey: apiKey.SerializeRaw(),
				},
				Direction: buckets.DirectionForward,
			})
			require.NoError(t, err)
			require.True(t, bucketExists(tc, buckets))
		}
	})
}

func TestBucketCreationWithDefaultPlacement(t *testing.T) {
	testplanet.Run(t, testplanet.Config{
		SatelliteCount: 1, UplinkCount: 1,
	}, func(t *testing.T, ctx *testcontext.Context, planet *testplanet.Planet) {
		projectID := planet.Uplinks[0].Projects[0].ID

		// change the default_placement of the project
		project, err := planet.Satellites[0].API.DB.Console().Projects().Get(ctx, projectID)
		project.DefaultPlacement = storj.EU
		require.NoError(t, err)
		err = planet.Satellites[0].API.DB.Console().Projects().Update(ctx, project)
		require.NoError(t, err)

		// create a new bucket
		up, err := planet.Uplinks[0].GetProject(ctx, planet.Satellites[0])
		require.NoError(t, err)

		_, err = up.CreateBucket(ctx, "eu1")
		require.NoError(t, err)

		// check if placement is set
		placement, err := planet.Satellites[0].API.DB.Buckets().GetBucketPlacement(ctx, []byte("eu1"), projectID)
		require.NoError(t, err)
		require.Equal(t, storj.EU, placement)

	})
}

func TestBucketCreationSelfServePlacement(t *testing.T) {
	var (
		placement       = storj.PlacementConstraint(40)
		placementDetail = console.PlacementDetail{
			ID:     40,
			IdName: "Poland",
		}
	)
	testplanet.Run(t, testplanet.Config{
		SatelliteCount: 1, UplinkCount: 1,
		Reconfigure: testplanet.Reconfigure{
			Satellite: func(log *zap.Logger, index int, config *satellite.Config) {
				config.Placement = nodeselection.ConfigurablePlacementRule{
					PlacementRules: `40:annotation("location", "Poland");50:annotation("location", "US");60:annotation("location", "AP")`,
				}
				config.Console.Placement.SelfServeEnabled = true
				config.Console.Placement.SelfServeDetails.SetMap(map[storj.PlacementConstraint]console.PlacementDetail{
					placement: placementDetail,
					60: {
						ID:          60,
						IdName:      "AP",
						WaitlistURL: "some-url",
					},
				})
			},
		},
	}, func(t *testing.T, ctx *testcontext.Context, planet *testplanet.Planet) {
		sat := planet.Satellites[0]
		projectID := planet.Uplinks[0].Projects[0].ID
		apiKey := planet.Uplinks[0].APIKey[planet.Satellites[0].ID()]
		bucket1 := "bucket1"

		// change the default_placement of the project
		err := planet.Satellites[0].API.DB.Console().Projects().UpdateDefaultPlacement(ctx, projectID, storj.EU)
		require.NoError(t, err)

		_, err = sat.API.Metainfo.Endpoint.CreateBucket(ctx, &pb.CreateBucketRequest{
			Header: &pb.RequestHeader{
				ApiKey: apiKey.SerializeRaw(),
			},
			Name:      []byte(bucket1),
			Placement: []byte("Poland"),
		})
		// cannot create bucket with custom placement if there is project default.
		require.True(t, errs2.IsRPC(err, rpcstatus.PlacementConflictingValues))

		// create a new bucket with default placement
		_, err = sat.API.Metainfo.Endpoint.CreateBucket(ctx, &pb.CreateBucketRequest{
			Header: &pb.RequestHeader{
				ApiKey: apiKey.SerializeRaw(),
			},
			Name: []byte(bucket1),
		})
		require.NoError(t, err)

		// check if placement is set to project default
		placement, err := planet.Satellites[0].API.DB.Buckets().GetBucketPlacement(ctx, []byte(bucket1), projectID)
		require.NoError(t, err)
		require.Equal(t, storj.EU, placement)

		// delete the bucket
		err = planet.Satellites[0].API.DB.Buckets().DeleteBucket(ctx, []byte(bucket1), projectID)
		require.NoError(t, err)

		// change the default_placement of the project
		err = planet.Satellites[0].API.DB.Console().Projects().UpdateDefaultPlacement(ctx, projectID, storj.DefaultPlacement)
		require.NoError(t, err)

		// recreate bucket with different placement fails because attribution already exists with original placement
		_, err = sat.API.Metainfo.Endpoint.CreateBucket(ctx, &pb.CreateBucketRequest{
			Header: &pb.RequestHeader{
				ApiKey: apiKey.SerializeRaw(),
			},
			Name:      []byte(bucket1),
			Placement: []byte("Poland"),
		})
		require.Error(t, err)
		require.Contains(t, err.Error(), "already attributed to a different placement constraint")

		// create brand new bucket with placement
		bucket2 := "bucket2"
		_, err = sat.API.Metainfo.Endpoint.CreateBucket(ctx, &pb.CreateBucketRequest{
			Header: &pb.RequestHeader{
				ApiKey: apiKey.SerializeRaw(),
			},
			Name:      []byte(bucket2),
			Placement: []byte("Poland"),
		})
		require.NoError(t, err)

		placement, err = planet.Satellites[0].API.DB.Buckets().GetBucketPlacement(ctx, []byte(bucket2), projectID)
		require.NoError(t, err)
		require.Equal(t, storj.PlacementConstraint(40), placement)

		// new bucket with invalid placement returns error
		bucket3 := []byte("bucket3")
		_, err = sat.API.Metainfo.Endpoint.CreateBucket(ctx, &pb.CreateBucketRequest{
			Header: &pb.RequestHeader{
				ApiKey: apiKey.SerializeRaw(),
			},
			Name:      bucket3,
			Placement: []byte("EU"), // invalid placement
		})
		require.True(t, errs2.IsRPC(err, rpcstatus.PlacementInvalidValue))
		require.Contains(t, "invalid placement value", err.Error())

		// new bucket with placement not in self-serve details returns error
		_, err = sat.API.Metainfo.Endpoint.CreateBucket(ctx, &pb.CreateBucketRequest{
			Header: &pb.RequestHeader{
				ApiKey: apiKey.SerializeRaw(),
			},
			Name:      bucket3,
			Placement: []byte("US"), // placement not in self-serve details
		})
		require.True(t, errs2.IsRPC(err, rpcstatus.PlacementInvalidValue))
		require.Contains(t, "placement not allowed", err.Error())

		// new bucket with placement with a waitlist returns error
		_, err = sat.API.Metainfo.Endpoint.CreateBucket(ctx, &pb.CreateBucketRequest{
			Header: &pb.RequestHeader{
				ApiKey: apiKey.SerializeRaw(),
			},
			Name:      bucket3,
			Placement: []byte("AP"), // placement with waitlist
		})
		require.True(t, errs2.IsRPC(err, rpcstatus.PlacementInvalidValue))
		require.Contains(t, "placement not allowed", err.Error())

		// disable self-serve placement
		sat.API.Metainfo.Endpoint.TestSelfServePlacementEnabled(false)

		// Passing invalid placement should not fail if self-serve placement is disabled.
		// This is for backward compatibility with integration tests that'll pass placements
		// regardless of self-serve placement being enabled or not.
		_, err = sat.API.Metainfo.Endpoint.CreateBucket(ctx, &pb.CreateBucketRequest{
			Header: &pb.RequestHeader{
				ApiKey: apiKey.SerializeRaw(),
			},
			Name:      bucket3,
			Placement: []byte("EU"), // invalid placement
		})
		require.NoError(t, err)

		// placement should be set to default even though a placement was passed
		// because self-serve placement is disabled.
		placement, err = planet.Satellites[0].API.DB.Buckets().GetBucketPlacement(ctx, bucket3, projectID)
		require.NoError(t, err)
		require.Equal(t, storj.DefaultPlacement, placement)
	})
}

func TestBucketCreation_EntitlementsPlacement(t *testing.T) {
	var (
		plPoland         = storj.PlacementConstraint(40)
		plUkraine        = storj.PlacementConstraint(60)
		selfServeDetails = map[storj.PlacementConstraint]console.PlacementDetail{
			plPoland:  {ID: 40, IdName: "poland"},
			plUkraine: {ID: 60, IdName: "ukraine", WaitlistURL: "waitlist"},
		}
		allowedPlacements = []storj.PlacementConstraint{plPoland, plUkraine}
	)

	testplanet.Run(t, testplanet.Config{
		SatelliteCount: 1,
		Reconfigure: testplanet.Reconfigure{
			Satellite: func(log *zap.Logger, _ int, config *satellite.Config) {
				config.Placement = nodeselection.ConfigurablePlacementRule{
					PlacementRules: `40:annotation("location","poland");60:annotation("location","ukraine")`,
				}
				config.Console.Placement.SelfServeEnabled = true
				config.Console.Placement.SelfServeDetails.SetMap(selfServeDetails)
				config.Console.Placement.AllowedPlacementIdsForNewProjects = allowedPlacements
				config.Entitlements.Enabled = true
			},
		},
	}, func(t *testing.T, ctx *testcontext.Context, planet *testplanet.Planet) {
		sat := planet.Satellites[0]
		endpoint := sat.API.Metainfo.Endpoint

		user, err := sat.AddUser(ctx, console.CreateUser{
			FullName: "Ent User",
			Email:    "test@test.test",
		}, 1)
		require.NoError(t, err)

		userCtx, err := sat.UserContext(ctx, user.ID)
		require.NoError(t, err)

		project, err := sat.API.Console.Service.CreateProject(userCtx, console.UpsertProjectInfo{Name: "test project"})
		require.NoError(t, err)
		require.Contains(t, sat.Config.Console.Placement.AllowedPlacementIdsForNewProjects, project.DefaultPlacement)

		_, apiKey, err := sat.API.Console.Service.CreateAPIKey(userCtx, project.ID, "ent-key", macaroon.APIKeyVersionMin)
		require.NoError(t, err)

		mkReq := func(name, placement string) *pb.CreateBucketRequest {
			var p []byte
			if placement != "" {
				p = []byte(placement)
			}
			return &pb.CreateBucketRequest{
				Header:    &pb.RequestHeader{ApiKey: apiKey.SerializeRaw()},
				Name:      []byte(name),
				Placement: p,
			}
		}

		tests := []struct {
			name                  string
			feats                 *entitlements.ProjectFeatures // nil => delete row (NotFound)
			placementName         string
			want                  rpcstatus.StatusCode
			expectBucketPlacement *storj.PlacementConstraint
		}{
			{
				name: "no placement (default) → allowed",
				feats: &entitlements.ProjectFeatures{
					NewBucketPlacements: allowedPlacements,
				},
				placementName:         "",
				want:                  rpcstatus.OK,
				expectBucketPlacement: &project.DefaultPlacement,
			},
			{
				name: "explicit allowlist includes placement → allowed",
				feats: &entitlements.ProjectFeatures{
					NewBucketPlacements: []storj.PlacementConstraint{plPoland},
				},
				placementName:         "poland",
				want:                  rpcstatus.OK,
				expectBucketPlacement: &plPoland,
			},
			{
				name: "explicit allowlist excludes placement (non-empty) → denied",
				feats: &entitlements.ProjectFeatures{
					// Non-empty list that does NOT include poland (40).
					NewBucketPlacements: []storj.PlacementConstraint{storj.PlacementConstraint(999)},
				},
				placementName: "poland",
				want:          rpcstatus.PlacementInvalidValue,
			},
			{
				name: "explicit allowlist includes waitlisted placement → denied by self-serve",
				feats: &entitlements.ProjectFeatures{
					NewBucketPlacements: []storj.PlacementConstraint{plUkraine},
				},
				placementName: "ukraine",
				want:          rpcstatus.PlacementInvalidValue,
			},
			{
				name:                  "no entitlements row (NotFound) → fallback to global allowlist",
				feats:                 nil,
				placementName:         "poland",
				want:                  rpcstatus.OK,
				expectBucketPlacement: &plPoland,
			},
		}

		for i, tc := range tests {
			t.Run(tc.name, func(t *testing.T) {
				if tc.feats == nil {
					err := sat.API.Entitlements.Service.Projects().DeleteByPublicID(ctx, project.PublicID)
					require.NoError(t, err)
				} else {
					err := sat.API.Entitlements.Service.Projects().SetNewBucketPlacementsByPublicID(ctx, project.PublicID, tc.feats.NewBucketPlacements)
					require.NoError(t, err)
				}

				bucketName := fmt.Sprintf("ent-bucket-%s-%d", tc.placementName, i)

				_, err := endpoint.CreateBucket(ctx, mkReq(bucketName, tc.placementName))
				switch tc.want {
				case rpcstatus.OK:
					require.NoError(t, err)
				default:
					require.True(t, errs2.IsRPC(err, tc.want))
				}

				if tc.expectBucketPlacement != nil {
					pl, err := sat.API.DB.Buckets().GetBucketPlacement(ctx, []byte(bucketName), project.ID)
					require.NoError(t, err)
					require.Equal(t, *tc.expectBucketPlacement, pl)
				}
			})
		}
	})
}

func TestCreateBucketOnDisabledProject(t *testing.T) {
	testplanet.Run(t, testplanet.Config{
		SatelliteCount: 1, UplinkCount: 1,
	}, func(t *testing.T, ctx *testcontext.Context, planet *testplanet.Planet) {
		sat := planet.Satellites[0]
		service := sat.API.Console.Service
		project := planet.Uplinks[0].Projects[0]

		userCtx, err := sat.UserContext(ctx, project.Owner.ID)
		require.NoError(t, err)

		apiKeyInfo, apiKey, err := service.CreateAPIKey(userCtx, project.ID, "test key", macaroon.APIKeyVersionMin)
		require.NoError(t, err)
		require.NotNil(t, apiKey)
		require.False(t, apiKeyInfo.CreatedBy.IsZero())

		err = sat.API.DB.Console().Projects().UpdateStatus(ctx, project.ID, console.ProjectDisabled)
		require.NoError(t, err)

		bucketName := []byte("bucket")
		_, err = sat.Metainfo.Endpoint.CreateBucket(ctx, &pb.BucketCreateRequest{
			Header: &pb.RequestHeader{
				ApiKey: apiKey.SerializeRaw(),
			},
			Name: bucketName,
		})
		require.True(t, errs2.IsRPC(err, rpcstatus.NotFound))
	})
}

func TestCreateBucketWithCreatedBy(t *testing.T) {
	testplanet.Run(t, testplanet.Config{
		SatelliteCount: 1, UplinkCount: 1,
	}, func(t *testing.T, ctx *testcontext.Context, planet *testplanet.Planet) {
		sat := planet.Satellites[0]
		service := sat.API.Console.Service
		project := planet.Uplinks[0].Projects[0]

		userCtx, err := sat.UserContext(ctx, project.Owner.ID)
		require.NoError(t, err)

		apiKeyInfo, apiKey, err := service.CreateAPIKey(userCtx, project.ID, "test key", macaroon.APIKeyVersionMin)
		require.NoError(t, err)
		require.NotNil(t, apiKey)
		require.False(t, apiKeyInfo.CreatedBy.IsZero())

		bucketName := []byte("bucket")
		_, err = sat.Metainfo.Endpoint.CreateBucket(ctx, &pb.BucketCreateRequest{
			Header: &pb.RequestHeader{
				ApiKey: apiKey.SerializeRaw(),
			},
			Name: bucketName,
		})
		require.NoError(t, err)

		bucket, err := sat.API.DB.Buckets().GetBucket(ctx, bucketName, project.ID)
		require.NoError(t, err)
		require.False(t, bucket.CreatedBy.IsZero())
		require.Equal(t, apiKeyInfo.CreatedBy, bucket.CreatedBy)
	})
}

func TestGetBucketLocation(t *testing.T) {
	testplanet.Run(t, testplanet.Config{
		SatelliteCount: 1, StorageNodeCount: 0, UplinkCount: 1,
		Reconfigure: testplanet.Reconfigure{
			Satellite: func(log *zap.Logger, index int, config *satellite.Config) {
				config.Placement = nodeselection.ConfigurablePlacementRule{
					PlacementRules: "endpoint_bucket_test_placement.yaml",
				}
			},
		},
	}, func(t *testing.T, ctx *testcontext.Context, planet *testplanet.Planet) {
		apiKey := planet.Uplinks[0].APIKey[planet.Satellites[0].ID()]

		satellite := planet.Satellites[0]

		// not existing bucket
		_, err := satellite.API.Metainfo.Endpoint.GetBucketLocation(ctx, &pb.GetBucketLocationRequest{
			Header: &pb.RequestHeader{
				ApiKey: apiKey.SerializeRaw(),
			},
			Name: []byte("test-bucket"),
		})
		require.True(t, errs2.IsRPC(err, rpcstatus.NotFound))

		err = planet.Uplinks[0].TestingCreateBucket(ctx, planet.Satellites[0], "test-bucket")
		require.NoError(t, err)

		// bucket without location
		response, err := satellite.API.Metainfo.Endpoint.GetBucketLocation(ctx, &pb.GetBucketLocationRequest{
			Header: &pb.RequestHeader{
				ApiKey: apiKey.SerializeRaw(),
			},
			Name: []byte("test-bucket"),
		})
		require.NoError(t, err)
		require.Empty(t, response.Location)

		_, err = satellite.DB.Buckets().UpdateBucket(ctx, buckets.Bucket{
			ProjectID: planet.Uplinks[0].Projects[0].ID,
			Name:      "test-bucket",
			Placement: storj.PlacementConstraint(40),
		})
		require.NoError(t, err)

		// bucket with location
		response, err = satellite.API.Metainfo.Endpoint.GetBucketLocation(ctx, &pb.GetBucketLocationRequest{
			Header: &pb.RequestHeader{
				ApiKey: apiKey.SerializeRaw(),
			},
			Name: []byte("test-bucket"),
		})
		require.NoError(t, err)
		require.Equal(t, "Poland", string(response.Location))
	})
}

func TestSetBucketTagging(t *testing.T) {
	testplanet.Run(t, testplanet.Config{
		SatelliteCount: 1, UplinkCount: 1,
		Reconfigure: testplanet.Reconfigure{
			Satellite: func(log *zap.Logger, index int, config *satellite.Config) {
				config.Metainfo.BucketTaggingEnabled = true
			},
		},
	}, func(t *testing.T, ctx *testcontext.Context, planet *testplanet.Planet) {
		sat := planet.Satellites[0]
		project := planet.Uplinks[0].Projects[0]
		endpoint := sat.API.Metainfo.Endpoint
		bucketsDB := sat.DB.Buckets()

		apiKey := planet.Uplinks[0].APIKey[sat.ID()]

		createBucket := func(t *testing.T) string {
			bucketName := testrand.BucketName()
			_, err := bucketsDB.CreateBucket(ctx, buckets.Bucket{
				ID:        testrand.UUID(),
				Name:      bucketName,
				ProjectID: project.ID,
			})
			require.NoError(t, err)
			return bucketName
		}

		requireTags := func(t *testing.T, bucketName string, protoTags []*pb.BucketTag) {
			tags, err := bucketsDB.GetBucketTagging(ctx, []byte(bucketName), project.ID)
			require.NoError(t, err)

			if len(protoTags) == 0 {
				require.Empty(t, tags)
				return
			}

			var expectedTags []buckets.Tag
			for _, protoTag := range protoTags {
				expectedTags = append(expectedTags, buckets.Tag{
					Key:   string(protoTag.Key),
					Value: string(protoTag.Value),
				})
			}
			require.Equal(t, expectedTags, tags)
		}

		t.Run("Nonexistent bucket", func(t *testing.T) {
			bucketName := testrand.BucketName()

			_, err := endpoint.SetBucketTagging(ctx, &pb.SetBucketTaggingRequest{
				Header: &pb.RequestHeader{ApiKey: apiKey.SerializeRaw()},
				Name:   []byte(bucketName),
				Tags:   []*pb.BucketTag{},
			})

			require.True(t, errs2.IsRPC(err, rpcstatus.NotFound))
		})

		t.Run("No tags", func(t *testing.T) {
			bucketName := createBucket(t)

			_, err := endpoint.SetBucketTagging(ctx, &pb.SetBucketTaggingRequest{
				Header: &pb.RequestHeader{ApiKey: apiKey.SerializeRaw()},
				Name:   []byte(bucketName),
				Tags:   nil,
			})
			require.NoError(t, err)

			requireTags(t, bucketName, nil)
		})

		t.Run("Basic", func(t *testing.T) {
			bucketName := createBucket(t)

			expectedTags := []*pb.BucketTag{
				{ // Basic Latin letters, numbers, and certain symbols
					Key:   []byte("abcdeABCDE01234+-./:=@_"),
					Value: []byte("_@=:/.-+fghijFGHIJ56789"),
				},
				{ // Letters and numbers beyond the Basic Latin block
					Key:   []byte(string([]rune{'Ա', 'א', 'ء', 'ऄ', 'ঀ', '٠', '०', '০'})),
					Value: []byte(string([]rune{'ֆ', 'ת', 'ي', 'ह', 'হ', '٩', '९', '৯'})),
				},
				{ // Whitespace
					Key:   []byte("\t\n\v\f\r \xc2\x85\xc2\xa0\u1680\u2002"),
					Value: []byte("\u3000\u2003\xc2\xa0\xc2\x85 \r\f\v\n\t"),
				},
				{ // Ensure that empty tag values are allowed.
					Key:   []byte("key"),
					Value: []byte{},
				},
			}

			_, err := endpoint.SetBucketTagging(ctx, &pb.SetBucketTaggingRequest{
				Header: &pb.RequestHeader{ApiKey: apiKey.SerializeRaw()},
				Name:   []byte(bucketName),
				Tags:   expectedTags,
			})
			require.NoError(t, err)

			requireTags(t, bucketName, expectedTags)
		})

		t.Run("Missing bucket name", func(t *testing.T) {
			_, err := endpoint.SetBucketTagging(ctx, &pb.SetBucketTaggingRequest{
				Header: &pb.RequestHeader{ApiKey: apiKey.SerializeRaw()},
				Name:   []byte{},
			})
			require.True(t, errs2.IsRPC(err, rpcstatus.BucketNameMissing))
		})

		for _, tt := range []struct{ name, rep string }{
			{"ASCII", "A"},
			{"UTF-8", "𝒜"}, // '𝒜' was chosen because it occupies 4 bytes, which is the maximum for UTF-8.
		} {
			t.Run("Tag key length restriction - "+tt.name, func(t *testing.T) {
				const maxKeyLen = 128

				bucketName := createBucket(t)

				req := &pb.SetBucketTaggingRequest{
					Header: &pb.RequestHeader{ApiKey: apiKey.SerializeRaw()},
					Name:   []byte(bucketName),
					Tags:   []*pb.BucketTag{{}},
				}

				req.Tags[0].Key = []byte(strings.Repeat(tt.rep, maxKeyLen+1))
				_, err := endpoint.SetBucketTagging(ctx, req)
				require.True(t, errs2.IsRPC(err, rpcstatus.TagKeyInvalid))

				requireTags(t, bucketName, nil)

				req.Tags[0].Key = req.Tags[0].Key[:maxKeyLen]
				_, err = endpoint.SetBucketTagging(ctx, req)
				require.NoError(t, err)

				requireTags(t, bucketName, req.Tags)
			})

			t.Run("Tag value length restriction - "+tt.name, func(t *testing.T) {
				const maxValueLen = 256

				bucketName := createBucket(t)

				req := &pb.SetBucketTaggingRequest{
					Header: &pb.RequestHeader{ApiKey: apiKey.SerializeRaw()},
					Name:   []byte(bucketName),
					Tags: []*pb.BucketTag{{
						Key: []byte("key"),
					}},
				}

				req.Tags[0].Value = []byte(strings.Repeat(tt.rep, maxValueLen+1))
				_, err := endpoint.SetBucketTagging(ctx, req)
				require.True(t, errs2.IsRPC(err, rpcstatus.TagValueInvalid))

				requireTags(t, bucketName, nil)

				req.Tags[0].Value = req.Tags[0].Value[:maxValueLen]
				_, err = endpoint.SetBucketTagging(ctx, req)
				require.NoError(t, err)

				requireTags(t, bucketName, req.Tags)
			})
		}

		var tooManyitems []*pb.BucketTag
		for range 51 {
			tooManyitems = append(tooManyitems, &pb.BucketTag{
				Key:   []byte("key"),
				Value: []byte("value"),
			})
		}

		for _, tt := range []struct {
			name           string
			tags           []*pb.BucketTag
			expectedStatus rpcstatus.StatusCode
		}{
			{
				name:           "Too many items",
				tags:           tooManyitems,
				expectedStatus: rpcstatus.TooManyTags,
			},
			{
				name: "Duplicate tag key",
				tags: []*pb.BucketTag{
					{Key: []byte("key")},
					{Key: []byte("key")},
				},
				expectedStatus: rpcstatus.TagKeyDuplicate,
			},
			{
				name:           "Missing tag key",
				tags:           []*pb.BucketTag{{Key: nil}},
				expectedStatus: rpcstatus.TagKeyInvalid,
			},
			{
				name:           "Invalid tag key",
				tags:           []*pb.BucketTag{{Key: []byte("❌")}},
				expectedStatus: rpcstatus.TagKeyInvalid,
			},
			{
				name: "Invalid tag value",
				tags: []*pb.BucketTag{{
					Key:   []byte("key"),
					Value: []byte("❌"),
				}},
				expectedStatus: rpcstatus.TagValueInvalid,
			},
		} {
			t.Run(tt.name, func(t *testing.T) {
				bucketName := createBucket(t)

				_, err := endpoint.SetBucketTagging(ctx, &pb.SetBucketTaggingRequest{
					Header: &pb.RequestHeader{ApiKey: apiKey.SerializeRaw()},
					Name:   []byte(bucketName),
					Tags:   tt.tags,
				})
				require.True(t, errs2.IsRPC(err, tt.expectedStatus))

				requireTags(t, bucketName, nil)
			})
		}

		t.Run("Permission-restricted API key", func(t *testing.T) {
			restricted, err := apiKey.Restrict(macaroon.Caveat{DisallowWrites: true})
			require.NoError(t, err)

			_, err = endpoint.SetBucketTagging(ctx, &pb.SetBucketTaggingRequest{
				Header: &pb.RequestHeader{ApiKey: restricted.SerializeRaw()},
				Name:   []byte(testrand.BucketName()),
				Tags: []*pb.BucketTag{{
					Key:   []byte("key"),
					Value: []byte("value"),
				}},
			})
			require.True(t, errs2.IsRPC(err, rpcstatus.PermissionDenied))
		})

		t.Run("Prefix-restricted API key", func(t *testing.T) {
			bucketName := createBucket(t)

			restricted, err := apiKey.Restrict(macaroon.Caveat{AllowedPaths: []*macaroon.Caveat_Path{{
				EncryptedPathPrefix: testrand.Bytes(16),
			}}})
			require.NoError(t, err)

			_, err = endpoint.SetBucketTagging(ctx, &pb.SetBucketTaggingRequest{
				Header: &pb.RequestHeader{ApiKey: restricted.SerializeRaw()},
				Name:   []byte(bucketName),
				Tags: []*pb.BucketTag{{
					Key:   []byte("key"),
					Value: []byte("value"),
				}},
			})
			require.True(t, errs2.IsRPC(err, rpcstatus.PermissionDenied))

			requireTags(t, bucketName, nil)
		})

		t.Run("Bucket-restricted API key", func(t *testing.T) {
			bucketName := createBucket(t)

			restricted, err := apiKey.Restrict(macaroon.Caveat{AllowedPaths: []*macaroon.Caveat_Path{{
				Bucket: []byte(bucketName),
			}}})
			require.NoError(t, err)

			expectedTags := []*pb.BucketTag{{
				Key:   []byte("key"),
				Value: []byte("value"),
			}}

			_, err = endpoint.SetBucketTagging(ctx, &pb.SetBucketTaggingRequest{
				Header: &pb.RequestHeader{ApiKey: restricted.SerializeRaw()},
				Name:   []byte(bucketName),
				Tags: []*pb.BucketTag{{
					Key:   []byte("key"),
					Value: []byte("value"),
				}},
			})
			require.NoError(t, err)
			requireTags(t, bucketName, expectedTags)

			restricted, err = apiKey.Restrict(macaroon.Caveat{AllowedPaths: []*macaroon.Caveat_Path{{
				Bucket: []byte(testrand.BucketName()),
			}}})
			require.NoError(t, err)

			_, err = endpoint.SetBucketTagging(ctx, &pb.SetBucketTaggingRequest{
				Header: &pb.RequestHeader{ApiKey: restricted.SerializeRaw()},
				Name:   []byte(bucketName),
				Tags: []*pb.BucketTag{{
					Key:   []byte("foo"),
					Value: []byte("bar"),
				}},
			})
			require.True(t, errs2.IsRPC(err, rpcstatus.PermissionDenied))
			requireTags(t, bucketName, expectedTags)
		})
	})
}

func TestGetBucketTagging(t *testing.T) {
	testplanet.Run(t, testplanet.Config{
		SatelliteCount: 1, UplinkCount: 1,
		Reconfigure: testplanet.Reconfigure{
			Satellite: func(log *zap.Logger, index int, config *satellite.Config) {
				config.Metainfo.BucketTaggingEnabled = true
			},
		},
	}, func(t *testing.T, ctx *testcontext.Context, planet *testplanet.Planet) {
		sat := planet.Satellites[0]
		project := planet.Uplinks[0].Projects[0]
		endpoint := sat.API.Metainfo.Endpoint
		bucketsDB := sat.DB.Buckets()

		apiKey := planet.Uplinks[0].APIKey[sat.ID()]

		createBucket := func(t *testing.T) string {
			bucketName := testrand.BucketName()
			_, err := bucketsDB.CreateBucket(ctx, buckets.Bucket{
				ID:        testrand.UUID(),
				Name:      bucketName,
				ProjectID: project.ID,
			})
			require.NoError(t, err)
			return bucketName
		}

		t.Run("Nonexistent bucket", func(t *testing.T) {
			tags, err := endpoint.GetBucketTagging(ctx, &pb.GetBucketTaggingRequest{
				Header: &pb.RequestHeader{ApiKey: apiKey.SerializeRaw()},
				Name:   []byte(testrand.BucketName()),
			})
			require.True(t, errs2.IsRPC(err, rpcstatus.NotFound))
			require.Nil(t, tags)
		})

		t.Run("No tags", func(t *testing.T) {
			bucketName := createBucket(t)
			tags, err := endpoint.GetBucketTagging(ctx, &pb.GetBucketTaggingRequest{
				Header: &pb.RequestHeader{ApiKey: apiKey.SerializeRaw()},
				Name:   []byte(bucketName),
			})
			require.True(t, errs2.IsRPC(err, rpcstatus.TagsNotFound))
			require.Nil(t, tags)
		})

		t.Run("Basic", func(t *testing.T) {
			bucketName := createBucket(t)

			expectedTags := []buckets.Tag{
				{
					Key:   "abcdeABCDE01234+-./:=@_",
					Value: "_@=:/.-+fghijFGHIJ56789",
				},
				{
					Key:   string([]rune{'Ա', 'א', 'ء', 'ऄ', 'ঀ', '٠', '०', '০'}),
					Value: string([]rune{'ֆ', 'ת', 'ي', 'ह', 'হ', '٩', '९', '৯'}),
				},
				{
					Key:   "\t\n\v\f\r \xc2\x85\xc2\xa0\u1680\u2002",
					Value: "\u3000\u2003\xc2\xa0\xc2\x85 \r\f\v\n\t",
				},
				{
					Key:   "key",
					Value: "",
				},
			}
			require.NoError(t, bucketsDB.SetBucketTagging(ctx, []byte(bucketName), project.ID, expectedTags))

			tags, err := endpoint.GetBucketTagging(ctx, &pb.GetBucketTaggingRequest{
				Header: &pb.RequestHeader{ApiKey: apiKey.SerializeRaw()},
				Name:   []byte(bucketName),
			})
			require.NoError(t, err)

			var actualTags []buckets.Tag
			for _, tag := range tags.Tags {
				actualTags = append(actualTags, buckets.Tag{
					Key:   string(tag.Key),
					Value: string(tag.Value),
				})
			}

			require.Equal(t, expectedTags, actualTags)
		})

		t.Run("Missing bucket name", func(t *testing.T) {
			tags, err := endpoint.GetBucketTagging(ctx, &pb.GetBucketTaggingRequest{
				Header: &pb.RequestHeader{ApiKey: apiKey.SerializeRaw()},
				Name:   []byte{},
			})
			require.True(t, errs2.IsRPC(err, rpcstatus.BucketNameMissing))
			require.Nil(t, tags)
		})

		t.Run("Prefix-restricted API key", func(t *testing.T) {
			restricted, err := apiKey.Restrict(macaroon.Caveat{AllowedPaths: []*macaroon.Caveat_Path{{
				EncryptedPathPrefix: testrand.Bytes(16),
			}}})
			require.NoError(t, err)

			tags, err := endpoint.GetBucketTagging(ctx, &pb.GetBucketTaggingRequest{
				Header: &pb.RequestHeader{ApiKey: restricted.SerializeRaw()},
				Name:   []byte(testrand.BucketName()),
			})
			require.True(t, errs2.IsRPC(err, rpcstatus.PermissionDenied))
			require.Nil(t, tags)
		})

		t.Run("Bucket-restricted API key", func(t *testing.T) {
			restricted, err := apiKey.Restrict(macaroon.Caveat{AllowedPaths: []*macaroon.Caveat_Path{{
				Bucket: []byte(testrand.BucketName()),
			}}})
			require.NoError(t, err)

			tags, err := endpoint.GetBucketTagging(ctx, &pb.GetBucketTaggingRequest{
				Header: &pb.RequestHeader{ApiKey: restricted.SerializeRaw()},
				Name:   []byte(testrand.BucketName()),
			})
			require.True(t, errs2.IsRPC(err, rpcstatus.PermissionDenied))
			require.Nil(t, tags)
		})
	})
}

func TestEnableSuspendBucketVersioning(t *testing.T) {
	testplanet.Run(t, testplanet.Config{
		SatelliteCount: 1, UplinkCount: 1,
		Reconfigure: testplanet.Reconfigure{
			Satellite: func(log *zap.Logger, index int, config *satellite.Config) {
				config.Metainfo.UseBucketLevelObjectVersioning = true
			},
		},
	}, func(t *testing.T, ctx *testcontext.Context, planet *testplanet.Planet) {
		bucketName := "testbucket"
		projectID := planet.Uplinks[0].Projects[0].ID
		satellite := planet.Satellites[0]

		deleteBucket := func() error {
			err := satellite.API.DB.Buckets().DeleteBucket(ctx, []byte(bucketName), projectID)
			return err
		}

		_, err := satellite.API.Metainfo.Endpoint.GetBucketVersioning(ctx, &pb.GetBucketVersioningRequest{
			Header: &pb.RequestHeader{
				ApiKey: planet.Uplinks[0].APIKey[satellite.ID()].SerializeRaw(),
			},
			Name: []byte("non-existing-bucket"),
		})
		require.True(t, errs2.IsRPC(err, rpcstatus.NotFound))

		_, err = satellite.API.Metainfo.Endpoint.SetBucketVersioning(ctx, &pb.SetBucketVersioningRequest{
			Header: &pb.RequestHeader{
				ApiKey: planet.Uplinks[0].APIKey[satellite.ID()].SerializeRaw(),
			},
			Name:       []byte("non-existing-bucket"),
			Versioning: true,
		})
		require.True(t, errs2.IsRPC(err, rpcstatus.NotFound))

		_, err = satellite.API.Metainfo.Endpoint.SetBucketVersioning(ctx, &pb.SetBucketVersioningRequest{
			Header: &pb.RequestHeader{
				ApiKey: planet.Uplinks[0].APIKey[satellite.ID()].SerializeRaw(),
			},
			Name:       []byte("non-existing-bucket"),
			Versioning: false,
		})
		require.True(t, errs2.IsRPC(err, rpcstatus.NotFound))

		for _, tt := range []struct {
			name                     string
			objectLockEnabled        bool
			initialVersioningState   buckets.Versioning
			versioning               bool
			resultantVersioningState buckets.Versioning
			expectedErrCode          rpcstatus.StatusCode
		}{
			{
				name:                     "Enable unsupported bucket fails",
				initialVersioningState:   buckets.VersioningUnsupported,
				versioning:               true,
				resultantVersioningState: buckets.VersioningUnsupported,
				expectedErrCode:          rpcstatus.FailedPrecondition,
			}, {
				name:                     "Suspend unsupported bucket fails",
				initialVersioningState:   buckets.VersioningUnsupported,
				resultantVersioningState: buckets.VersioningUnsupported,
				expectedErrCode:          rpcstatus.FailedPrecondition,
			}, {
				name:                     "Enable unversioned bucket succeeds",
				initialVersioningState:   buckets.Unversioned,
				versioning:               true,
				resultantVersioningState: buckets.VersioningEnabled,
			}, {
				name:                     "Suspend unversioned bucket fails",
				initialVersioningState:   buckets.Unversioned,
				resultantVersioningState: buckets.Unversioned,
				expectedErrCode:          rpcstatus.FailedPrecondition,
			}, {
				name:                     "Enable enabled bucket succeeds",
				initialVersioningState:   buckets.VersioningEnabled,
				versioning:               true,
				resultantVersioningState: buckets.VersioningEnabled,
			}, {
				name:                     "Suspend enabled bucket succeeds",
				initialVersioningState:   buckets.VersioningEnabled,
				resultantVersioningState: buckets.VersioningSuspended,
			}, {
				name:                     "Enable suspended bucket succeeds",
				initialVersioningState:   buckets.VersioningSuspended,
				versioning:               true,
				resultantVersioningState: buckets.VersioningEnabled,
			}, {
				name:                     "Suspend suspended bucket succeeds",
				initialVersioningState:   buckets.VersioningSuspended,
				resultantVersioningState: buckets.VersioningSuspended,
			}, {
				name:                     "Suspend bucket with Object Lock enabled fails",
				objectLockEnabled:        true,
				initialVersioningState:   buckets.VersioningEnabled,
				resultantVersioningState: buckets.VersioningEnabled,
				expectedErrCode:          rpcstatus.ObjectLockInvalidBucketState,
			},
		} {
			t.Run(tt.name, func(t *testing.T) {
				defer ctx.Check(deleteBucket)
				bucket, err := satellite.API.DB.Buckets().CreateBucket(ctx, buckets.Bucket{
					ProjectID:  projectID,
					Name:       bucketName,
					Versioning: tt.initialVersioningState,
					ObjectLock: buckets.ObjectLockSettings{
						Enabled: tt.objectLockEnabled,
					},
				})
				require.NoError(t, err)
				require.NotNil(t, bucket)
				setResponse, err := satellite.API.Metainfo.Endpoint.SetBucketVersioning(ctx, &pb.SetBucketVersioningRequest{
					Header: &pb.RequestHeader{
						ApiKey: planet.Uplinks[0].APIKey[satellite.ID()].SerializeRaw(),
					},
					Name:       []byte(bucketName),
					Versioning: tt.versioning,
				})
				if tt.expectedErrCode != 0 {
					require.Error(t, err)
					rpctest.RequireCode(t, err, tt.expectedErrCode)
				} else {
					require.NoError(t, err)
				}
				require.Empty(t, setResponse)
				getResponse, err := satellite.API.Metainfo.Endpoint.GetBucketVersioning(ctx, &pb.GetBucketVersioningRequest{
					Header: &pb.RequestHeader{
						ApiKey: planet.Uplinks[0].APIKey[satellite.ID()].SerializeRaw(),
					},
					Name: []byte(bucketName),
				})
				require.NoError(t, err)
				require.Equal(t, tt.resultantVersioningState, buckets.Versioning(getResponse.Versioning))
			})
		}
	})
}

func TestDefaultBucketVersioning(t *testing.T) {
	testplanet.Run(t, testplanet.Config{
		SatelliteCount: 1, UplinkCount: 1,
		Reconfigure: testplanet.Reconfigure{
			Satellite: func(log *zap.Logger, index int, config *satellite.Config) {
				config.Metainfo.UseBucketLevelObjectVersioning = true
			},
		},
	}, func(t *testing.T, ctx *testcontext.Context, planet *testplanet.Planet) {
		bucketName := "testbucket"
		satellite := planet.Satellites[0]
		apiKey := planet.Uplinks[0].APIKey[satellite.ID()]
		projectID := planet.Uplinks[0].Projects[0].ID

		deleteBucket := func() error {
			err := satellite.API.DB.Buckets().DeleteBucket(ctx, []byte(bucketName), projectID)
			return err
		}
		// unversioned is tested first here because it should be the default and not require
		// an update to the test planet project's default versioning state.
		t.Run("default versioning - unversioned", func(t *testing.T) {
			defer ctx.Check(deleteBucket)
			_, err := satellite.Metainfo.Endpoint.CreateBucket(ctx, &pb.BucketCreateRequest{
				Header: &pb.RequestHeader{
					ApiKey: apiKey.SerializeRaw(),
				},
				Name: []byte(bucketName),
			})
			require.NoError(t, err)

			getResponse, err := satellite.API.Metainfo.Endpoint.GetBucketVersioning(ctx, &pb.GetBucketVersioningRequest{
				Header: &pb.RequestHeader{
					ApiKey: apiKey.SerializeRaw(),
				},
				Name: []byte(bucketName),
			})
			require.NoError(t, err)
			require.Equal(t, buckets.Unversioned, buckets.Versioning(getResponse.Versioning))
		})

		t.Run("default versioning - unsupported", func(t *testing.T) {
			defer ctx.Check(deleteBucket)
			err := satellite.DB.Console().Projects().UpdateDefaultVersioning(ctx, projectID, console.VersioningUnsupported)
			require.NoError(t, err)

			_, err = satellite.Metainfo.Endpoint.CreateBucket(ctx, &pb.BucketCreateRequest{
				Header: &pb.RequestHeader{
					ApiKey: apiKey.SerializeRaw(),
				},
				Name: []byte(bucketName),
			})
			require.NoError(t, err)

			getResponse, err := satellite.API.Metainfo.Endpoint.GetBucketVersioning(ctx, &pb.GetBucketVersioningRequest{
				Header: &pb.RequestHeader{
					ApiKey: apiKey.SerializeRaw(),
				},
				Name: []byte(bucketName),
			})
			require.NoError(t, err)
			require.Equal(t, buckets.Unversioned, buckets.Versioning(getResponse.Versioning))
		})

		t.Run("default versioning - enabled", func(t *testing.T) {
			defer ctx.Check(deleteBucket)
			err := satellite.DB.Console().Projects().UpdateDefaultVersioning(ctx, projectID, console.VersioningEnabled)
			require.NoError(t, err)

			_, err = satellite.Metainfo.Endpoint.CreateBucket(ctx, &pb.BucketCreateRequest{
				Header: &pb.RequestHeader{
					ApiKey: apiKey.SerializeRaw(),
				},
				Name: []byte(bucketName),
			})
			require.NoError(t, err)

			getResponse, err := satellite.API.Metainfo.Endpoint.GetBucketVersioning(ctx, &pb.GetBucketVersioningRequest{
				Header: &pb.RequestHeader{
					ApiKey: apiKey.SerializeRaw(),
				},
				Name: []byte(bucketName),
			})
			require.NoError(t, err)
			require.Equal(t, buckets.VersioningEnabled, buckets.Versioning(getResponse.Versioning))
		})
	})
}

func TestCreateBucketWithObjectLockEnabled(t *testing.T) {
	testplanet.Run(t, testplanet.Config{
		SatelliteCount: 1, UplinkCount: 1,
		Reconfigure: testplanet.Reconfigure{
			Satellite: func(log *zap.Logger, index int, config *satellite.Config) {
				config.Metainfo.ObjectLockEnabled = true
				config.Metainfo.UseBucketLevelObjectVersioning = true
			},
		},
	}, func(t *testing.T, ctx *testcontext.Context, planet *testplanet.Planet) {
		sat := planet.Satellites[0]
		project := planet.Uplinks[0].Projects[0]
		endpoint := sat.API.Metainfo.Endpoint

		userCtx, err := sat.UserContext(ctx, project.Owner.ID)
		require.NoError(t, err)

		_, apiKey, err := sat.API.Console.Service.CreateAPIKey(userCtx, project.ID, "test key", macaroon.APIKeyVersionObjectLock)
		require.NoError(t, err)

		require.NoError(t, sat.DB.Console().Projects().UpdateDefaultVersioning(ctx, project.ID, console.VersioningEnabled))

		t.Run("Success - Object Lock enabled", func(t *testing.T) {
			bucketName := []byte(testrand.BucketName())
			_, err = endpoint.CreateBucket(ctx, &pb.CreateBucketRequest{
				Header: &pb.RequestHeader{
					ApiKey: apiKey.SerializeRaw(),
				},
				Name:              bucketName,
				ObjectLockEnabled: true,
			})
			require.NoError(t, err)

			enabled, err := sat.DB.Buckets().GetBucketObjectLockEnabled(ctx, bucketName, project.ID)
			require.NoError(t, err)
			require.True(t, enabled)
		})

		t.Run("Success - Object Lock disabled", func(t *testing.T) {
			bucketName := []byte(testrand.BucketName())
			_, err = endpoint.CreateBucket(ctx, &pb.CreateBucketRequest{
				Header: &pb.RequestHeader{
					ApiKey: apiKey.SerializeRaw(),
				},
				Name: bucketName,
			})
			require.NoError(t, err)

			enabled, err := sat.DB.Buckets().GetBucketObjectLockEnabled(ctx, bucketName, project.ID)
			require.NoError(t, err)
			require.False(t, enabled)
		})

		t.Run("Object Lock not globally supported", func(t *testing.T) {
			endpoint.TestSetObjectLockEnabled(false)
			defer endpoint.TestSetObjectLockEnabled(true)

			bucketName := []byte(testrand.BucketName())
			req := &pb.CreateBucketRequest{
				Header: &pb.RequestHeader{
					ApiKey: apiKey.SerializeRaw(),
				},
				Name:              bucketName,
				ObjectLockEnabled: true,
			}
			_, err = endpoint.CreateBucket(ctx, req)
			rpctest.RequireCode(t, err, rpcstatus.ObjectLockDisabledForProject)
		})

		t.Run("Unauthorized API key", func(t *testing.T) {
			_, oldApiKey, err := sat.API.Console.Service.CreateAPIKey(userCtx, project.ID, "old key", macaroon.APIKeyVersionMin)
			require.NoError(t, err)

			bucketName := []byte(testrand.BucketName())
			_, err = endpoint.CreateBucket(ctx, &pb.CreateBucketRequest{
				Header: &pb.RequestHeader{
					ApiKey: oldApiKey.SerializeRaw(),
				},
				Name:              bucketName,
				ObjectLockEnabled: true,
			})
			rpctest.RequireCode(t, err, rpcstatus.PermissionDenied)

			restrictedApiKey, err := apiKey.Restrict(macaroon.Caveat{DisallowPutBucketObjectLockConfiguration: true})
			require.NoError(t, err)

			_, err = endpoint.CreateBucket(ctx, &pb.CreateBucketRequest{
				Header: &pb.RequestHeader{
					ApiKey: restrictedApiKey.SerializeRaw(),
				},
				Name:              bucketName,
				ObjectLockEnabled: true,
			})
			rpctest.RequireCode(t, err, rpcstatus.PermissionDenied)
		})

		t.Run("Object versioning disabled", func(t *testing.T) {
			endpoint.TestSetUseBucketLevelVersioning(false)
			defer endpoint.TestSetUseBucketLevelVersioning(true)

			bucketName := []byte(testrand.BucketName())
			req := &pb.CreateBucketRequest{
				Header: &pb.RequestHeader{
					ApiKey: apiKey.SerializeRaw(),
				},
				Name:              bucketName,
				ObjectLockEnabled: true,
			}

			_, err = endpoint.CreateBucket(ctx, req)
			rpctest.RequireCode(t, err, rpcstatus.ObjectLockDisabledForProject)
		})
	})
}

func TestGetBucketObjectLockConfiguration(t *testing.T) {
	testplanet.Run(t, testplanet.Config{
		SatelliteCount: 1, UplinkCount: 1,
		Reconfigure: testplanet.Reconfigure{
			Satellite: func(log *zap.Logger, index int, config *satellite.Config) {
				config.Metainfo.ObjectLockEnabled = true
				config.Metainfo.UseBucketLevelObjectVersioning = true
			},
		},
	}, func(t *testing.T, ctx *testcontext.Context, planet *testplanet.Planet) {
		sat := planet.Satellites[0]
		project := planet.Uplinks[0].Projects[0]
		endpoint := sat.API.Metainfo.Endpoint

		require.NoError(t, sat.DB.Console().Projects().UpdateDefaultVersioning(ctx, project.ID, console.VersioningEnabled))

		userCtx, err := sat.UserContext(ctx, project.Owner.ID)
		require.NoError(t, err)

		_, apiKey, err := sat.API.Console.Service.CreateAPIKey(userCtx, project.ID, "test key", macaroon.APIKeyVersionObjectLock)
		require.NoError(t, err)

		createBucket := func(t *testing.T, name []byte, lockEnabled bool) {
			_, err := endpoint.CreateBucket(ctx, &pb.BucketCreateRequest{
				Header: &pb.RequestHeader{
					ApiKey: apiKey.SerializeRaw(),
				},
				Name:              name,
				ObjectLockEnabled: lockEnabled,
			})
			require.NoError(t, err)
		}

		t.Run("Success", func(t *testing.T) {
			bucketName := []byte(testrand.BucketName())
			createBucket(t, bucketName, true)

			duration := pb.DefaultRetention_Years{Years: 5}
			_, err = endpoint.SetBucketObjectLockConfiguration(ctx, &pb.SetBucketObjectLockConfigurationRequest{
				Header: &pb.RequestHeader{
					ApiKey: apiKey.SerializeRaw(),
				},
				Name: bucketName,
				Configuration: &pb.ObjectLockConfiguration{
					Enabled: true,
					DefaultRetention: &pb.DefaultRetention{
						Mode:     pb.Retention_GOVERNANCE,
						Duration: &duration,
					},
				},
			})
			require.NoError(t, err)

			resp, err := endpoint.GetBucketObjectLockConfiguration(ctx, &pb.GetBucketObjectLockConfigurationRequest{
				Header: &pb.RequestHeader{
					ApiKey: apiKey.SerializeRaw(),
				},
				Name: bucketName,
			})
			require.NoError(t, err)
			require.True(t, resp.Configuration.Enabled)
			require.Equal(t, pb.Retention_GOVERNANCE, resp.Configuration.DefaultRetention.Mode)
			require.Equal(t, &duration, resp.Configuration.DefaultRetention.Duration)
		})

		t.Run("Object Lock disabled", func(t *testing.T) {
			bucketName := []byte(testrand.BucketName())
			createBucket(t, bucketName, false)

			_, err := endpoint.GetBucketObjectLockConfiguration(ctx, &pb.GetBucketObjectLockConfigurationRequest{
				Header: &pb.RequestHeader{
					ApiKey: apiKey.SerializeRaw(),
				},
				Name: bucketName,
			})
			rpctest.RequireCode(t, err, rpcstatus.ObjectLockBucketRetentionConfigurationMissing)
		})

		t.Run("Object Lock not globally supported", func(t *testing.T) {
			bucketName := []byte(testrand.BucketName())
			createBucket(t, bucketName, true)

			endpoint.TestSetObjectLockEnabled(false)
			defer endpoint.TestSetObjectLockEnabled(true)

			req := &pb.GetBucketObjectLockConfigurationRequest{
				Header: &pb.RequestHeader{
					ApiKey: apiKey.SerializeRaw(),
				},
				Name: bucketName,
			}
			_, err := endpoint.GetBucketObjectLockConfiguration(ctx, req)
			rpctest.RequireCode(t, err, rpcstatus.ObjectLockEndpointsDisabled)
		})

		t.Run("Nonexistent bucket", func(t *testing.T) {
			_, err = endpoint.GetBucketObjectLockConfiguration(ctx, &pb.GetBucketObjectLockConfigurationRequest{
				Header: &pb.RequestHeader{
					ApiKey: apiKey.SerializeRaw(),
				},
				Name: []byte(testrand.BucketName()),
			})
			rpctest.RequireCode(t, err, rpcstatus.NotFound)
		})

		t.Run("Unauthorized API key", func(t *testing.T) {
			bucketName := []byte(testrand.BucketName())
			createBucket(t, bucketName, true)

			_, oldApiKey, err := sat.API.Console.Service.CreateAPIKey(userCtx, project.ID, "old key", macaroon.APIKeyVersionMin)
			require.NoError(t, err)

			_, err = endpoint.GetBucketObjectLockConfiguration(ctx, &pb.GetBucketObjectLockConfigurationRequest{
				Header: &pb.RequestHeader{
					ApiKey: oldApiKey.SerializeRaw(),
				},
				Name: bucketName,
			})
			rpctest.RequireCode(t, err, rpcstatus.PermissionDenied)

			bucketName = []byte(testrand.BucketName())
			createBucket(t, bucketName, true)

			restrictedApiKey, err := apiKey.Restrict(macaroon.Caveat{
				DisallowGetBucketObjectLockConfiguration: true,
				DisallowPutBucketObjectLockConfiguration: true,
			})
			require.NoError(t, err)

			_, err = endpoint.GetBucketObjectLockConfiguration(ctx, &pb.GetBucketObjectLockConfigurationRequest{
				Header: &pb.RequestHeader{
					ApiKey: restrictedApiKey.SerializeRaw(),
				},
				Name: bucketName,
			})
			rpctest.RequireCode(t, err, rpcstatus.PermissionDenied)
		})
	})
}

func TestSetBucketObjectLockConfiguration(t *testing.T) {
	testplanet.Run(t, testplanet.Config{
		SatelliteCount: 1, UplinkCount: 1,
		Reconfigure: testplanet.Reconfigure{
			Satellite: func(log *zap.Logger, index int, config *satellite.Config) {
				config.Metainfo.ObjectLockEnabled = true
				config.Metainfo.UseBucketLevelObjectVersioning = true
			},
		},
	}, func(t *testing.T, ctx *testcontext.Context, planet *testplanet.Planet) {
		sat := planet.Satellites[0]
		project := planet.Uplinks[0].Projects[0]
		endpoint := sat.API.Metainfo.Endpoint
		bucketsDB := sat.DB.Buckets()

		require.NoError(t, sat.DB.Console().Projects().UpdateDefaultVersioning(ctx, project.ID, console.VersioningEnabled))

		userCtx, err := sat.UserContext(ctx, project.Owner.ID)
		require.NoError(t, err)

		_, apiKey, err := sat.API.Console.Service.CreateAPIKey(userCtx, project.ID, "test key", macaroon.APIKeyVersionObjectLock)
		require.NoError(t, err)

		createBucket := func(t *testing.T, name []byte, lockEnabled bool) {
			_, err := endpoint.CreateBucket(ctx, &pb.BucketCreateRequest{
				Header: &pb.RequestHeader{
					ApiKey: apiKey.SerializeRaw(),
				},
				Name:              name,
				ObjectLockEnabled: lockEnabled,
			})
			require.NoError(t, err)
		}

		t.Run("Success", func(t *testing.T) {
			bucketName := []byte(testrand.BucketName())
			createBucket(t, bucketName, false)

			resp, err := endpoint.SetBucketObjectLockConfiguration(ctx, &pb.SetBucketObjectLockConfigurationRequest{
				Header: &pb.RequestHeader{
					ApiKey: apiKey.SerializeRaw(),
				},
				Name: bucketName,
				Configuration: &pb.ObjectLockConfiguration{
					Enabled: true,
				},
			})
			require.NoError(t, err)
			require.NotNil(t, resp)

			enabled, err := bucketsDB.GetBucketObjectLockEnabled(ctx, bucketName, project.ID)
			require.NoError(t, err)
			require.True(t, enabled)
		})

		t.Run("set Object Lock config on unversioned bucket", func(t *testing.T) {
			bucketName := []byte(testrand.BucketName())
			createBucket(t, bucketName, false)

			_, err = endpoint.SetBucketVersioning(ctx, &pb.SetBucketVersioningRequest{
				Header: &pb.RequestHeader{
					ApiKey: apiKey.SerializeRaw(),
				},
				Name:       bucketName,
				Versioning: false,
			})
			require.NoError(t, err)

			_, err = endpoint.SetBucketObjectLockConfiguration(ctx, &pb.SetBucketObjectLockConfigurationRequest{
				Header: &pb.RequestHeader{
					ApiKey: apiKey.SerializeRaw(),
				},
				Name: bucketName,
				Configuration: &pb.ObjectLockConfiguration{
					Enabled: true,
				},
			})
			rpctest.RequireCode(t, err, rpcstatus.ObjectLockInvalidBucketState)
		})

		t.Run("try disable Object Lock", func(t *testing.T) {
			bucketName := []byte(testrand.BucketName())
			createBucket(t, bucketName, true)

			_, err := endpoint.SetBucketObjectLockConfiguration(ctx, &pb.SetBucketObjectLockConfigurationRequest{
				Header: &pb.RequestHeader{
					ApiKey: apiKey.SerializeRaw(),
				},
				Name: bucketName,
				Configuration: &pb.ObjectLockConfiguration{
					Enabled: false,
				},
			})
			rpctest.RequireCode(t, err, rpcstatus.ObjectLockInvalidBucketRetentionConfiguration)
		})

		t.Run("Object Lock not globally supported", func(t *testing.T) {
			bucketName := []byte(testrand.BucketName())
			createBucket(t, bucketName, false)

			endpoint.TestSetObjectLockEnabled(false)
			defer endpoint.TestSetObjectLockEnabled(true)

			req := &pb.SetBucketObjectLockConfigurationRequest{
				Header: &pb.RequestHeader{
					ApiKey: apiKey.SerializeRaw(),
				},
				Name: bucketName,
				Configuration: &pb.ObjectLockConfiguration{
					Enabled: true,
				},
			}
			_, err := endpoint.SetBucketObjectLockConfiguration(ctx, req)
			rpctest.RequireCode(t, err, rpcstatus.ObjectLockEndpointsDisabled)
		})

		t.Run("Nonexistent bucket", func(t *testing.T) {
			_, err = endpoint.SetBucketObjectLockConfiguration(ctx, &pb.SetBucketObjectLockConfigurationRequest{
				Header: &pb.RequestHeader{
					ApiKey: apiKey.SerializeRaw(),
				},
				Name: []byte(testrand.BucketName()),
				Configuration: &pb.ObjectLockConfiguration{
					Enabled: true,
				},
			})
			rpctest.RequireCode(t, err, rpcstatus.NotFound)
		})

		t.Run("Unauthorized API key", func(t *testing.T) {
			bucketName := []byte(testrand.BucketName())
			createBucket(t, bucketName, true)

			_, oldApiKey, err := sat.API.Console.Service.CreateAPIKey(userCtx, project.ID, "old key", macaroon.APIKeyVersionMin)
			require.NoError(t, err)

			_, err = endpoint.SetBucketObjectLockConfiguration(ctx, &pb.SetBucketObjectLockConfigurationRequest{
				Header: &pb.RequestHeader{
					ApiKey: oldApiKey.SerializeRaw(),
				},
				Name: bucketName,
			})
			rpctest.RequireCode(t, err, rpcstatus.PermissionDenied)

			bucketName = []byte(testrand.BucketName())
			createBucket(t, bucketName, true)

			restrictedApiKey, err := apiKey.Restrict(macaroon.Caveat{
				DisallowPutBucketObjectLockConfiguration: true,
			})
			require.NoError(t, err)

			_, err = endpoint.SetBucketObjectLockConfiguration(ctx, &pb.SetBucketObjectLockConfigurationRequest{
				Header: &pb.RequestHeader{
					ApiKey: restrictedApiKey.SerializeRaw(),
				},
				Name: bucketName,
			})
			rpctest.RequireCode(t, err, rpcstatus.PermissionDenied)
		})

		t.Run("invalid config", func(t *testing.T) {
			bucketName := []byte(testrand.BucketName())
			createBucket(t, bucketName, false)

			config := &pb.ObjectLockConfiguration{
				Enabled: true,
				DefaultRetention: &pb.DefaultRetention{
					Mode: pb.Retention_COMPLIANCE,
				},
			}
			request := &pb.SetBucketObjectLockConfigurationRequest{
				Header: &pb.RequestHeader{
					ApiKey: apiKey.SerializeRaw(),
				},
				Name:          bucketName,
				Configuration: config,
			}

			// Mode is set but Duration is missing.
			_, err = endpoint.SetBucketObjectLockConfiguration(ctx, request)
			rpctest.RequireCode(t, err, rpcstatus.ObjectLockInvalidBucketRetentionConfiguration)

			// Object Lock cannot be disabled once enabled.
			config.Enabled = false
			_, err = endpoint.SetBucketObjectLockConfiguration(ctx, request)
			rpctest.RequireCode(t, err, rpcstatus.ObjectLockInvalidBucketRetentionConfiguration)

			config.Enabled = true

			// Mode is invalid.
			config.DefaultRetention.Mode = pb.Retention_INVALID
			config.DefaultRetention.Duration = &pb.DefaultRetention_Days{
				Days: 1,
			}
			_, err = endpoint.SetBucketObjectLockConfiguration(ctx, request)
			rpctest.RequireCode(t, err, rpcstatus.ObjectLockInvalidBucketRetentionConfiguration)

			// Duration value is zero (invalid).
			config.DefaultRetention.Mode = pb.Retention_COMPLIANCE
			config.DefaultRetention.Duration = &pb.DefaultRetention_Days{
				Days: 0,
			}
			_, err = endpoint.SetBucketObjectLockConfiguration(ctx, request)
			rpctest.RequireCode(t, err, rpcstatus.ObjectLockInvalidBucketRetentionConfiguration)

			// Duration value is negative (invalid).
			config.DefaultRetention.Duration = &pb.DefaultRetention_Days{
				Days: -1,
			}
			_, err = endpoint.SetBucketObjectLockConfiguration(ctx, request)
			rpctest.RequireCode(t, err, rpcstatus.ObjectLockInvalidBucketRetentionConfiguration)

			// Duration with days set above maximum (invalid).
			config.DefaultRetention.Duration = &pb.DefaultRetention_Days{
				Days: 36501,
			}
			_, err = endpoint.SetBucketObjectLockConfiguration(ctx, request)
			rpctest.RequireCode(t, err, rpcstatus.ObjectLockInvalidBucketRetentionConfiguration)

			// Duration with Years set to zero (invalid).
			config.DefaultRetention.Duration = &pb.DefaultRetention_Years{
				Years: 0,
			}
			_, err = endpoint.SetBucketObjectLockConfiguration(ctx, request)
			rpctest.RequireCode(t, err, rpcstatus.ObjectLockInvalidBucketRetentionConfiguration)

			// Duration with Years set to negative value (invalid).
			config.DefaultRetention.Duration = &pb.DefaultRetention_Years{
				Years: -1,
			}
			_, err = endpoint.SetBucketObjectLockConfiguration(ctx, request)
			rpctest.RequireCode(t, err, rpcstatus.ObjectLockInvalidBucketRetentionConfiguration)

			// Duration with Years set above maximum (invalid).
			config.DefaultRetention.Duration = &pb.DefaultRetention_Years{
				Years: 11,
			}
			_, err = endpoint.SetBucketObjectLockConfiguration(ctx, request)
			rpctest.RequireCode(t, err, rpcstatus.ObjectLockInvalidBucketRetentionConfiguration)

			// Both Days and Years are not set in Duration (invalid).
			config.DefaultRetention.Duration = nil
			_, err = endpoint.SetBucketObjectLockConfiguration(ctx, request)
			rpctest.RequireCode(t, err, rpcstatus.ObjectLockInvalidBucketRetentionConfiguration)
		})

		t.Run("disable default retention", func(t *testing.T) {
			bucketName := []byte(testrand.BucketName())
			createBucket(t, bucketName, false)

			config := &pb.ObjectLockConfiguration{
				Enabled: true,
				DefaultRetention: &pb.DefaultRetention{
					Mode: pb.Retention_COMPLIANCE,
					Duration: &pb.DefaultRetention_Years{
						Years: 1,
					},
				},
			}
			request := &pb.SetBucketObjectLockConfigurationRequest{
				Header: &pb.RequestHeader{
					ApiKey: apiKey.SerializeRaw(),
				},
				Name:          bucketName,
				Configuration: config,
			}

			_, err = endpoint.SetBucketObjectLockConfiguration(ctx, request)
			require.NoError(t, err)

			bucket, err := bucketsDB.GetBucket(ctx, bucketName, project.ID)
			require.NoError(t, err)
			require.Equal(t, buckets.ObjectLockSettings{
				Enabled:               true,
				DefaultRetentionMode:  storj.ComplianceMode,
				DefaultRetentionDays:  0,
				DefaultRetentionYears: 1,
			}, bucket.ObjectLock)

			config.DefaultRetention = nil
			_, err = endpoint.SetBucketObjectLockConfiguration(ctx, request)
			require.NoError(t, err)

			bucket, err = bucketsDB.GetBucket(ctx, bucketName, project.ID)
			require.NoError(t, err)
			require.Equal(t, buckets.ObjectLockSettings{
				Enabled: true,
			}, bucket.ObjectLock)
		})
	})
}
