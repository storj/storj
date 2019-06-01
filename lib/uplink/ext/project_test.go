package main

import (
	"reflect"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"storj.io/storj/lib/uplink"

	"storj.io/storj/internal/testcontext"
)

// TODO: Start up test planet and call these from bash instead
func TestCProjectTests(t *testing.T) {
	ctx := testcontext.New(t)
	defer ctx.Cleanup()

	planet := startTestPlanet(t, ctx)
	defer ctx.Check(planet.Shutdown)

	project := newProject(t, planet)
	apikey := newAPIKey(t, ctx, planet, project.ID)
	satelliteAddr := planet.Satellites[0].Addr()

	envVars := []string{
		"SATELLITE_ADDR=" + satelliteAddr,
		"APIKEY=" + apikey,
	}

	runCTest(t, ctx, "project_test.c", envVars...)
}

func TestCreateBucket(t *testing.T) {
	// TODO: figure this out (there may be other inconsistencies as well)
	t.Log("listed bucket *always* has `PathCipher` = `AESGCM`; is this expected behavior?")
	t.SkipNow()

	ctx := testcontext.New(t)
	defer ctx.Cleanup()

	planet := startTestPlanet(t, ctx)
	defer ctx.Check(planet.Shutdown)

	var cErr Cchar
	bucketName := "TestBucket"
	project, cProjectRef := openTestProject(t, ctx, planet)

	testEachBucketConfig(t, func(bucketCfg uplink.BucketConfig) {
		cBucketConfig := NewCBucketConfig(&bucketCfg)
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

	var cErr Cchar
	bucketName := "TestBucket"
	project, cProjectRef := openTestProject(t, ctx, planet)

	testEachBucketConfig(t, func(bucketCfg uplink.BucketConfig) {
		bucket, err := project.CreateBucket(ctx, bucketName, &bucketCfg)
		require.NoError(t, err)
		require.NotNil(t, bucket)

		expectedBucket, err := project.OpenBucket(ctx, bucketName, nil)
		require.NoError(t, err)
		require.NotNil(t, expectedBucket)

		cBucketRef := OpenBucket(cProjectRef, stringToCCharPtr(bucketName), nil, &cErr)
		require.Empty(t, cCharToGoString(cErr))
		require.NotEmpty(t, cBucketRef)

		goBucket, ok := structRefMap.Get(token(cBucketRef)).(*uplink.Bucket)
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
}

func TestListBuckets(t *testing.T) {
	ctx := testcontext.New(t)
	defer ctx.Cleanup()

	planet := startTestPlanet(t, ctx)
	defer ctx.Check(planet.Shutdown)
}

func TestGetBucketInfo(t *testing.T) {
	ctx := testcontext.New(t)
	defer ctx.Cleanup()

	planet := startTestPlanet(t, ctx)
	defer ctx.Check(planet.Shutdown)
}

func TestCloseProject(t *testing.T) {
	ctx := testcontext.New(t)
	defer ctx.Cleanup()

	planet := startTestPlanet(t, ctx)
	defer ctx.Check(planet.Shutdown)
}
