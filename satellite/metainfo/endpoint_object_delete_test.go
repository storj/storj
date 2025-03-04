// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package metainfo_test

import (
	"context"
	"strconv"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"github.com/zeebo/errs"
	"go.uber.org/zap"

	"storj.io/common/macaroon"
	"storj.io/common/memory"
	"storj.io/common/pb"
	"storj.io/common/rpc/rpcstatus"
	"storj.io/common/rpc/rpctest"
	"storj.io/common/storj"
	"storj.io/common/testcontext"
	"storj.io/common/testrand"
	"storj.io/common/uuid"
	"storj.io/storj/private/testplanet"
	"storj.io/storj/satellite"
	"storj.io/storj/satellite/buckets"
	"storj.io/storj/satellite/metabase"
	"storj.io/storj/satellite/metabase/metabasetest"
	"storj.io/storj/satellite/metainfo"
)

func TestEndpoint_DeleteCommittedObject(t *testing.T) {
	createObject := func(ctx context.Context, t *testing.T, planet *testplanet.Planet, bucket, key string, data []byte) {
		err := planet.Uplinks[0].Upload(ctx, planet.Satellites[0], bucket, key, data)
		require.NoError(t, err)
	}
	deleteObject := func(ctx context.Context, t *testing.T, planet *testplanet.Planet, bucket, encryptedKey string, streamID uuid.UUID) {
		projectID := planet.Uplinks[0].Projects[0].ID

		_, err := planet.Satellites[0].Metainfo.Endpoint.DeleteCommittedObject(ctx, metainfo.DeleteCommittedObject{
			ObjectLocation: metabase.ObjectLocation{
				ObjectKey:  metabase.ObjectKey(encryptedKey),
				ProjectID:  projectID,
				BucketName: metabase.BucketName(bucket),
			},
			Version: []byte{},
		})
		require.NoError(t, err)
	}
	testDeleteObject(t, createObject, deleteObject)
}

