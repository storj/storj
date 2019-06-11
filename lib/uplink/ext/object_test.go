// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package main

import (
	"github.com/stretchr/testify/require"
	"io/ioutil"
	"reflect"
	"storj.io/storj/internal/testcontext"
	"storj.io/storj/lib/uplink"
	"testing"
)

func TestObjectMeta(t *testing.T) {
	ctx := testcontext.New(t)
	defer ctx.Cleanup()

	planet := startTestPlanet(t, ctx)
	defer ctx.Check(planet.Shutdown)

	var cErr Cchar
	bucketName := "TestBucket"
	project, _ := openTestProject(t, ctx, planet)

	testObjects := newTestObjects(1)
	testEachBucketConfig(t, func(bucketCfg *uplink.BucketConfig) {
		_, err := project.CreateBucket(ctx, bucketName, bucketCfg)
		require.NoError(t, err)

		openBucket, err := project.OpenBucket(ctx, bucketName, nil)
		require.NoError(t, err)
		require.NotNil(t, openBucket)

		for _, testObj := range testObjects {
			testObj.goUpload(t, ctx, openBucket)

			require.Empty(t, cCharToGoString(cErr))

			object, err := openBucket.OpenObject(ctx, testObj.Path)
			require.NoError(t, err)
			require.NotNil(t, object)

			cObjectRef := CObjectRef(structRefMap.Add(object))

			cObjectMeta := ObjectMeta(cObjectRef, &cErr)
			require.Empty(t, cCharToGoString(cErr))

			actualObjectMeta := newGoObjectMeta(t, &cObjectMeta)
			// NB: c structs ignore `Volatile` fields; set to zero value for comparison
			object.Meta.Volatile = uplink.ObjectMeta{}.Volatile

			require.True(t, reflect.DeepEqual(object.Meta, actualObjectMeta))
		}

	})
}

func TestDownloadRange(t *testing.T) {
	ctx := testcontext.New(t)
	defer ctx.Cleanup()

	planet := startTestPlanet(t, ctx)
	defer ctx.Check(planet.Shutdown)

	var cErr Cchar
	bucketName := "TestBucket"
	project, _ := openTestProject(t, ctx, planet)

	testObjects := newTestObjects(1)
	testEachBucketConfig(t, func(bucketCfg *uplink.BucketConfig) {
		_, err := project.CreateBucket(ctx, bucketName, bucketCfg)
		require.NoError(t, err)

		openBucket, err := project.OpenBucket(ctx, bucketName, nil)
		require.NoError(t, err)
		require.NotNil(t, openBucket)

		cBucketRef := CBucketRef(structRefMap.Add(openBucket))
		for _, testObj := range testObjects {
			testObj.goUpload(t, ctx, openBucket)

			require.Empty(t, cCharToGoString(cErr))

			path := stringToCCharPtr(string(testObj.Path))
			objectRef := OpenObject(cBucketRef, path, &cErr)
			require.Empty(t, cCharToGoString(cErr))

			objectMeta := ObjectMeta(objectRef, &cErr)
			require.Empty(t, cCharToGoString(cErr))

			f := TempFile(nil)
			defer f.Close()

			DownloadRange(objectRef, 0, Cint64(objectMeta.Size), f, &cErr)
			require.Empty(t, cCharToGoString(cErr))

			f.Seek(0, 0)
			b, err := ioutil.ReadAll(f)
			require.Empty(t, err)

			require.Equal(t, len(testObj.Data), len(b))
			require.Equal(t, testObj.Data, b)
		}
	})
}