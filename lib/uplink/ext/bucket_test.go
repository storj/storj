// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package main

import (
	"github.com/stretchr/testify/assert"
	"storj.io/storj/pkg/storj"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"storj.io/storj/internal/testcontext"
	"storj.io/storj/lib/uplink"
)

// TODO: Start up test planet and call these from bash instead
func TestCBucketTests(t *testing.T) {
	ctx := testcontext.New(t)
	defer ctx.Cleanup()

	planet := startTestPlanet(t, ctx)
	defer ctx.Check(planet.Shutdown)

	project := newProject(t, planet)
	apikey := newAPIKey(t, ctx, planet, project.ID)
	satelliteAddr := planet.Satellites[0].Addr()
	bucketName := "TestBucket"

	envVars := []string{
		"SATELLITE_ADDR=" + satelliteAddr,
		"APIKEY=" + apikey,
		"BUCKET_NAME=" + bucketName,
	}

	{
		goUplink, err := uplink.NewUplink(ctx, testConfig)
		require.NoError(t, err)

		apikey, err := uplink.ParseAPIKey(apikey)
		require.NoError(t, err)

		project, err := goUplink.OpenProject(ctx, satelliteAddr, apikey, nil)
		require.NoError(t, err)

		_, err = project.CreateBucket(ctx, bucketName, nil)
		require.NoError(t, err)

		key := storj.Key{}
		copy(key[:], []byte("abcdefghijklmnopqrstuvwxyzABCDEF"))
		bucket, err := project.OpenBucket(ctx, bucketName, nil)
		require.NoError(t, err)


		runCTest(t, ctx, "bucket_test.c", envVars...)

		objectList, err := bucket.ListObjects(ctx, nil)
		require.NoError(t, err)

		require.Len(t, objectList.Items, 1)
		object, err := bucket.OpenObject(ctx, objectList.Items[0].Path)
		require.NoError(t, err)

		assert.Condition(t, func() bool {
			return time.Now().Sub(object.Meta.Modified).Seconds() < 5
		})
		// TODO: add more assertions
	}
}
