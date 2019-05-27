package main_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"storj.io/storj/internal/testcontext"
	"storj.io/storj/lib/uplink"
)

func TestCProjectTests(t *testing.T) {
	ctx := testcontext.New(t)
	defer ctx.Cleanup()

	planet := startTestPlanet(t, ctx)
	defer ctx.Check(planet.Shutdown)

	createBucketName := "create-bucket-" + t.Name()
	deleteBucketName := "delete-bucket-" + t.Name()
	project := newProject(t, planet)
	apikey := newAPIKey(t, ctx, planet, project.ID)
	satelliteAddr := planet.Satellites[0].Addr()

	envVars := []string{
		"CREATE_BUCKET_NAME=" + createBucketName,
		"DELETE_BUCKET_NAME=" + deleteBucketName,
		"SATELLITE_ADDR=" + satelliteAddr,
		"APIKEY=" + apikey,
	}

	{
		u, err := uplink.NewUplink(ctx, testConfig)
		require.NoError(t, err)

		apikey, err := uplink.ParseAPIKey(apikey)
		require.NoError(t, err)

		project, err := u.OpenProject(ctx, satelliteAddr, apikey, nil)
		require.NoError(t, err)

		deleteBucket, err := project.CreateBucket(ctx, deleteBucketName, nil)
		require.NoError(t, err)
		require.NotNil(t, deleteBucket)

		buckets, err := project.ListBuckets(ctx, nil)
		require.NoError(t, err)
		require.NotNil(t, buckets)
		require.NotEmpty(t, buckets.Items)
		assert.Len(t, buckets.Items, 1)

		runCTest(t, ctx, "project_test.c", envVars...)

		buckets, err = project.ListBuckets(ctx, nil)
		require.NoError(t, err)
		require.NotNil(t, buckets)
		require.NotEmpty(t, buckets.Items)
		assert.Len(t, buckets.Items, 1)
		assert.Equal(t, createBucketName, buckets.Items[0].Name)
	}
}
