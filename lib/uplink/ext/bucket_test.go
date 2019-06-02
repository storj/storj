// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package main

import (
	"testing"

	"storj.io/storj/internal/testcontext"
)

// TODO: Start up test planet and call these from bash instead
func TestCBucketTests(t *testing.T) {
	ctx := testcontext.New(t)
	defer ctx.Cleanup()

	planet := startTestPlanet(t, ctx)
	defer ctx.Check(planet.Shutdown)

	consoleProject := newProject(t, planet)
	consoleApikey := newAPIKey(t, ctx, planet, consoleProject.ID)
	satelliteAddr := planet.Satellites[0].Addr()

	envVars := []string{
		"SATELLITE_ADDR=" + satelliteAddr,
		"APIKEY=" + consoleApikey,
	}

	runCTest(t, ctx, "bucket_test.c", envVars...)
}

//func TestUploadObject(t *testing.T) {
//	ctx := testcontext.New(t)
//	defer ctx.Cleanup()
//
//	planet := startTestPlanet(t, ctx)
//	defer ctx.Check(planet.Shutdown)
//
//	var cErr Cchar
//	project, cProjectRef := openTestProject(t, ctx, planet)
//
//	testEachBucketConfig(t, func(bucketCfg *uplink.BucketConfig) {
//		project.CreateBucket(ctx, bucketName, bucketCfg)
//	})
//}