func testDeleteObject(t *testing.T,
	createObject func(ctx context.Context, t *testing.T, planet *testplanet.Planet, bucket, key string, data []byte),
	deleteObject func(ctx context.Context, t *testing.T, planet *testplanet.Planet, bucket, encryptedKey string, streamID uuid.UUID),
) {
	bucketName := "deleteobjects"
	t.Run("all nodes up", func(t *testing.T) {
		t.Parallel()

		var testCases = []struct {
			caseDescription string
			objData         []byte
			hasRemote       bool
		}{
			{caseDescription: "one remote segment", objData: testrand.Bytes(10 * memory.KiB)},
			{caseDescription: "one inline segment", objData: testrand.Bytes(3 * memory.KiB)},
			{caseDescription: "several segments (all remote)", objData: testrand.Bytes(50 * memory.KiB)},
			{caseDescription: "several segments (remote + inline)", objData: testrand.Bytes(33 * memory.KiB)},
		}

		testplanet.Run(t, testplanet.Config{
			SatelliteCount: 1, StorageNodeCount: 4, UplinkCount: 1,
			Reconfigure: testplanet.Reconfigure{
				// Reconfigure RS for ensuring that we don't have long-tail cancellations
				// and the upload doesn't leave garbage in the SNs
				Satellite: testplanet.Combine(
					testplanet.ReconfigureRS(2, 2, 4, 4),
					testplanet.MaxSegmentSize(13*memory.KiB),
				),
			},
		}, func(t *testing.T, ctx *testcontext.Context, planet *testplanet.Planet) {
			for _, tc := range testCases {
				tc := tc
				t.Run(tc.caseDescription, func(t *testing.T) {
					createObject(ctx, t, planet, bucketName, tc.caseDescription, tc.objData)

					// calculate the SNs total used space after data upload
					var totalUsedSpace int64
					for _, sn := range planet.StorageNodes {
						report, err := sn.Storage2.Monitor.DiskSpace(ctx)
						require.NoError(t, err)
						totalUsedSpace += report.UsedForPieces
					}

					objects, err := planet.Satellites[0].Metabase.DB.TestingAllObjects(ctx)
					require.NoError(t, err)
					for _, object := range objects {
						deleteObject(ctx, t, planet, bucketName, string(object.ObjectKey), object.StreamID)
					}

					// calculate the SNs used space after delete the pieces
					var totalUsedSpaceAfterDelete int64
					for _, sn := range planet.StorageNodes {
						report, err := sn.Storage2.Monitor.DiskSpace(ctx)
						require.NoError(t, err)
						totalUsedSpaceAfterDelete += report.UsedForPieces
					}

					// we are not deleting data from SN right away so used space should be the same
					require.Equal(t, totalUsedSpace, totalUsedSpaceAfterDelete)
				})
			}
		})

	})

	t.Run("some nodes down", func(t *testing.T) {
		t.Parallel()

		var testCases = []struct {
			caseDescription string
			objData         []byte
		}{
			{caseDescription: "one remote segment", objData: testrand.Bytes(10 * memory.KiB)},
			{caseDescription: "several segments (all remote)", objData: testrand.Bytes(50 * memory.KiB)},
			{caseDescription: "several segments (remote + inline)", objData: testrand.Bytes(33 * memory.KiB)},
		}

		testplanet.Run(t, testplanet.Config{
			SatelliteCount: 1, StorageNodeCount: 4, UplinkCount: 1,
			Reconfigure: testplanet.Reconfigure{
				// Reconfigure RS for ensuring that we don't have long-tail cancellations
				// and the upload doesn't leave garbage in the SNs
				Satellite: testplanet.Combine(
					testplanet.ReconfigureRS(2, 2, 4, 4),
					testplanet.MaxSegmentSize(13*memory.KiB),
				),
			},
		}, func(t *testing.T, ctx *testcontext.Context, planet *testplanet.Planet) {
			numToShutdown := 2

			for _, tc := range testCases {
				createObject(ctx, t, planet, bucketName, tc.caseDescription, tc.objData)
			}

			require.NoError(t, planet.WaitForStorageNodeEndpoints(ctx))

			// Shutdown the first numToShutdown storage nodes before we delete the pieces
			// and collect used space values for those nodes
			snUsedSpace := make([]int64, len(planet.StorageNodes))
			for i, node := range planet.StorageNodes {
				report, err := node.Storage2.Monitor.DiskSpace(ctx)
				require.NoError(t, err)
				snUsedSpace[i] = report.UsedForPieces

				if i < numToShutdown {
					require.NoError(t, planet.StopPeer(node))
				}
			}

			objects, err := planet.Satellites[0].Metabase.DB.TestingAllObjects(ctx)
			require.NoError(t, err)
			for _, object := range objects {
				deleteObject(ctx, t, planet, bucketName, string(object.ObjectKey), object.StreamID)
			}

			// we are not deleting data from SN right away so used space should be the same
			// for online and shutdown/offline node
			for i, sn := range planet.StorageNodes {
				report, err := sn.Storage2.Monitor.DiskSpace(ctx)
				require.NoError(t, err)

				require.Equal(t, snUsedSpace[i], report.UsedForPieces, "StorageNode #%d", i)
			}
		})
	})

	t.Run("all nodes down", func(t *testing.T) {
		t.Parallel()

		var testCases = []struct {
			caseDescription string
			objData         []byte
		}{
			{caseDescription: "one remote segment", objData: testrand.Bytes(10 * memory.KiB)},
			{caseDescription: "several segments (all remote)", objData: testrand.Bytes(50 * memory.KiB)},
			{caseDescription: "several segments (remote + inline)", objData: testrand.Bytes(33 * memory.KiB)},
		}

		testplanet.Run(t, testplanet.Config{
			SatelliteCount: 1, StorageNodeCount: 4, UplinkCount: 1,
			Reconfigure: testplanet.Reconfigure{
				// Reconfigure RS for ensuring that we don't have long-tail cancellations
				// and the upload doesn't leave garbage in the SNs
				Satellite: testplanet.Combine(
					testplanet.ReconfigureRS(2, 2, 4, 4),
					testplanet.MaxSegmentSize(13*memory.KiB),
				),
			},
		}, func(t *testing.T, ctx *testcontext.Context, planet *testplanet.Planet) {
			for _, tc := range testCases {
				createObject(ctx, t, planet, bucketName, tc.caseDescription, tc.objData)
			}

			// calculate the SNs total used space after data upload
			var usedSpaceBeforeDelete int64
			for _, sn := range planet.StorageNodes {
				report, err := sn.Storage2.Monitor.DiskSpace(ctx)
				require.NoError(t, err)
				usedSpaceBeforeDelete += report.UsedForPieces
			}

			// Shutdown all the storage nodes before we delete the pieces
			for _, sn := range planet.StorageNodes {
				require.NoError(t, planet.StopPeer(sn))
			}

			objects, err := planet.Satellites[0].Metabase.DB.TestingAllObjects(ctx)
			require.NoError(t, err)
			for _, object := range objects {
				deleteObject(ctx, t, planet, bucketName, string(object.ObjectKey), object.StreamID)
			}

			// Check that storage nodes that were offline when deleting the pieces
			// they are still holding data
			var totalUsedSpace int64
			for _, sn := range planet.StorageNodes {
				report, err := sn.Storage2.Monitor.DiskSpace(ctx)
				require.NoError(t, err)
				totalUsedSpace += report.UsedForPieces
			}

			require.Equal(t, usedSpaceBeforeDelete, totalUsedSpace, "totalUsedSpace")
		})
	})
}

