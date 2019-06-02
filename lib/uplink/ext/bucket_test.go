// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package main

import (
	"storj.io/storj/pkg/storj"
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

type TestObject struct {
	storj.Object
}

func (obj *TestObject) Upload(cErr *Cchar) {

}

//var testObjects = NewTestObjects(15)
//
//func NewTestObjects(count int) (objects []TestObject) {
//	randPath := make([]byte, 15)
//	rand.Read(randPath)
//
//	obj := storj.Object{
//		//Version:,
//		//Bucket:,
//		Path: string(randPath),
//		//IsPrefix:,
//		//Metadata:,
//		//ContentType:,
//		//Expires:,
//	}
//
//	for i := 0; i < count; i++ {
//		objects = append(objects, TestObject{obj})
//	}
//
//	return objects
//}
//
//func TestUploadObject(t *testing.T) {
//	ctx := testcontext.New(t)
//	defer ctx.Cleanup()
//
//	planet := startTestPlanet(t, ctx)
//	defer ctx.Check(planet.Shutdown)
//
//	var cErr Cchar
//	bucketName := "TestBucket"
//	project, cProjectRef := openTestProject(t, ctx, planet)
//
//	testEachBucketConfig(t, func(bucketCfg *uplink.BucketConfig) {
//		bucket, err := project.CreateBucket(ctx, bucketName, bucketCfg)
//
//		for i, object := range testObjects {
//			object.Upload(&cErr)
//			assert.NotEmpty(t, cCharToGoString(cErr))
//		}
//	})
//}
