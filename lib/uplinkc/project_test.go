// +build ignore

package main

import (
	"fmt"
	"reflect"
	"testing"
	"time"
	"unsafe"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"storj.io/storj/internal/testcontext"
	"storj.io/storj/lib/uplink"
)

func TestCreateBucket(t *testing.T) {
	// TODO: figure this out (there may be other inconsistencies as well)
	t.Log("listed bucket *always* has `PathCipher` = `AESGCM`; is this expected behavior?")
	t.SkipNow()

	ctx := testcontext.New(t)
	defer ctx.Cleanup()

	planet := startTestPlanet(t, ctx)
	defer ctx.Check(planet.Shutdown)

	var cErr CCharPtr
	bucketName := "TestBucket"
	project, cProjectRef := openTestProject(t, ctx, planet)

	testEachBucketConfig(t, func(bucketCfg *uplink.BucketConfig) {
		cBucketConfig := NewCBucketConfig(bucketCfg)
		cBucket := CreateBucket(cProjectRef, stringToCCharPtr(bucketName), &cBucketConfig, &cErr)
		require.Empty(t, cCharToGoString(cErr))
		require.NotNil(t, cBucket)

		// TODO: test with different options
		bucketList, err := project.ListBuckets(ctx, nil)
		require.NoError(t, err)

		expectedBucket := bucketList.Items[0]
		goBucket := newGoBucket(&cBucket)

		assert.True(t, reflect.DeepEqual(expectedBucket, goBucket))

		err = project.DeleteBucket(ctx, bucketName)
		require.NoError(t, err)
	})
}

func TestOpenBucket(t *testing.T) {
	ctx := testcontext.New(t)
	defer ctx.Cleanup()

	planet := startTestPlanet(t, ctx)
	defer ctx.Check(planet.Shutdown)

	var cErr CCharPtr
	bucketName := "TestBucket"
	project, cProjectRef := openTestProject(t, ctx, planet)

	testEachBucketConfig(t, func(bucketCfg *uplink.BucketConfig) {
		bucket, err := project.CreateBucket(ctx, bucketName, bucketCfg)
		require.NoError(t, err)
		require.NotNil(t, bucket)

		expectedBucket, err := project.OpenBucket(ctx, bucketName, nil)
		require.NoError(t, err)
		require.NotNil(t, expectedBucket)

		cBucketRef := OpenBucket(cProjectRef, stringToCCharPtr(bucketName), nil, &cErr)
		require.Empty(t, cCharToGoString(cErr))
		require.NotEmpty(t, cBucketRef)

		goBucket, ok := universe.Get(Token(cBucketRef)).(*uplink.Bucket)
		require.True(t, ok)
		require.NotNil(t, goBucket)

		assert.True(t, reflect.DeepEqual(expectedBucket, goBucket))
	})
}

func TestDeleteBucket(t *testing.T) {
	ctx := testcontext.New(t)
	defer ctx.Cleanup()

	planet := startTestPlanet(t, ctx)
	defer ctx.Check(planet.Shutdown)

	var cErr CCharPtr
	bucketName := "TestBucket"
	project, cProjectRef := openTestProject(t, ctx, planet)

	testEachBucketConfig(t, func(bucketCfg *uplink.BucketConfig) {
		bucket, err := project.CreateBucket(ctx, bucketName, bucketCfg)
		require.NoError(t, err)
		require.NotNil(t, bucket)

		DeleteBucket(cProjectRef, stringToCCharPtr(bucketName), &cErr)
		require.Empty(t, cCharToGoString(cErr))
	})
}

func TestListBuckets(t *testing.T) {
	ctx := testcontext.New(t)
	defer ctx.Cleanup()

	planet := startTestPlanet(t, ctx)
	defer ctx.Check(planet.Shutdown)

	var cErr CCharPtr
	project, cProjectRef := openTestProject(t, ctx, planet)

	bucketCount := 15
	testEachBucketConfig(t, func(bucketCfg *uplink.BucketConfig) {
		for i := 0; i < bucketCount; i++ {
			bucketName := fmt.Sprintf("TestBucket%d", i)
			_, err := project.CreateBucket(ctx, bucketName, bucketCfg)
			require.NoError(t, err)
		}

		// TODO: test with different list options
		cBucketList := ListBuckets(cProjectRef, nil, &cErr)
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
}

func TestGetBucketInfo(t *testing.T) {
	ctx := testcontext.New(t)
	defer ctx.Cleanup()

	planet := startTestPlanet(t, ctx)
	defer ctx.Check(planet.Shutdown)

	var cErr CCharPtr
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

			// NB (workaround): should we use nano precision in c bucket?
			bucket.Created = time.Unix(bucket.Created.Unix(), 0).UTC()
			// TODO: c structs ignore `Volatile` fields; set to zero value for comparison
			bucketConfig.Volatile = uplink.BucketConfig{}.Volatile

			cBucketInfo := GetBucketInfo(cProjectRef, stringToCCharPtr(bucketName), &cErr)
			cConfig, cBucket := cBucketInfo.config, cBucketInfo.bucket

			assert.True(t, reflect.DeepEqual(bucket, newGoBucket(&cBucket)))
			assert.True(t, reflect.DeepEqual(*bucketConfig, newGoBucketConfig(&cConfig)))
		}
	})
}

func TestCloseProject(t *testing.T) {
	ctx := testcontext.New(t)
	defer ctx.Cleanup()

	planet := startTestPlanet(t, ctx)
	defer ctx.Check(planet.Shutdown)

	var cErr CCharPtr
	_, cProjectRef := openTestProject(t, ctx, planet)

	CloseProject(cProjectRef, &cErr)
	require.Empty(t, cCharToGoString(cErr))
}