func TestEndpoint_ParallelDeletes(t *testing.T) {
	t.Skip("to be fixed - creating deadlocks")
	testplanet.Run(t, testplanet.Config{
		SatelliteCount:   1,
		StorageNodeCount: 4,
		UplinkCount:      1,
	}, func(t *testing.T, ctx *testcontext.Context, planet *testplanet.Planet) {
		project, err := planet.Uplinks[0].OpenProject(ctx, planet.Satellites[0])
		require.NoError(t, err)
		defer ctx.Check(project.Close)
		testData := testrand.Bytes(5 * memory.KiB)
		for i := 0; i < 50; i++ {
			err := planet.Uplinks[0].Upload(ctx, planet.Satellites[0], "bucket", "object"+strconv.Itoa(i), testData)
			require.NoError(t, err)
			_, err = project.CopyObject(ctx, "bucket", "object"+strconv.Itoa(i), "bucket", "object"+strconv.Itoa(i)+"copy", nil)
			require.NoError(t, err)
		}
		list := project.ListObjects(ctx, "bucket", nil)
		keys := []string{}
		for list.Next() {
			item := list.Item()
			keys = append(keys, item.Key)
		}
		require.NoError(t, list.Err())
		var wg sync.WaitGroup
		wg.Add(len(keys))
		var errlist errs.Group

		for i, name := range keys {
			name := name
			go func(toDelete string, index int) {
				_, err := project.DeleteObject(ctx, "bucket", toDelete)
				errlist.Add(err)
				wg.Done()
			}(name, i)
		}
		wg.Wait()

		require.NoError(t, errlist.Err())

		// check all objects have been deleted
		listAfterDelete := project.ListObjects(ctx, "bucket", nil)
		require.False(t, listAfterDelete.Next())
		require.NoError(t, listAfterDelete.Err())

		_, err = project.DeleteBucket(ctx, "bucket")
		require.NoError(t, err)
	})
}

func TestEndpoint_ParallelDeletesSameAncestor(t *testing.T) {
	t.Skip("to be fixed - creating deadlocks")
	testplanet.Run(t, testplanet.Config{
		SatelliteCount:   1,
		StorageNodeCount: 4,
		UplinkCount:      1,
	}, func(t *testing.T, ctx *testcontext.Context, planet *testplanet.Planet) {
		project, err := planet.Uplinks[0].OpenProject(ctx, planet.Satellites[0])
		require.NoError(t, err)
		defer ctx.Check(project.Close)
		testData := testrand.Bytes(5 * memory.KiB)
		err = planet.Uplinks[0].Upload(ctx, planet.Satellites[0], "bucket", "original-object", testData)
		require.NoError(t, err)
		for i := 0; i < 50; i++ {
			_, err = project.CopyObject(ctx, "bucket", "original-object", "bucket", "copy"+strconv.Itoa(i), nil)
			require.NoError(t, err)
		}
		list := project.ListObjects(ctx, "bucket", nil)
		keys := []string{}
		for list.Next() {
			item := list.Item()
			keys = append(keys, item.Key)
		}
		require.NoError(t, list.Err())
		var wg sync.WaitGroup
		wg.Add(len(keys))
		var errlist errs.Group

		for i, name := range keys {
			name := name
			go func(toDelete string, index int) {
				_, err := project.DeleteObject(ctx, "bucket", toDelete)
				errlist.Add(err)
				wg.Done()
			}(name, i)
		}
		wg.Wait()

		require.NoError(t, errlist.Err())

		// check all objects have been deleted
		listAfterDelete := project.ListObjects(ctx, "bucket", nil)
		require.False(t, listAfterDelete.Next())
		require.NoError(t, listAfterDelete.Err())

		_, err = project.DeleteBucket(ctx, "bucket")
		require.NoError(t, err)
	})
}

