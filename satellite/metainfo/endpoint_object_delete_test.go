// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package metainfo_test

import (
	"context"
	"errors"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"github.com/zeebo/errs"
	"go.uber.org/zap"
	"golang.org/x/exp/slices"

	"storj.io/common/errs2"
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
	"storj.io/storj/satellite/internalpb"
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
			for _, node := range planet.StorageNodes {
				node.Storage2.MigrationChore.Loop.Pause()
			}

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
			for _, node := range planet.StorageNodes {
				node.Storage2.MigrationChore.Loop.Pause()
			}

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
			for _, node := range planet.StorageNodes {
				node.Storage2.MigrationChore.Loop.Pause()
			}

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

func TestEndpoint_DeleteObjects(t *testing.T) {
	const errorTriggerObjectKey = "internal-error"

	testplanet.Run(t, testplanet.Config{
		SatelliteCount: 1, UplinkCount: 1,
		Reconfigure: testplanet.Reconfigure{
			Satellite: func(log *zap.Logger, index int, config *satellite.Config) {
				config.Metainfo.DeleteObjectsEnabled = true
			},
			SatelliteMetabaseDBConfig: func(log *zap.Logger, index int, config *metabase.Config) {
				config.TestingWrapAdapter = func(adapter metabase.Adapter) metabase.Adapter {
					return &deleteObjectsTestAdapter{
						Adapter:               adapter,
						errorTriggerObjectKey: errorTriggerObjectKey,
					}
				}
			},
		},
	}, func(t *testing.T, ctx *testcontext.Context, planet *testplanet.Planet) {
		sat := planet.Satellites[0]
		project := planet.Uplinks[0].Projects[0]
		endpoint := sat.Metainfo.Endpoint

		userCtx, err := sat.UserContext(ctx, project.Owner.ID)
		require.NoError(t, err)

		_, apiKey, err := sat.API.Console.Service.CreateAPIKey(userCtx, project.ID, "test key", macaroon.APIKeyVersionObjectLock)
		require.NoError(t, err)
		apiKeyHeader := &pb.RequestHeader{ApiKey: apiKey.SerializeRaw()}

		type minimalObject struct {
			key     []byte
			version []byte
		}

		createBucket := func(t *testing.T) string {
			bucketName := testrand.BucketName()
			require.NoError(t, planet.Uplinks[0].TestingCreateBucket(ctx, sat, bucketName))
			return bucketName
		}

		setBucketVersioning := func(t *testing.T, bucketName string, enabled bool) {
			_, err := endpoint.SetBucketVersioning(ctx, &pb.SetBucketVersioningRequest{
				Header:     apiKeyHeader,
				Name:       []byte(bucketName),
				Versioning: enabled,
			})
			require.NoError(t, err)
		}

		enableObjectLock := func(t *testing.T, bucketName string) {
			_, err := endpoint.SetBucketObjectLockConfiguration(ctx, &pb.SetBucketObjectLockConfigurationRequest{
				Header: apiKeyHeader,
				Name:   []byte(bucketName),
				Configuration: &pb.ObjectLockConfiguration{
					Enabled: true,
				},
			})
			require.NoError(t, err)
		}

		beginObject := func(t *testing.T, bucketName string, objectKey []byte) *pb.BeginObjectResponse {
			beginResp, err := endpoint.BeginObject(ctx, &pb.BeginObjectRequest{
				Header:             apiKeyHeader,
				Bucket:             []byte(bucketName),
				EncryptedObjectKey: objectKey,
				EncryptionParameters: &pb.EncryptionParameters{
					CipherSuite: pb.CipherSuite_ENC_AESGCM,
					BlockSize:   256,
				},
			})
			require.NoError(t, err)
			return beginResp
		}

		createCommittedObjectWithKey := func(t *testing.T, bucketName string, objectKey []byte) minimalObject {
			beginResp := beginObject(t, bucketName, objectKey)

			commitResp, err := endpoint.CommitObject(ctx, &pb.CommitObjectRequest{
				Header:   apiKeyHeader,
				StreamId: beginResp.StreamId,
			})
			require.NoError(t, err)

			return minimalObject{
				key:     objectKey,
				version: commitResp.Object.ObjectVersion,
			}
		}

		createCommittedObject := func(t *testing.T, bucketName string) minimalObject {
			return createCommittedObjectWithKey(t, bucketName, []byte(testrand.Path()))
		}

		createLockedCommittedObjectWithKey := func(t *testing.T, bucketName string, objectKey []byte, retention metabase.Retention, legalHold bool) minimalObject {
			objStream := metabase.ObjectStream{
				ProjectID:  project.ID,
				BucketName: metabase.BucketName(bucketName),
				ObjectKey:  metabase.ObjectKey(objectKey),
				Version:    1,
				StreamID:   testrand.UUID(),
			}

			metabasetest.CreateTestObject{
				BeginObjectExactVersion: &metabase.BeginObjectExactVersion{
					ObjectStream: objStream,
					Retention:    retention,
					LegalHold:    legalHold,
					Encryption:   metabasetest.DefaultEncryption,
				},
				CommitObject: &metabase.CommitObject{
					ObjectStream: objStream,
					Versioned:    true,
				},
			}.Run(ctx, t, sat.Metabase.DB, objStream, 0)

			return minimalObject{
				key:     objectKey,
				version: metabase.NewStreamVersionID(objStream.Version, objStream.StreamID).Bytes(),
			}
		}

		createLockedCommittedObject := func(t *testing.T, bucketName string, retention metabase.Retention, legalHold bool) minimalObject {
			return createLockedCommittedObjectWithKey(t, bucketName, []byte(testrand.Path()), retention, legalHold)
		}

		createPendingObject := func(t *testing.T, bucketName string) minimalObject {
			objectKey := []byte(testrand.Path())
			beginResp := beginObject(t, bucketName, objectKey)

			satStreamID := &internalpb.StreamID{}
			err = pb.Unmarshal(beginResp.StreamId, satStreamID)
			require.NoError(t, err)

			streamID, err := uuid.FromBytes(satStreamID.StreamId)
			require.NoError(t, err)

			return minimalObject{
				key:     objectKey,
				version: metabase.NewStreamVersionID(metabase.Version(satStreamID.Version), streamID).Bytes(),
			}
		}

		committedObjectExists := func(t *testing.T, bucketName string, obj minimalObject) bool {
			_, err := endpoint.GetObject(ctx, &pb.GetObjectRequest{
				Header:             apiKeyHeader,
				Bucket:             []byte(bucketName),
				EncryptedObjectKey: obj.key,
				ObjectVersion:      obj.version,
			})
			if errs2.IsRPC(err, rpcstatus.NotFound) {
				return false
			}
			require.NoError(t, err)
			return true
		}

		pendingObjectExists := func(t *testing.T, bucketName string, obj minimalObject) bool {
			streamVersionID, err := metabase.StreamVersionIDFromBytes(obj.version)
			require.NoError(t, err)

			listResp, err := endpoint.ListPendingObjectStreams(ctx, &pb.ListPendingObjectStreamsRequest{
				Header:             apiKeyHeader,
				Bucket:             []byte(bucketName),
				EncryptedObjectKey: obj.key,
				Limit:              1,
			})
			require.NoError(t, err)

			switch {
			case len(listResp.Items) == 0:
				return false
			case !slices.Equal(listResp.Items[0].EncryptedObjectKey, obj.key):
				return false
			case !slices.Equal(listResp.Items[0].ObjectVersion, obj.version):
				return false
			}

			require.Len(t, listResp.Items, 1)
			require.Equal(t, listResp.Items[0].ObjectVersion, streamVersionID.Bytes())
			return true
		}

		getLastCommittedVersionAndStatus := func(t *testing.T, bucketName string, objectKey []byte) ([]byte, pb.Object_Status) {
			listResp, err := endpoint.ListObjects(ctx, &pb.ListObjectsRequest{
				Header:             apiKeyHeader,
				Bucket:             []byte(bucketName),
				EncryptedCursor:    objectKey,
				VersionCursor:      metabase.NewStreamVersionID(metabase.MaxVersion+1, uuid.UUID{}).Bytes(),
				IncludeAllVersions: true,
				Recursive:          true,
				Limit:              1,
			})
			require.NoError(t, err)
			require.NotEmpty(t, listResp.Items)

			listItem := listResp.Items[0]
			require.Equal(t, listItem.EncryptedObjectKey, objectKey)

			return listItem.ObjectVersion, listItem.Status
		}

		randStreamVersionID := func() []byte {
			return metabase.NewStreamVersionID(randVersion(), testrand.UUID()).Bytes()
		}

		prefixObjectKey := func(prefix []byte, objectKey []byte) []byte {
			key := make([]byte, len(prefix)+len(objectKey)+1)
			copy(key[0:], prefix)
			key[len(prefix)] = '/'
			copy(key[len(prefix)+1:], objectKey)
			return key
		}

		randPrefixedObjectKey := func(prefix []byte) []byte {
			return prefixObjectKey(prefix, []byte(testrand.Path()))
		}

		unversionedBucketName := createBucket(t)

		versionedBucketName := createBucket(t)
		setBucketVersioning(t, versionedBucketName, true)
		enableObjectLock(t, versionedBucketName)

		t.Run("Unversioned", func(t *testing.T) {
			t.Run("Basic", func(t *testing.T) {
				obj1 := createCommittedObject(t, unversionedBucketName)
				obj2 := createCommittedObject(t, unversionedBucketName)

				resp, err := endpoint.DeleteObjects(ctx, &pb.DeleteObjectsRequest{
					Header: apiKeyHeader,
					Bucket: []byte(unversionedBucketName),
					Items: []*pb.DeleteObjectsRequestItem{
						{
							EncryptedObjectKey: obj1.key,
							ObjectVersion:      obj1.version,
						},
						{
							EncryptedObjectKey: obj2.key,
						},
					},
				})
				require.NoError(t, err)

				require.ElementsMatch(t, []*pb.DeleteObjectsResponseItem{
					{
						EncryptedObjectKey:     obj1.key,
						RequestedObjectVersion: obj1.version,
						Removed: &pb.DeleteObjectsResponseItemInfo{
							ObjectVersion: obj1.version,
							Status:        pb.Object_COMMITTED_UNVERSIONED,
						},
						Status: pb.DeleteObjectsResponseItem_OK,
					},
					{
						EncryptedObjectKey: obj2.key,
						Removed: &pb.DeleteObjectsResponseItemInfo{
							ObjectVersion: obj2.version,
							Status:        pb.Object_COMMITTED_UNVERSIONED,
						},
						Status: pb.DeleteObjectsResponseItem_OK,
					},
				}, resp.Items)

				require.False(t, committedObjectExists(t, unversionedBucketName, obj1))
				require.False(t, committedObjectExists(t, unversionedBucketName, obj2))
			})

			t.Run("Not found", func(t *testing.T) {
				object1Key, object2key := []byte(testrand.Path()), []byte(testrand.Path())
				object1Version := randStreamVersionID()

				resp, err := endpoint.DeleteObjects(ctx, &pb.DeleteObjectsRequest{
					Header: apiKeyHeader,
					Bucket: []byte(unversionedBucketName),
					Items: []*pb.DeleteObjectsRequestItem{
						{
							EncryptedObjectKey: object1Key,
							ObjectVersion:      object1Version,
						},
						{
							EncryptedObjectKey: object2key,
						},
					},
				})
				require.NoError(t, err)

				require.ElementsMatch(t, []*pb.DeleteObjectsResponseItem{
					{
						EncryptedObjectKey:     object1Key,
						RequestedObjectVersion: object1Version,
						Status:                 pb.DeleteObjectsResponseItem_NOT_FOUND,
					},
					{
						EncryptedObjectKey: object2key,
						Status:             pb.DeleteObjectsResponseItem_NOT_FOUND,
					},
				}, resp.Items)
			})

			t.Run("Pending object", func(t *testing.T) {
				obj := createPendingObject(t, unversionedBucketName)

				req := &pb.DeleteObjectsRequest{
					Header: apiKeyHeader,
					Bucket: []byte(unversionedBucketName),
					Items: []*pb.DeleteObjectsRequestItem{{
						EncryptedObjectKey: obj.key,
					}},
				}

				resp, err := endpoint.DeleteObjects(ctx, req)
				require.NoError(t, err)

				require.Equal(t, &pb.DeleteObjectsResponse{
					Items: []*pb.DeleteObjectsResponseItem{{
						EncryptedObjectKey: obj.key,
						Status:             pb.DeleteObjectsResponseItem_NOT_FOUND,
					}},
				}, resp)

				require.True(t, pendingObjectExists(t, unversionedBucketName, obj))

				req.Items[0].ObjectVersion = obj.version

				resp, err = endpoint.DeleteObjects(ctx, req)
				require.NoError(t, err)

				require.Equal(t, []*pb.DeleteObjectsResponseItem{{
					EncryptedObjectKey:     obj.key,
					RequestedObjectVersion: obj.version,
					Removed: &pb.DeleteObjectsResponseItemInfo{
						ObjectVersion: obj.version,
						Status:        pb.Object_UPLOADING,
					},
					Status: pb.DeleteObjectsResponseItem_OK,
				}}, resp.Items)

				require.False(t, pendingObjectExists(t, unversionedBucketName, obj))
			})

			t.Run("Duplicate deletion", func(t *testing.T) {
				obj := createCommittedObject(t, unversionedBucketName)

				reqItem := &pb.DeleteObjectsRequestItem{
					EncryptedObjectKey: obj.key,
					ObjectVersion:      obj.version,
				}

				resp, err := endpoint.DeleteObjects(ctx, &pb.DeleteObjectsRequest{
					Header: apiKeyHeader,
					Bucket: []byte(unversionedBucketName),
					Items:  []*pb.DeleteObjectsRequestItem{reqItem, reqItem},
				})
				require.NoError(t, err)

				require.Equal(t, []*pb.DeleteObjectsResponseItem{{
					EncryptedObjectKey:     obj.key,
					RequestedObjectVersion: obj.version,
					Removed: &pb.DeleteObjectsResponseItemInfo{
						ObjectVersion: obj.version,
						Status:        pb.Object_COMMITTED_UNVERSIONED,
					},
					Status: pb.DeleteObjectsResponseItem_OK,
				}}, resp.Items)

				require.False(t, committedObjectExists(t, unversionedBucketName, obj))
			})

			// This tests the case where an object's last committed version is specified
			// in the deletion request both indirectly and explicitly.
			t.Run("Duplicate deletion (indirect)", func(t *testing.T) {
				obj := createCommittedObject(t, unversionedBucketName)

				expectedRemoved := &pb.DeleteObjectsResponseItemInfo{
					ObjectVersion: obj.version,
					Status:        pb.Object_COMMITTED_UNVERSIONED,
				}

				resp, err := endpoint.DeleteObjects(ctx, &pb.DeleteObjectsRequest{
					Header: apiKeyHeader,
					Bucket: []byte(unversionedBucketName),
					Items: []*pb.DeleteObjectsRequestItem{
						{
							EncryptedObjectKey: obj.key,
							ObjectVersion:      obj.version,
						},
						{
							EncryptedObjectKey: obj.key,
						},
					},
				})
				require.NoError(t, err)

				require.ElementsMatch(t, []*pb.DeleteObjectsResponseItem{
					{
						EncryptedObjectKey:     obj.key,
						RequestedObjectVersion: obj.version,
						Removed:                expectedRemoved,
						Status:                 pb.DeleteObjectsResponseItem_OK,
					},
					{
						EncryptedObjectKey: obj.key,
						Removed:            expectedRemoved,
						Status:             pb.DeleteObjectsResponseItem_OK,
					},
				}, resp.Items)

				require.False(t, committedObjectExists(t, unversionedBucketName, obj))
			})
		})

		t.Run("Versioned", func(t *testing.T) {
			t.Run("Basic", func(t *testing.T) {
				obj1 := createCommittedObject(t, versionedBucketName)
				obj2 := createCommittedObject(t, versionedBucketName)

				obj3Key := []byte(testrand.Path())
				obj4Key := []byte(testrand.Path())

				resp, err := endpoint.DeleteObjects(ctx, &pb.DeleteObjectsRequest{
					Header: apiKeyHeader,
					Bucket: []byte(versionedBucketName),
					Items: []*pb.DeleteObjectsRequestItem{
						{
							EncryptedObjectKey: obj1.key,
							ObjectVersion:      obj1.version,
						},
						{
							EncryptedObjectKey: obj2.key,
							ObjectVersion:      obj2.version,
						},
						{
							EncryptedObjectKey: obj3Key,
						},
						{
							EncryptedObjectKey: obj4Key,
						},
					},
				})
				require.NoError(t, err)

				obj3MarkerVersion, obj3MarkerStatus := getLastCommittedVersionAndStatus(t, versionedBucketName, obj3Key)
				require.Equal(t, pb.Object_DELETE_MARKER_VERSIONED, obj3MarkerStatus)

				obj4MarkerVersion, obj4MarkerStatus := getLastCommittedVersionAndStatus(t, versionedBucketName, obj4Key)
				require.Equal(t, pb.Object_DELETE_MARKER_VERSIONED, obj4MarkerStatus)

				require.ElementsMatch(t, []*pb.DeleteObjectsResponseItem{
					{
						EncryptedObjectKey:     obj1.key,
						RequestedObjectVersion: obj1.version,
						Removed: &pb.DeleteObjectsResponseItemInfo{
							ObjectVersion: obj1.version,
							Status:        pb.Object_COMMITTED_VERSIONED,
						},
						Status: pb.DeleteObjectsResponseItem_OK,
					},
					{
						EncryptedObjectKey:     obj2.key,
						RequestedObjectVersion: obj2.version,
						Removed: &pb.DeleteObjectsResponseItemInfo{
							ObjectVersion: obj2.version,
							Status:        pb.Object_COMMITTED_VERSIONED,
						},
						Status: pb.DeleteObjectsResponseItem_OK,
					},
					{
						EncryptedObjectKey: obj3Key,
						Marker: &pb.DeleteObjectsResponseItemInfo{
							ObjectVersion: obj3MarkerVersion,
							Status:        pb.Object_DELETE_MARKER_VERSIONED,
						},
						Status: pb.DeleteObjectsResponseItem_OK,
					},
					{
						EncryptedObjectKey: obj4Key,
						Marker: &pb.DeleteObjectsResponseItemInfo{
							ObjectVersion: obj4MarkerVersion,
							Status:        pb.Object_DELETE_MARKER_VERSIONED,
						},
						Status: pb.DeleteObjectsResponseItem_OK,
					},
				}, resp.Items)

				require.False(t, committedObjectExists(t, versionedBucketName, obj1))
				require.False(t, committedObjectExists(t, versionedBucketName, obj2))
			})

			t.Run("Not found", func(t *testing.T) {
				obj1Key, obj2Key := []byte(testrand.Path()), []byte(testrand.Path())
				obj1Version := randStreamVersionID()

				resp, err := endpoint.DeleteObjects(ctx, &pb.DeleteObjectsRequest{
					Header: apiKeyHeader,
					Bucket: []byte(versionedBucketName),
					Items: []*pb.DeleteObjectsRequestItem{
						{
							EncryptedObjectKey: obj1Key,
							ObjectVersion:      obj1Version,
						},
						{
							EncryptedObjectKey: obj2Key,
						},
					},
				})
				require.NoError(t, err)

				obj2MarkerVersion, obj2MarkerStatus := getLastCommittedVersionAndStatus(t, versionedBucketName, obj2Key)
				require.Equal(t, pb.Object_DELETE_MARKER_VERSIONED, obj2MarkerStatus)

				require.ElementsMatch(t, []*pb.DeleteObjectsResponseItem{
					{
						EncryptedObjectKey:     obj1Key,
						RequestedObjectVersion: obj1Version,
						Status:                 pb.DeleteObjectsResponseItem_NOT_FOUND,
					},
					{
						EncryptedObjectKey: obj2Key,
						Marker: &pb.DeleteObjectsResponseItemInfo{
							ObjectVersion: obj2MarkerVersion,
							Status:        pb.Object_DELETE_MARKER_VERSIONED,
						},
						Status: pb.DeleteObjectsResponseItem_OK,
					},
				}, resp.Items)
			})

			t.Run("Pending object", func(t *testing.T) {
				pending := createPendingObject(t, versionedBucketName)

				req := &pb.DeleteObjectsRequest{
					Header: apiKeyHeader,
					Bucket: []byte(versionedBucketName),
					Items: []*pb.DeleteObjectsRequestItem{{
						EncryptedObjectKey: pending.key,
					}},
				}

				resp, err := endpoint.DeleteObjects(ctx, &pb.DeleteObjectsRequest{
					Header: apiKeyHeader,
					Bucket: []byte(versionedBucketName),
					Items: []*pb.DeleteObjectsRequestItem{{
						EncryptedObjectKey: pending.key,
					}},
				})
				require.NoError(t, err)

				markerVersion, markerStatus := getLastCommittedVersionAndStatus(t, versionedBucketName, pending.key)
				require.Equal(t, pb.Object_DELETE_MARKER_VERSIONED, markerStatus)

				require.Equal(t, []*pb.DeleteObjectsResponseItem{{
					EncryptedObjectKey: pending.key,
					Marker: &pb.DeleteObjectsResponseItemInfo{
						ObjectVersion: markerVersion,
						Status:        pb.Object_DELETE_MARKER_VERSIONED,
					},
					Status: pb.DeleteObjectsResponseItem_OK,
				}}, resp.Items)

				require.True(t, pendingObjectExists(t, versionedBucketName, pending))

				req.Items[0].ObjectVersion = pending.version

				resp, err = endpoint.DeleteObjects(ctx, req)
				require.NoError(t, err)

				require.Equal(t, []*pb.DeleteObjectsResponseItem{{
					EncryptedObjectKey:     pending.key,
					RequestedObjectVersion: pending.version,
					Removed: &pb.DeleteObjectsResponseItemInfo{
						ObjectVersion: pending.version,
						Status:        pb.Object_UPLOADING,
					},
					Status: pb.DeleteObjectsResponseItem_OK,
				}}, resp.Items)

				require.False(t, pendingObjectExists(t, versionedBucketName, pending))
			})

			t.Run("Duplicate deletion", func(t *testing.T) {
				obj1 := createCommittedObject(t, versionedBucketName)
				reqItem1 := &pb.DeleteObjectsRequestItem{
					EncryptedObjectKey: obj1.key,
					ObjectVersion:      obj1.version,
				}

				obj2 := createCommittedObject(t, versionedBucketName)
				reqItem2 := &pb.DeleteObjectsRequestItem{
					EncryptedObjectKey: obj2.key,
				}

				resp, err := endpoint.DeleteObjects(ctx, &pb.DeleteObjectsRequest{
					Header: apiKeyHeader,
					Bucket: []byte(versionedBucketName),
					Items:  []*pb.DeleteObjectsRequestItem{reqItem1, reqItem1, reqItem2, reqItem2},
				})
				require.NoError(t, err)

				markerVersion, markerStatus := getLastCommittedVersionAndStatus(t, versionedBucketName, obj2.key)
				require.Equal(t, pb.Object_DELETE_MARKER_VERSIONED, markerStatus)

				require.ElementsMatch(t, []*pb.DeleteObjectsResponseItem{
					{
						EncryptedObjectKey:     obj1.key,
						RequestedObjectVersion: obj1.version,
						Removed: &pb.DeleteObjectsResponseItemInfo{
							ObjectVersion: obj1.version,
							Status:        pb.Object_COMMITTED_VERSIONED,
						},
						Status: pb.DeleteObjectsResponseItem_OK,
					},
					{
						EncryptedObjectKey: obj2.key,
						Marker: &pb.DeleteObjectsResponseItemInfo{
							ObjectVersion: markerVersion,
							Status:        pb.Object_DELETE_MARKER_VERSIONED,
						},
						Status: pb.DeleteObjectsResponseItem_OK,
					},
				}, resp.Items)

				require.False(t, committedObjectExists(t, versionedBucketName, obj1))
			})

			t.Run("Duplicate deletion (indirect)", func(t *testing.T) {
				obj := createCommittedObject(t, versionedBucketName)

				resp, err := endpoint.DeleteObjects(ctx, &pb.DeleteObjectsRequest{
					Header: apiKeyHeader,
					Bucket: []byte(versionedBucketName),
					Items: []*pb.DeleteObjectsRequestItem{
						{
							EncryptedObjectKey: obj.key,
							ObjectVersion:      obj.version,
						},
						{
							EncryptedObjectKey: obj.key,
						},
					},
				})
				require.NoError(t, err)

				markerVersion, markerStatus := getLastCommittedVersionAndStatus(t, versionedBucketName, obj.key)
				require.Equal(t, pb.Object_DELETE_MARKER_VERSIONED, markerStatus)

				require.ElementsMatch(t, []*pb.DeleteObjectsResponseItem{
					{
						EncryptedObjectKey:     obj.key,
						RequestedObjectVersion: obj.version,
						Removed: &pb.DeleteObjectsResponseItemInfo{
							ObjectVersion: obj.version,
							Status:        pb.Object_COMMITTED_VERSIONED,
						},
						Status: pb.DeleteObjectsResponseItem_OK,
					},
					{
						EncryptedObjectKey: obj.key,
						Marker: &pb.DeleteObjectsResponseItemInfo{
							ObjectVersion: markerVersion,
							Status:        pb.Object_DELETE_MARKER_VERSIONED,
						},
						Status: pb.DeleteObjectsResponseItem_OK,
					},
				}, resp.Items)

				require.False(t, committedObjectExists(t, versionedBucketName, obj))
			})
		})

		t.Run("Suspended", func(t *testing.T) {
			t.Run("Basic", func(t *testing.T) {
				suspendedBucketName := createBucket(t)

				// Insert an unversioned object at this location to ensure that
				// version-omitted deletion removes it and inserts a delete marker
				// as the last version.
				obj1 := createCommittedObject(t, suspendedBucketName)

				setBucketVersioning(t, suspendedBucketName, true)

				// Insert a versioned object at this location to ensure that
				// version-omitted deletion preserves it and inserts a delete marker
				// as the last version.
				obj2 := createCommittedObject(t, suspendedBucketName)

				setBucketVersioning(t, suspendedBucketName, false)

				obj3 := createCommittedObject(t, suspendedBucketName)

				resp, err := endpoint.DeleteObjects(ctx, &pb.DeleteObjectsRequest{
					Header: apiKeyHeader,
					Bucket: []byte(suspendedBucketName),
					Items: []*pb.DeleteObjectsRequestItem{
						{
							EncryptedObjectKey: obj1.key,
						},
						{
							EncryptedObjectKey: obj2.key,
						},
						{
							EncryptedObjectKey: obj3.key,
							ObjectVersion:      obj3.version,
						},
					},
				})
				require.NoError(t, err)

				obj1MarkerVersion, obj1MarkerStatus := getLastCommittedVersionAndStatus(t, suspendedBucketName, obj1.key)
				require.Equal(t, pb.Object_DELETE_MARKER_UNVERSIONED, obj1MarkerStatus)

				obj2MarkerVersion, obj2MarkerStatus := getLastCommittedVersionAndStatus(t, suspendedBucketName, obj2.key)
				require.Equal(t, pb.Object_DELETE_MARKER_UNVERSIONED, obj2MarkerStatus)

				require.ElementsMatch(t, []*pb.DeleteObjectsResponseItem{
					{
						EncryptedObjectKey: obj1.key,
						Removed: &pb.DeleteObjectsResponseItemInfo{
							ObjectVersion: obj1.version,
							Status:        pb.Object_COMMITTED_UNVERSIONED,
						},
						Marker: &pb.DeleteObjectsResponseItemInfo{
							ObjectVersion: obj1MarkerVersion,
							Status:        pb.Object_DELETE_MARKER_UNVERSIONED,
						},
						Status: pb.DeleteObjectsResponseItem_OK,
					},
					{
						EncryptedObjectKey: obj2.key,
						Marker: &pb.DeleteObjectsResponseItemInfo{
							ObjectVersion: obj2MarkerVersion,
							Status:        pb.Object_DELETE_MARKER_UNVERSIONED,
						},
						Status: pb.DeleteObjectsResponseItem_OK,
					},
					{
						EncryptedObjectKey:     obj3.key,
						RequestedObjectVersion: obj3.version,
						Removed: &pb.DeleteObjectsResponseItemInfo{
							ObjectVersion: obj3.version,
							Status:        pb.Object_COMMITTED_UNVERSIONED,
						},
						Status: pb.DeleteObjectsResponseItem_OK,
					},
				}, resp.Items)
			})

			suspendedBucketName := createBucket(t)
			setBucketVersioning(t, suspendedBucketName, true)
			setBucketVersioning(t, suspendedBucketName, false)

			t.Run("Not found", func(t *testing.T) {
				object1Key, object2Key := []byte(testrand.Path()), []byte(testrand.Path())
				object1Version := randStreamVersionID()

				resp, err := endpoint.DeleteObjects(ctx, &pb.DeleteObjectsRequest{
					Header: apiKeyHeader,
					Bucket: []byte(suspendedBucketName),
					Items: []*pb.DeleteObjectsRequestItem{
						{
							EncryptedObjectKey: object1Key,
							ObjectVersion:      object1Version,
						},
						{
							EncryptedObjectKey: object2Key,
						},
					},
				})
				require.NoError(t, err)

				require.ElementsMatch(t, []*pb.DeleteObjectsResponseItem{
					{
						EncryptedObjectKey:     object1Key,
						RequestedObjectVersion: object1Version,
						Status:                 pb.DeleteObjectsResponseItem_NOT_FOUND,
					},
					{
						EncryptedObjectKey: object2Key,
						Status:             pb.DeleteObjectsResponseItem_NOT_FOUND,
					},
				}, resp.Items)
			})

			t.Run("Pending object", func(t *testing.T) {
				obj := createPendingObject(t, suspendedBucketName)

				req := &pb.DeleteObjectsRequest{
					Header: apiKeyHeader,
					Bucket: []byte(suspendedBucketName),
					Items: []*pb.DeleteObjectsRequestItem{{
						EncryptedObjectKey: obj.key,
					}},
				}

				resp, err := endpoint.DeleteObjects(ctx, req)
				require.NoError(t, err)

				require.Equal(t, []*pb.DeleteObjectsResponseItem{{
					EncryptedObjectKey: obj.key,
					Status:             pb.DeleteObjectsResponseItem_NOT_FOUND,
				}}, resp.Items)

				require.True(t, pendingObjectExists(t, suspendedBucketName, obj))

				req.Items[0].ObjectVersion = obj.version

				resp, err = endpoint.DeleteObjects(ctx, req)
				require.NoError(t, err)

				require.Equal(t, []*pb.DeleteObjectsResponseItem{{
					EncryptedObjectKey:     obj.key,
					RequestedObjectVersion: obj.version,
					Removed: &pb.DeleteObjectsResponseItemInfo{
						ObjectVersion: obj.version,
						Status:        pb.Object_UPLOADING,
					},
					Status: pb.DeleteObjectsResponseItem_OK,
				}}, resp.Items)

				require.False(t, pendingObjectExists(t, suspendedBucketName, obj))
			})

			t.Run("Duplicate deletion", func(t *testing.T) {
				obj1 := createCommittedObject(t, suspendedBucketName)
				reqItem1 := &pb.DeleteObjectsRequestItem{
					EncryptedObjectKey: obj1.key,
					ObjectVersion:      obj1.version,
				}

				obj2 := createCommittedObject(t, suspendedBucketName)
				reqItem2 := &pb.DeleteObjectsRequestItem{
					EncryptedObjectKey: obj2.key,
				}

				resp, err := endpoint.DeleteObjects(ctx, &pb.DeleteObjectsRequest{
					Header: apiKeyHeader,
					Bucket: []byte(suspendedBucketName),
					Items:  []*pb.DeleteObjectsRequestItem{reqItem1, reqItem1, reqItem2, reqItem2},
				})
				require.NoError(t, err)

				markerVersion, markerStatus := getLastCommittedVersionAndStatus(t, suspendedBucketName, obj2.key)
				require.Equal(t, pb.Object_DELETE_MARKER_UNVERSIONED, markerStatus)

				require.ElementsMatch(t, []*pb.DeleteObjectsResponseItem{
					{
						EncryptedObjectKey:     obj1.key,
						RequestedObjectVersion: obj1.version,
						Removed: &pb.DeleteObjectsResponseItemInfo{
							ObjectVersion: obj1.version,
							Status:        pb.Object_COMMITTED_UNVERSIONED,
						},
						Status: pb.DeleteObjectsResponseItem_OK,
					},
					{
						EncryptedObjectKey: obj2.key,
						Removed: &pb.DeleteObjectsResponseItemInfo{
							ObjectVersion: obj2.version,
							Status:        pb.Object_COMMITTED_UNVERSIONED,
						},
						Marker: &pb.DeleteObjectsResponseItemInfo{
							ObjectVersion: markerVersion,
							Status:        pb.Object_DELETE_MARKER_UNVERSIONED,
						},
						Status: pb.DeleteObjectsResponseItem_OK,
					},
				}, resp.Items)

				require.False(t, committedObjectExists(t, suspendedBucketName, obj1))
				require.False(t, committedObjectExists(t, suspendedBucketName, obj2))
			})

			t.Run("Duplicate deletion (indirect)", func(t *testing.T) {
				obj := createCommittedObject(t, suspendedBucketName)

				resp, err := endpoint.DeleteObjects(ctx, &pb.DeleteObjectsRequest{
					Header: apiKeyHeader,
					Bucket: []byte(suspendedBucketName),
					Items: []*pb.DeleteObjectsRequestItem{
						{
							EncryptedObjectKey: obj.key,
							ObjectVersion:      obj.version,
						},
						{
							EncryptedObjectKey: obj.key,
						},
					},
				})
				require.NoError(t, err)

				markerVersion, markerStatus := getLastCommittedVersionAndStatus(t, suspendedBucketName, obj.key)
				require.Equal(t, pb.Object_DELETE_MARKER_UNVERSIONED, markerStatus)

				require.ElementsMatch(t, []*pb.DeleteObjectsResponseItem{
					{
						EncryptedObjectKey:     obj.key,
						RequestedObjectVersion: obj.version,
						Removed: &pb.DeleteObjectsResponseItemInfo{
							ObjectVersion: obj.version,
							Status:        pb.Object_COMMITTED_UNVERSIONED,
						},
						Status: pb.DeleteObjectsResponseItem_OK,
					},
					{
						EncryptedObjectKey: obj.key,
						Removed: &pb.DeleteObjectsResponseItemInfo{
							ObjectVersion: obj.version,
							Status:        pb.Object_COMMITTED_UNVERSIONED,
						},
						Marker: &pb.DeleteObjectsResponseItemInfo{
							ObjectVersion: markerVersion,
							Status:        pb.Object_DELETE_MARKER_UNVERSIONED,
						},
						Status: pb.DeleteObjectsResponseItem_OK,
					},
				}, resp.Items)

				require.False(t, committedObjectExists(t, suspendedBucketName, obj))
			})
		})

		t.Run("Object Lock", func(t *testing.T) {
			metabasetest.ObjectLockDeletionTestRunner{
				TestProtected: func(t *testing.T, testCase metabasetest.ObjectLockDeletionTestCase) {
					obj := createLockedCommittedObject(t, versionedBucketName, testCase.Retention, testCase.LegalHold)

					resp, err := endpoint.DeleteObjects(ctx, &pb.DeleteObjectsRequest{
						Header:                    apiKeyHeader,
						Bucket:                    []byte(versionedBucketName),
						BypassGovernanceRetention: testCase.BypassGovernance,
						Items: []*pb.DeleteObjectsRequestItem{{
							EncryptedObjectKey: obj.key,
							ObjectVersion:      obj.version,
						}},
					})
					require.NoError(t, err)

					require.Equal(t, []*pb.DeleteObjectsResponseItem{{
						EncryptedObjectKey:     obj.key,
						RequestedObjectVersion: obj.version,
						Status:                 pb.DeleteObjectsResponseItem_LOCKED,
					}}, resp.Items)

					require.True(t, committedObjectExists(t, versionedBucketName, obj))
				},
				TestRemovable: func(t *testing.T, testCase metabasetest.ObjectLockDeletionTestCase) {
					obj := createLockedCommittedObject(t, versionedBucketName, testCase.Retention, testCase.LegalHold)

					resp, err := endpoint.DeleteObjects(ctx, &pb.DeleteObjectsRequest{
						Header:                    apiKeyHeader,
						Bucket:                    []byte(versionedBucketName),
						BypassGovernanceRetention: testCase.BypassGovernance,
						Items: []*pb.DeleteObjectsRequestItem{{
							EncryptedObjectKey: obj.key,
							ObjectVersion:      obj.version,
						}},
					})
					require.NoError(t, err)

					require.Equal(t, []*pb.DeleteObjectsResponseItem{{
						EncryptedObjectKey:     obj.key,
						RequestedObjectVersion: obj.version,
						Removed: &pb.DeleteObjectsResponseItemInfo{
							ObjectVersion: obj.version,
							Status:        pb.Object_COMMITTED_VERSIONED,
						},
						Status: pb.DeleteObjectsResponseItem_OK,
					}}, resp.Items)

					require.False(t, committedObjectExists(t, versionedBucketName, obj))
				},
			}.Run(t)
		})

		t.Run("Quiet mode", func(t *testing.T) {
			prefix := []byte(testrand.Path())

			restrictedAPIKey, err := apiKey.Restrict(macaroon.Caveat{
				AllowedPaths: []*macaroon.Caveat_Path{{
					Bucket:              []byte(versionedBucketName),
					EncryptedPathPrefix: prefix,
				}},
			})
			require.NoError(t, err)

			obj := createCommittedObjectWithKey(t, versionedBucketName, randPrefixedObjectKey(prefix))
			lockedObj := createLockedCommittedObjectWithKey(t, versionedBucketName, randPrefixedObjectKey(prefix), metabase.Retention{}, true)
			unauthorizedObj := createCommittedObject(t, versionedBucketName)
			notFoundObj := minimalObject{
				key:     randPrefixedObjectKey(prefix),
				version: randStreamVersionID(),
			}

			resp, err := endpoint.DeleteObjects(ctx, &pb.DeleteObjectsRequest{
				Header: &pb.RequestHeader{ApiKey: restrictedAPIKey.SerializeRaw()},
				Bucket: []byte(versionedBucketName),
				Quiet:  true,
				Items: []*pb.DeleteObjectsRequestItem{
					{
						EncryptedObjectKey: obj.key,
						ObjectVersion:      obj.version,
					},
					{
						EncryptedObjectKey: unauthorizedObj.key,
						ObjectVersion:      unauthorizedObj.version,
					},
					{
						EncryptedObjectKey: notFoundObj.key,
						ObjectVersion:      notFoundObj.version,
					},
					{
						EncryptedObjectKey: lockedObj.key,
						ObjectVersion:      lockedObj.version,
					},
				},
			})
			require.NoError(t, err)

			require.ElementsMatch(t, []*pb.DeleteObjectsResponseItem{
				{
					EncryptedObjectKey:     unauthorizedObj.key,
					RequestedObjectVersion: unauthorizedObj.version,
					Status:                 pb.DeleteObjectsResponseItem_UNAUTHORIZED,
				},
				{
					EncryptedObjectKey:     notFoundObj.key,
					RequestedObjectVersion: notFoundObj.version,
					Status:                 pb.DeleteObjectsResponseItem_NOT_FOUND,
				},
				{
					EncryptedObjectKey:     lockedObj.key,
					RequestedObjectVersion: lockedObj.version,
					Status:                 pb.DeleteObjectsResponseItem_LOCKED,
				},
			}, resp.Items)

			internalErrorObj := minimalObject{
				key:     prefixObjectKey(prefix, []byte(errorTriggerObjectKey)),
				version: randStreamVersionID(),
			}

			resp, err = endpoint.DeleteObjects(ctx, &pb.DeleteObjectsRequest{
				Header: &pb.RequestHeader{ApiKey: restrictedAPIKey.SerializeRaw()},
				Bucket: []byte(versionedBucketName),
				Quiet:  true,
				Items: []*pb.DeleteObjectsRequestItem{{
					EncryptedObjectKey: internalErrorObj.key,
					ObjectVersion:      internalErrorObj.version,
				}},
			})
			require.NoError(t, err)

			require.Equal(t, []*pb.DeleteObjectsResponseItem{{
				EncryptedObjectKey:     internalErrorObj.key,
				RequestedObjectVersion: internalErrorObj.version,
				Status:                 pb.DeleteObjectsResponseItem_INTERNAL_ERROR,
			}}, resp.Items)
		})

		t.Run("Missing bucket", func(t *testing.T) {
			resp, err := endpoint.DeleteObjects(ctx, &pb.DeleteObjectsRequest{
				Header: apiKeyHeader,
				Bucket: []byte(testrand.BucketName()),
				Items: []*pb.DeleteObjectsRequestItem{{
					EncryptedObjectKey: []byte(testrand.Path()),
					ObjectVersion:      randStreamVersionID(),
				}},
			})
			require.Nil(t, resp)
			rpctest.RequireCode(t, err, rpcstatus.BucketNotFound)
		})

		t.Run("Unauthorized API key", func(t *testing.T) {
			t.Run("Prefix restriction", func(t *testing.T) {
				prefix := []byte(testrand.Path())

				restrictedAPIKey, err := apiKey.Restrict(macaroon.Caveat{
					AllowedPaths: []*macaroon.Caveat_Path{{
						Bucket:              []byte(unversionedBucketName),
						EncryptedPathPrefix: prefix,
					}},
				})
				require.NoError(t, err)

				restrictedObj := createCommittedObject(t, unversionedBucketName)
				allowedObj := createCommittedObjectWithKey(t, unversionedBucketName, randPrefixedObjectKey(prefix))

				resp, err := endpoint.DeleteObjects(ctx, &pb.DeleteObjectsRequest{
					Header: &pb.RequestHeader{ApiKey: restrictedAPIKey.SerializeRaw()},
					Bucket: []byte(unversionedBucketName),
					Items: []*pb.DeleteObjectsRequestItem{
						{
							EncryptedObjectKey: allowedObj.key,
							ObjectVersion:      allowedObj.version,
						},
						{
							EncryptedObjectKey: restrictedObj.key,
							ObjectVersion:      restrictedObj.version,
						},
					},
				})
				require.NoError(t, err)

				require.ElementsMatch(t, []*pb.DeleteObjectsResponseItem{
					{
						EncryptedObjectKey:     allowedObj.key,
						RequestedObjectVersion: allowedObj.version,
						Removed: &pb.DeleteObjectsResponseItemInfo{
							ObjectVersion: allowedObj.version,
							Status:        pb.Object_COMMITTED_UNVERSIONED,
						},
						Status: pb.DeleteObjectsResponseItem_OK,
					},
					{
						EncryptedObjectKey:     restrictedObj.key,
						RequestedObjectVersion: restrictedObj.version,
						Status:                 pb.DeleteObjectsResponseItem_UNAUTHORIZED,
					},
				}, resp.Items)

				require.True(t, committedObjectExists(t, unversionedBucketName, restrictedObj))
				require.False(t, committedObjectExists(t, unversionedBucketName, allowedObj))
			})

			t.Run("Bucket restriction", func(t *testing.T) {
				restrictedAPIKey, err := apiKey.Restrict(macaroon.Caveat{
					AllowedPaths: []*macaroon.Caveat_Path{{
						Bucket: []byte(testrand.BucketName()),
					}},
				})
				require.NoError(t, err)

				restrictedAPIKeyHeader := &pb.RequestHeader{
					ApiKey: restrictedAPIKey.SerializeRaw(),
				}

				obj := createCommittedObject(t, unversionedBucketName)

				req := &pb.DeleteObjectsRequest{
					Header: restrictedAPIKeyHeader,
					Bucket: []byte(unversionedBucketName),
					Items: []*pb.DeleteObjectsRequestItem{{
						EncryptedObjectKey: obj.key,
						ObjectVersion:      obj.version,
					}},
				}

				expectedResp := &pb.DeleteObjectsResponse{
					Items: []*pb.DeleteObjectsResponseItem{{
						EncryptedObjectKey:     obj.key,
						RequestedObjectVersion: obj.version,
						Status:                 pb.DeleteObjectsResponseItem_UNAUTHORIZED,
					}},
				}

				resp, err := endpoint.DeleteObjects(ctx, req)
				require.NoError(t, err)
				require.Equal(t, expectedResp, resp)

				// Ensure that we respond the same way for a nonexistent bucket.
				req.Bucket = []byte(testrand.BucketName())
				resp, err = endpoint.DeleteObjects(ctx, req)
				require.NoError(t, err)
				require.Equal(t, expectedResp, resp)

				require.True(t, committedObjectExists(t, unversionedBucketName, obj))
			})

			t.Run("No delete permission", func(t *testing.T) {
				restrictedAPIKey, err := apiKey.Restrict(macaroon.Caveat{
					DisallowDeletes: true,
				})
				require.NoError(t, err)

				obj := createCommittedObject(t, unversionedBucketName)

				resp, err := endpoint.DeleteObjects(ctx, &pb.DeleteObjectsRequest{
					Header: &pb.RequestHeader{ApiKey: restrictedAPIKey.SerializeRaw()},
					Bucket: []byte(unversionedBucketName),
					Items: []*pb.DeleteObjectsRequestItem{{
						EncryptedObjectKey: obj.key,
						ObjectVersion:      obj.version,
					}},
				})
				require.NoError(t, err)

				require.Equal(t, &pb.DeleteObjectsResponse{
					Items: []*pb.DeleteObjectsResponseItem{{
						EncryptedObjectKey:     obj.key,
						RequestedObjectVersion: obj.version,
						Status:                 pb.DeleteObjectsResponseItem_UNAUTHORIZED,
					}},
				}, resp)

				require.True(t, committedObjectExists(t, unversionedBucketName, obj))
			})

			t.Run("No governance bypass permission", func(t *testing.T) {
				test := func(t *testing.T, apiKey *macaroon.APIKey) {
					obj := createCommittedObject(t, versionedBucketName)

					lockedObj := createLockedCommittedObject(t, versionedBucketName, metabase.Retention{
						Mode:        storj.GovernanceMode,
						RetainUntil: time.Now().Add(time.Hour),
					}, false)

					resp, err := endpoint.DeleteObjects(ctx, &pb.DeleteObjectsRequest{
						Header:                    &pb.RequestHeader{ApiKey: apiKey.SerializeRaw()},
						Bucket:                    []byte(versionedBucketName),
						BypassGovernanceRetention: true,
						Items: []*pb.DeleteObjectsRequestItem{
							{
								EncryptedObjectKey: lockedObj.key,
								ObjectVersion:      lockedObj.version,
							},
							{
								EncryptedObjectKey: obj.key,
								ObjectVersion:      obj.version,
							},
						},
					})
					require.NoError(t, err)

					require.ElementsMatch(t, []*pb.DeleteObjectsResponseItem{
						{
							EncryptedObjectKey:     lockedObj.key,
							RequestedObjectVersion: lockedObj.version,
							Status:                 pb.DeleteObjectsResponseItem_UNAUTHORIZED,
						},
						{
							// Ensure that we return UNAUTHORIZED for all objects
							// as opposed to just the objects that are locked.
							EncryptedObjectKey:     obj.key,
							RequestedObjectVersion: obj.version,
							Status:                 pb.DeleteObjectsResponseItem_UNAUTHORIZED,
						},
					}, resp.Items)

					require.True(t, committedObjectExists(t, versionedBucketName, lockedObj))
					require.True(t, committedObjectExists(t, versionedBucketName, obj))
				}

				t.Run("Old API key", func(t *testing.T) {
					_, apiKey, err := sat.API.Console.Service.CreateAPIKey(userCtx, project.ID, "old key", macaroon.APIKeyVersionMin)
					require.NoError(t, err)
					test(t, apiKey)
				})

				t.Run("Restricted API key", func(t *testing.T) {
					restrictedAPIKey, err := apiKey.Restrict(macaroon.Caveat{
						DisallowBypassGovernanceRetention: true,
					})
					require.NoError(t, err)
					test(t, restrictedAPIKey)
				})
			})
		})

		t.Run("Invalid request", func(t *testing.T) {
			t.Run("No items", func(t *testing.T) {
				_, err := endpoint.DeleteObjects(ctx, &pb.DeleteObjectsRequest{
					Header: apiKeyHeader,
					Bucket: []byte(unversionedBucketName),
				})
				rpctest.RequireCode(t, err, rpcstatus.DeleteObjectsNoItems)
			})

			t.Run("Too many items", func(t *testing.T) {
				obj := createCommittedObject(t, unversionedBucketName)

				items := make([]*pb.DeleteObjectsRequestItem, metabase.DeleteObjectsMaxItems+1)
				for i := 0; i < len(items); i++ {
					items[i] = &pb.DeleteObjectsRequestItem{
						EncryptedObjectKey: obj.key,
						ObjectVersion:      obj.version,
					}
				}

				_, err := endpoint.DeleteObjects(ctx, &pb.DeleteObjectsRequest{
					Header: apiKeyHeader,
					Bucket: []byte(unversionedBucketName),
					Items:  items,
				})
				rpctest.RequireCode(t, err, rpcstatus.DeleteObjectsTooManyItems)

				require.True(t, committedObjectExists(t, unversionedBucketName, obj))
			})

			t.Run("Invalid object key", func(t *testing.T) {
				objectKey := testrand.Bytes(memory.Size(sat.Config.Metainfo.MaxEncryptedObjectKeyLength + 1))
				_, err := endpoint.DeleteObjects(ctx, &pb.DeleteObjectsRequest{
					Header: apiKeyHeader,
					Bucket: []byte(unversionedBucketName),
					Items: []*pb.DeleteObjectsRequestItem{{
						EncryptedObjectKey: objectKey,
					}},
				})
				rpctest.RequireCode(t, err, rpcstatus.ObjectKeyTooLong)
			})

			t.Run("Invalid object version", func(t *testing.T) {
				for _, tt := range []struct {
					name    string
					version []byte
				}{
					{name: "Too short", version: randStreamVersionID()[1:]},
					{name: "Zero internal version ID", version: metabase.NewStreamVersionID(0, testrand.UUID()).Bytes()},
				} {
					_, err := endpoint.DeleteObjects(ctx, &pb.DeleteObjectsRequest{
						Header: apiKeyHeader,
						Bucket: []byte(unversionedBucketName),
						Items: []*pb.DeleteObjectsRequestItem{{
							EncryptedObjectKey: []byte(testrand.Path()),
							ObjectVersion:      tt.version,
						}},
					})
					rpctest.RequireCode(t, err, rpcstatus.ObjectVersionInvalid)
				}
			})
		})
	})
}

func TestEndpoint_DeleteObjectsDisabled(t *testing.T) {
	testplanet.Run(t, testplanet.Config{
		SatelliteCount: 1, UplinkCount: 1,
		Reconfigure: testplanet.Reconfigure{
			Satellite: func(log *zap.Logger, index int, config *satellite.Config) {
				config.Metainfo.DeleteObjectsEnabled = false
			},
		},
	}, func(t *testing.T, ctx *testcontext.Context, planet *testplanet.Planet) {
		sat := planet.Satellites[0]
		project := planet.Uplinks[0].Projects[0]
		endpoint := sat.Metainfo.Endpoint

		userCtx, err := sat.UserContext(ctx, project.Owner.ID)
		require.NoError(t, err)

		_, apiKey, err := sat.API.Console.Service.CreateAPIKey(userCtx, project.ID, "test key", macaroon.APIKeyVersionObjectLock)
		require.NoError(t, err)

		_, err = endpoint.DeleteObjects(ctx, &pb.DeleteObjectsRequest{
			Header: &pb.RequestHeader{
				ApiKey: apiKey.SerializeRaw(),
			},
			Bucket: randomBucketName,
			Items: []*pb.DeleteObjectsRequestItem{{
				EncryptedObjectKey: randomEncryptedKey,
				ObjectVersion:      metabase.NewStreamVersionID(randVersion(), testrand.UUID()).Bytes(),
			}},
		})
		rpctest.RequireCode(t, err, rpcstatus.Unimplemented)
	})
}

var _ metabase.Adapter = (*deleteObjectsTestAdapter)(nil)

type deleteObjectsTestAdapter struct {
	metabase.Adapter
	errorTriggerObjectKey metabase.ObjectKey
}

func (adapter *deleteObjectsTestAdapter) DeleteObjectExactVersion(ctx context.Context, opts metabase.DeleteObjectExactVersion) (metabase.DeleteObjectResult, error) {
	if opts.ObjectKey == adapter.errorTriggerObjectKey || strings.HasSuffix(string(opts.ObjectKey), "/"+string(adapter.errorTriggerObjectKey)) {
		return metabase.DeleteObjectResult{}, errors.New("internal error")
	}
	return adapter.Adapter.DeleteObjectExactVersion(ctx, opts)
}
