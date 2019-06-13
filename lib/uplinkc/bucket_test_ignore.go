// +build ignore

// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package main

import (
	"fmt"
	"reflect"
	"storj.io/storj/internal/testplanet"
	"testing"
	"time"
	"unsafe"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"storj.io/storj/internal/testcontext"
	"storj.io/storj/lib/uplink"
)

func Test_create_bucket(t *testing.T) {
	// TODO: figure this out (there may be other inconsistencies as well)
	t.Log("listed bucket *always* has `PathCipher` = `AESGCM`; is this expected behavior?")
	t.SkipNow()

	RunPlanet(t, func(ctx *testcontext.Context, planet *testplanet.Planet) {
		var cErr Cchar
		bucketName := "TestBucket"
		project, cProjectRef := openTestProject(t, ctx, planet)

		testEachBucketConfig(t, func(bucketCfg *uplink.BucketConfig) {
			cBucketConfig := NewCBucketConfig(bucketCfg)
			cBucket := create_bucket(cProjectRef, stringToCCharPtr(bucketName), &cBucketConfig, &cErr)
			require.Empty(t, cCharToGoString(cErr))
			require.NotNil(t, cBucket)

			bucketList, err := project.ListBuckets(ctx, nil)
			require.NoError(t, err)

			expectedBucket := bucketList.Items[0]
			goBucket := newGoBucket(&cBucket)

			assert.True(t, reflect.DeepEqual(expectedBucket, goBucket))

			err = project.DeleteBucket(ctx, bucketName)
			require.NoError(t, err)
		})
	})
}

func Test_open_bucket(t *testing.T) {
	RunPlanet(t, func(ctx *testcontext.Context, planet *testplanet.Planet) {
		var cErr Cchar
		bucketName := "TestBucket"
		project, cProjectRef := openTestProject(t, ctx, planet)

		testEachBucketConfig(t, func(bucketCfg *uplink.BucketConfig) {
			bucket, err := project.CreateBucket(ctx, bucketName, bucketCfg)
			require.NoError(t, err)
			require.NotNil(t, bucket)

			expectedBucket, err := project.OpenBucket(ctx, bucketName, nil)
			require.NoError(t, err)
			require.NotNil(t, expectedBucket)

			cBucketRef := open_bucket(cProjectRef, stringToCCharPtr(bucketName), nil, &cErr)
			require.Empty(t, cCharToGoString(cErr))
			require.NotEmpty(t, cBucketRef)

			goBucket, ok := structRefMap.Get(token(cBucketRef)).(*uplink.Bucket)
			require.True(t, ok)
			require.NotNil(t, goBucket)

			assert.True(t, reflect.DeepEqual(expectedBucket, goBucket))
		})
	})
}

func Test_delete_bucket(t *testing.T) {
	RunPlanet(t, func(ctx *testcontext.Context, planet *testplanet.Planet) {
		var cErr Cchar
		bucketName := "TestBucket"
		project, cProjectRef := openTestProject(t, ctx, planet)

		testEachBucketConfig(t, func(bucketCfg *uplink.BucketConfig) {
			bucket, err := project.CreateBucket(ctx, bucketName, bucketCfg)
			require.NoError(t, err)
			require.NotNil(t, bucket)

			delete_bucket(cProjectRef, stringToCCharPtr(bucketName), &cErr)
			require.Empty(t, cCharToGoString(cErr))
		})
	})
}

