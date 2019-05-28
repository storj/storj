package main_test

import (
	"fmt"
	"testing"

	"storj.io/storj/internal/testcontext"
	"storj.io/storj/lib/uplink"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

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

	{
		runCTest(t, ctx, "project_test.c", envVars...)

		goUplink, err := uplink.NewUplink(ctx, testConfig)
		require.NoError(t, err)

		apikey, err := uplink.ParseAPIKey(apikey)
		require.NoError(t, err)

		project, err := goUplink.OpenProject(ctx, satelliteAddr, apikey, nil)
		require.NoError(t, err)

		buckets, err := project.ListBuckets(ctx, nil)
		require.NoError(t, err)

		assert.Len(t, buckets.Items, 2)
		for i, bucket := range buckets.Items {
			num := (i + 1) * 2
			assert.Equal(t, fmt.Sprintf("TestBucket%d", num), bucket.Name)
		}
	}
}
