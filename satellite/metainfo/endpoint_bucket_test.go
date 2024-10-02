// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package metainfo_test

import (
	"errors"
	"fmt"
	"strconv"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"github.com/zeebo/errs"
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
	"storj.io/storj/satellite/metabase"
	"storj.io/storj/satellite/nodeselection"
	"storj.io/uplink"
	"storj.io/uplink/private/metaclient"
)

func TestBucketExistenceCheck(t *testing.T) {
	testplanet.Run(t, testplanet.Config{
		SatelliteCount: 1, UplinkCount: 1, EnableSpanner: true,
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
		require.Equal(t, buckets.ErrBucketNotFound.New("%s", "non-existing-bucket").Error(), errs.Unwrap(err).Error())

		_, _, err = metainfoClient.ListObjects(ctx, metaclient.ListObjectsParams{
			Bucket: []byte("non-existing-bucket"),
		})
		require.Error(t, err)
		require.True(t, errs2.IsRPC(err, rpcstatus.NotFound))
		require.Equal(t, buckets.ErrBucketNotFound.New("%s", "non-existing-bucket").Error(), errs.Unwrap(err).Error())
	})
}

func TestMaxOutBuckets(t *testing.T) {
	testplanet.Run(t, testplanet.Config{
		SatelliteCount: 1, UplinkCount: 1, EnableSpanner: true,
	}, func(t *testing.T, ctx *testcontext.Context, planet *testplanet.Planet) {
		limit := planet.Satellites[0].Config.Metainfo.ProjectLimits.MaxBuckets
		for i := 1; i <= limit; i++ {
			name := "test" + strconv.Itoa(i)
			err := planet.Uplinks[0].CreateBucket(ctx, planet.Satellites[0], name)
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
		SatelliteCount: 1, UplinkCount: 1, EnableSpanner: true,
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
		},
		SatelliteCount: 1, StorageNodeCount: 4, UplinkCount: 1, EnableSpanner: true,
	}, func(t *testing.T, ctx *testcontext.Context, planet *testplanet.Planet) {
		ownerAPIKey := planet.Uplinks[0].APIKey[planet.Satellites[0].ID()]
		sat := planet.Satellites[0]
		uplnk := planet.Uplinks[0]
		project := uplnk.Projects[0]
		endpoint := sat.API.Metainfo.Endpoint

		expectedObjects := map[string][]byte{
			"single-segment-object":        testrand.Bytes(10 * memory.KiB),
			"multi-segment-object":         testrand.Bytes(50 * memory.KiB),
			"remote-segment-inline-object": testrand.Bytes(33 * memory.KiB),
		}

		uploadObjects := func(t *testing.T, bucketName metabase.BucketName) {
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
				ProjectID:         project.ID,
				Name:              bucketName.String(),
				ObjectLockEnabled: true,
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
		SatelliteCount: 1, UplinkCount: 1, EnableSpanner: true,
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

func TestCraeteBucketWithCreatedBy(t *testing.T) {
	testplanet.Run(t, testplanet.Config{
		SatelliteCount: 1, UplinkCount: 1, EnableSpanner: true,
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
		SatelliteCount: 1, StorageNodeCount: 0, UplinkCount: 1, EnableSpanner: true,
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

		err = planet.Uplinks[0].CreateBucket(ctx, planet.Satellites[0], "test-bucket")
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

func TestEnableSuspendBucketVersioning(t *testing.T) {
	testplanet.Run(t, testplanet.Config{
		SatelliteCount: 1, UplinkCount: 1, EnableSpanner: true,
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
					ProjectID:         projectID,
					Name:              bucketName,
					Versioning:        tt.initialVersioningState,
					ObjectLockEnabled: tt.objectLockEnabled,
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
		SatelliteCount: 1, UplinkCount: 1, EnableSpanner: true,
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
			require.Equal(t, buckets.VersioningUnsupported, buckets.Versioning(getResponse.Versioning))
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
		SatelliteCount: 1, UplinkCount: 1, EnableSpanner: true,
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

			endpoint.TestSetUseBucketLevelVersioningByProjectID(project.ID, true)

			_, err = endpoint.CreateBucket(ctx, req)
			require.NoError(t, err)

			endpoint.TestSetUseBucketLevelVersioningByProjectID(project.ID, false)

			req.Name = []byte(testrand.BucketName())
			_, err = endpoint.CreateBucket(ctx, req)
			rpctest.RequireCode(t, err, rpcstatus.ObjectLockDisabledForProject)

			require.NoError(t, sat.API.Console.Service.UpdateVersioningOptInStatus(userCtx, project.ID, console.VersioningOptIn))
			_, err = endpoint.CreateBucket(ctx, req)
			require.NoError(t, err)
		})
	})
}

func TestGetBucketObjectLockConfiguration(t *testing.T) {
	testplanet.Run(t, testplanet.Config{
		SatelliteCount: 1, UplinkCount: 1, EnableSpanner: true,
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

			resp, err := endpoint.GetBucketObjectLockConfiguration(ctx, &pb.GetBucketObjectLockConfigurationRequest{
				Header: &pb.RequestHeader{
					ApiKey: apiKey.SerializeRaw(),
				},
				Name: bucketName,
			})
			require.NoError(t, err)
			require.True(t, resp.Configuration.Enabled)
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
				DisallowGetRetention: true,
				DisallowPutRetention: true, // GetRetention is implicitly allowed if PutRetention is allowed
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
