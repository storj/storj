// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package main

import (
	"bytes"
	"math/rand"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"storj.io/storj/lib/uplink"
	"storj.io/storj/pkg/storj"

	"storj.io/storj/internal/testcontext"
)

type TestObject struct {
	storj.Object
	UploadOpts uplink.UploadOptions
}

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

func TestUploadObject(t *testing.T) {
	ctx := testcontext.New(t)
	defer ctx.Cleanup()

	planet := startTestPlanet(t, ctx)
	defer ctx.Check(planet.Shutdown)

	var cErr Cchar
	bucketName := "TestBucket"
	project, _ := openTestProject(t, ctx, planet)

	testObjects := newTestObjects(15)
	testEachBucketConfig(t, func(bucketCfg *uplink.BucketConfig) {
		_, err := project.CreateBucket(ctx, bucketName, bucketCfg)
		require.NoError(t, err)

		// TODO: test with EncryptionAccess
		// TODO: test with different content types
		bucket, err := project.OpenBucket(ctx, bucketName, nil)
		require.NoError(t, err)
		require.NotNil(t, bucket)

		cBucketRef := CBucketRef(structRefMap.Add(bucket))
		for _, testObj := range testObjects {
			testObj.Upload(t, cBucketRef, &cErr)
			require.Empty(t, cCharToGoString(cErr))

			objectList, err := bucket.ListObjects(ctx, nil)
			require.NoError(t, err)
			require.NotEmpty(t, objectList)
			require.Len(t, objectList.Items, 1)

			object := objectList.Items[0]

			assert.Equal(t, object.Bucket.Name, bucketName)
			assert.Equal(t, object.Path, testObj.Path)
			assert.True(t, object.Created.Sub(time.Now()).Seconds() < 2)
			assert.Equal(t, object.Created, object.Modified)
			assert.Equal(t, object.Expires.Unix(), testObj.UploadOpts.Expires.Unix())
			assert.Equal(t, object.ContentType, testObj.UploadOpts.ContentType)
			assert.Equal(t, object.Metadata, testObj.UploadOpts.Metadata)
			// TODO: test with `IsPrefix` == true
			assert.Equal(t, object.IsPrefix, testObj.IsPrefix)
			// TODO: assert version

			err = bucket.DeleteObject(ctx, object.Path)
			require.NoError(t, err)
		}
	})
}

func (obj *TestObject) Upload(t *testing.T, cBucketRef CProjectRef, cErr *Cchar) {
	dataRef := NewBuffer()
	buf, ok := structRefMap.Get(token(dataRef)).(*bytes.Buffer)
	require.True(t, ok)

	_, err := buf.Write([]byte("test data for path " + obj.Path))
	require.NoError(t, err)
	require.NotEmpty(t, buf.Bytes())

	cOpts := newCUploadOpts(&obj.UploadOpts)
	UploadObject(cBucketRef, stringToCCharPtr(obj.Path), dataRef, cOpts, cErr)
	require.Empty(t, cCharToGoString(*cErr))
}

func newTestObjects(count int) (objects []TestObject) {
	randPath := make([]byte, 15)
	rand.Read(randPath)

	obj := storj.Object{
		// TODO: test `Version`?
		// TODO: test `IsPrefix`?
		//Version:,
		//IsPrefix:,
		Path: string(randPath),
	}

	expiration := time.Now().Add(time.Duration(rand.Intn(1000) * int(time.Second)))
	opts := uplink.UploadOptions{
		ContentType: "text/plain",
		Expires:     expiration,
		// TODO: randomize
		Metadata: map[string]string{
			"key_one":   "value_one",
			"key_two":   "value_two",
			"key_three": "value_three",
		},
	}

	for i := 0; i < count; i++ {
		objects = append(objects, TestObject{
			Object:     obj,
			UploadOpts: opts,
		})
	}

	return objects
}