func TestEndpoint_DeleteLockedObject(t *testing.T) {
	testplanet.Run(t, testplanet.Config{
		SatelliteCount: 1, UplinkCount: 1,
		Reconfigure: testplanet.Reconfigure{
			Satellite: func(log *zap.Logger, index int, config *satellite.Config) {
				config.Metainfo.ObjectLockEnabled = true
				config.Metainfo.UseBucketLevelObjectVersioning = true
			},
		},
	}, func(t *testing.T, ctx *testcontext.Context, planet *testplanet.Planet) {
		const unauthorizedErrMsg = "Unauthorized API credentials"

		sat := planet.Satellites[0]
		project := planet.Uplinks[0].Projects[0]
		endpoint := sat.Metainfo.Endpoint
		db := sat.Metabase.DB

		userCtx, err := sat.UserContext(ctx, project.Owner.ID)
		require.NoError(t, err)

		_, apiKey, err := sat.API.Console.Service.CreateAPIKey(userCtx, project.ID, "test key", macaroon.APIKeyVersionObjectLock)
		require.NoError(t, err)

		getObject := func(bucketName, key string) metabase.Object {
			objects, err := sat.Metabase.DB.TestingAllObjects(ctx)
			require.NoError(t, err)
			for _, o := range objects {
				if o.Location() == (metabase.ObjectLocation{
					ProjectID:  project.ID,
					BucketName: metabase.BucketName(bucketName),
					ObjectKey:  metabase.ObjectKey(key),
				}) {
					return o
				}
			}
			return metabase.Object{}
		}

		requireObject := func(t *testing.T, bucketName, key string) {
			obj := getObject(bucketName, key)
			require.NotZero(t, obj)
		}

		requireNoObject := func(t *testing.T, bucketName, key string) {
			obj := getObject(bucketName, key)
			require.Zero(t, obj)
		}

		createBucket := func(t *testing.T, name string, lockEnabled bool) {
			_, err := sat.DB.Buckets().CreateBucket(ctx, buckets.Bucket{
				Name:       name,
				ProjectID:  project.ID,
				Versioning: buckets.VersioningEnabled,
				ObjectLock: buckets.ObjectLockSettings{
					Enabled: lockEnabled,
				},
			})
			require.NoError(t, err)
		}

		type testOpts struct {
			bucketName  string
			testCase    metabasetest.ObjectLockDeletionTestCase
			expectError bool
		}

		test := func(t *testing.T, opts testOpts) {
			fn := func(useExactVersion bool) {
				objStream := randObjectStream(project.ID, opts.bucketName)

				object, _ := metabasetest.CreateTestObject{
					BeginObjectExactVersion: &metabase.BeginObjectExactVersion{
						ObjectStream: objStream,
						Encryption:   metabasetest.DefaultEncryption,
						Retention:    opts.testCase.Retention,
						LegalHold:    opts.testCase.LegalHold,
					},
				}.Run(ctx, t, db, objStream, 0)

				var version []byte
				if useExactVersion {
					version = object.StreamVersionID().Bytes()
				}

				_, err := endpoint.BeginDeleteObject(ctx, &pb.BeginDeleteObjectRequest{
					Header:                    &pb.RequestHeader{ApiKey: apiKey.SerializeRaw()},
					Bucket:                    []byte(objStream.BucketName),
					EncryptedObjectKey:        []byte(objStream.ObjectKey),
					ObjectVersion:             version,
					BypassGovernanceRetention: opts.testCase.BypassGovernance,
				})

				if opts.expectError && useExactVersion {
					require.Error(t, err)
					rpctest.RequireStatus(t, err, rpcstatus.ObjectLockObjectProtected, objectLockedErrMsg)
					requireObject(t, opts.bucketName, string(objStream.ObjectKey))
					return
				}
				require.NoError(t, err)
				if useExactVersion {
					requireNoObject(t, opts.bucketName, string(objStream.ObjectKey))
				} else {
					requireObject(t, opts.bucketName, string(objStream.ObjectKey))
				}
			}

			t.Run("Exact version", func(t *testing.T) { fn(true) })
			t.Run("Last committed version", func(t *testing.T) { fn(false) })
		}

		t.Run("Object Lock enabled for bucket", func(t *testing.T) {
			bucketName := testrand.BucketName()
			createBucket(t, bucketName, true)

			metabasetest.ObjectLockDeletionTestRunner{
				TestProtected: func(t *testing.T, testCase metabasetest.ObjectLockDeletionTestCase) {
					test(t, testOpts{
						bucketName:  bucketName,
						testCase:    testCase,
						expectError: true,
					})
				},
				TestRemovable: func(t *testing.T, testCase metabasetest.ObjectLockDeletionTestCase) {
					test(t, testOpts{
						bucketName:  bucketName,
						testCase:    testCase,
						expectError: false,
					})
				},
			}.Run(t)

			t.Run("Active retention - Pending", func(t *testing.T) {
				objectKey := metabasetest.RandObjectKey()

				beginResp, err := endpoint.BeginObject(ctx, &pb.BeginObjectRequest{
					Header:             &pb.RequestHeader{ApiKey: apiKey.SerializeRaw()},
					Bucket:             []byte(bucketName),
					EncryptedObjectKey: []byte(objectKey),
					EncryptionParameters: &pb.EncryptionParameters{
						CipherSuite: pb.CipherSuite_ENC_AESGCM,
					},
					Retention: &pb.Retention{
						Mode:        pb.Retention_COMPLIANCE,
						RetainUntil: time.Now().Add(time.Hour),
					},
				})
				require.NoError(t, err)

				_, err = endpoint.BeginDeleteObject(ctx, &pb.BeginDeleteObjectRequest{
					Header:             &pb.RequestHeader{ApiKey: apiKey.SerializeRaw()},
					Bucket:             []byte(bucketName),
					EncryptedObjectKey: []byte(objectKey),
					StreamId:           &beginResp.StreamId,
					Status:             int32(metabase.Pending),
				})
				require.NoError(t, err)

				requireNoObject(t, bucketName, string(objectKey))
			})

			t.Run("Unauthorized API key - Governance bypass", func(t *testing.T) {
				objStream := randObjectStream(project.ID, bucketName)
				object, _ := metabasetest.CreateObjectWithRetention(ctx, t, db, objStream, 0, metabase.Retention{
					Mode:        storj.GovernanceMode,
					RetainUntil: time.Now().Add(time.Hour),
				})

				_, oldApiKey, err := sat.API.Console.Service.CreateAPIKey(userCtx, project.ID, "old key", macaroon.APIKeyVersionMin)
				require.NoError(t, err)

				req := &pb.BeginDeleteObjectRequest{
					Header:                    &pb.RequestHeader{ApiKey: oldApiKey.SerializeRaw()},
					Bucket:                    []byte(objStream.BucketName),
					EncryptedObjectKey:        []byte(objStream.ObjectKey),
					ObjectVersion:             object.StreamVersionID().Bytes(),
					BypassGovernanceRetention: true,
				}

				_, err = endpoint.BeginDeleteObject(ctx, req)
				require.Error(t, err)
				rpctest.RequireStatus(t, err, rpcstatus.PermissionDenied, unauthorizedErrMsg)

				restrictedApiKey, err := apiKey.Restrict(macaroon.Caveat{DisallowBypassGovernanceRetention: true})
				require.NoError(t, err)

				req.Header.ApiKey = restrictedApiKey.SerializeRaw()
				_, err = endpoint.BeginDeleteObject(ctx, req)
				require.Error(t, err)
				rpctest.RequireStatus(t, err, rpcstatus.PermissionDenied, unauthorizedErrMsg)

				requireObject(t, bucketName, string(objStream.ObjectKey))
			})
		})

		t.Run("Object Lock disabled for bucket", func(t *testing.T) {
			bucketName := testrand.BucketName()
			createBucket(t, bucketName, false)

			testFn := func(t *testing.T, testCase metabasetest.ObjectLockDeletionTestCase) {
				test(t, testOpts{
					bucketName:  bucketName,
					testCase:    testCase,
					expectError: false,
				})
			}

			metabasetest.ObjectLockDeletionTestRunner{
				TestProtected: testFn,
				TestRemovable: testFn,
			}.Run(t)
		})
	})
}