func Test_list_buckets(t *testing.T) {
	RunPlanet(t, func(ctx *testcontext.Context, planet *testplanet.Planet) {
		var cErr Cchar
		project, cProjectRef := openTestProject(t, ctx, planet)

		bucketCount := 15
		testEachBucketConfig(t, func(bucketCfg *uplink.BucketConfig) {
			for i := 0; i < bucketCount; i++ {
				bucketName := fmt.Sprintf("TestBucket%d", i)
				_, err := project.CreateBucket(ctx, bucketName, bucketCfg)
				require.NoError(t, err)
			}

			// TODO: test with different list options
			cBucketList := list_buckets(cProjectRef, nil, &cErr)
			require.Empty(t, cCharToGoString(cErr))
			require.NotNil(t, cBucketList)
			require.NotNil(t, cBucketList.items)
			require.Equal(t, int(cBucketList.length), bucketCount)

			bucketList, err := project.ListBuckets(ctx, nil)
			require.NoError(t, err)
			require.Len(t, bucketList.Items, bucketCount)

			assert.Equal(t, bucketList.More, bool(cBucketList.more))
			//TODO: test with `more` being true

			// Compare buckets
			bucketSize := int(unsafe.Sizeof(CBucket{}))
			for i, bucket := range bucketList.Items {
				itemsAddress := uintptr(unsafe.Pointer(cBucketList.items))
				nextAddress := uintptr(int(itemsAddress) + (i * bucketSize))
				cBucket := (*CBucket)(unsafe.Pointer(nextAddress))
				require.NotNil(t, cBucket)
				require.NotEmpty(t, cBucket.name)

				reflect.DeepEqual(bucket, newGoBucket(cBucket))
			}
		})
	})
}

func TestGetBucketInfo(t *testing.T) {
	RunPlanet(t, func(ctx *testcontext.Context, planet *testplanet.Planet) {
		var cErr Cchar
		project, cProjectRef := openTestProject(t, ctx, planet)

		bucketCount := 15
		testEachBucketConfig(t, func(bucketCfg *uplink.BucketConfig) {
			for i := 0; i < bucketCount; i++ {
				bucketName := fmt.Sprintf("TestBucket%d", i)
				_, err := project.CreateBucket(ctx, bucketName, bucketCfg)
				require.NoError(t, err)

				bucket, bucketConfig, err := project.GetBucketInfo(ctx, bucketName)
				require.NoError(t, err)
				require.NotEmpty(t, bucket)
				require.NotEmpty(t, bucketConfig)

				// NB (workaround): timezones are different
				bucket.Created = time.Unix(bucket.Created.Unix(), 0).UTC()
				// NB: c structs ignore `Volatile` fields; set to zero value for comparison
				bucketConfig.Volatile = uplink.BucketConfig{}.Volatile

				cBucketInfo := GetBucketInfo(cProjectRef, stringToCCharPtr(bucketName), &cErr)
				cConfig, cBucket := cBucketInfo.config, cBucketInfo.bucket

				assert.True(t, reflect.DeepEqual(bucket, newGoBucket(&cBucket)))
				assert.True(t, reflect.DeepEqual(*bucketConfig, newGoBucketConfig(&cConfig)))
			}
		})
	})
}

func Testclose_project(t *testing.T) {
	RunPlanet(t, func(ctx *testcontext.Context, planet *testplanet.Planet) {
		var cErr Cchar
		_, cProjectRef := openTestProject(t, ctx, planet)

		close_project(cProjectRef, &cErr)
		require.Empty(t, cCharToGoString(cErr))
	})
}


func TestProject(t *testing.T) {
	RunPlanet(t, func(ctx *testcontext.Context, planet *testplanet.Planet) {
		satelliteAddr := planet.Satellites[0].Addr()
		apikeyStr := planet.Uplinks[0].APIKey[planet.Satellites[0].ID()]

		{
			var config CUplinkConfig
			config.Volatile.TLS.SkipPeerCAWhitelist = Cbool(true)

			var cerr Cpchar
			uplink := new_uplink(config, &cerr)
			require.Nil(t, cerr)
			require.NotEmpty(t, uplink)

			defer func() {
				close_uplink(uplink, &cerr)
				require.Nil(t, cerr)
			}()

			{
				capikeyStr := CString(apikeyStr)
				defer CFree(unsafe.Pointer(capikeyStr))

				apikey := parse_api_key(capikeyStr, &cerr)
				require.Nil(t, cerr)
				require.NotEmpty(t, apikey)
				defer free_api_key(apikey)

				cSatelliteAddr := CString(satelliteAddr)
				defer CFree(unsafe.Pointer(cSatelliteAddr))

				project := open_project(uplink, cSatelliteAddr, apikey, &cerr)
				require.Nil(t, cerr)
				require.NotEmpty(t, uplink)

				defer func() {
					close_project(project, &cerr)
					require.Nil(t, cerr)
				}()
			}
		}
	})
}
