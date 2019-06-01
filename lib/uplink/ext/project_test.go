package main

import (
	"storj.io/storj/lib/uplink"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

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
	ctx := testcontext.New(t)
	defer ctx.Cleanup()

	planet := startTestPlanet(t, ctx)
	defer ctx.Check(planet.Shutdown)

	var cErr Cchar
	bucketName := "TestBucket"
	_, cProjectRef := openTestProject(t, ctx, planet)

	testEachBucketConfig(t, func(bucketCfg uplink.BucketConfig) {
		cBucketConfig := NewCBucketConfig(&bucketCfg)
		cBucket := CreateBucket(cProjectRef, stringToCCharPtr(bucketName), &cBucketConfig, &cErr)
		require.Empty(t, cCharToGoString(cErr))
		require.NotNil(t, cBucket)

		assert.Equal(t, bucketName, cCharToGoString(cBucket.name))
		assert.Condition(t, func() bool {
			createdTime := time.Unix(int64(cBucket.created), 0)
			return time.Now().Sub(createdTime).Seconds() < 3
		})

		assert.NotNil(t, cBucket.encryption_parameters)
		// TODO: encryption_parameters assertions

		assert.NotNil(t, cBucket.redundancy_scheme)
		// TODO: redundancy_scheme assertions
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

		cBucketRef := OpenBucket(cProjectRef, stringToCCharPtr(bucketName), nil, &cErr)
		require.Empty(t, cCharToGoString(cErr))
		require.NotEmpty(t, cBucketRef)
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
