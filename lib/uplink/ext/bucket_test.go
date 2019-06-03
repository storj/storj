// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package main

import (
	"bytes"
	"math/rand"
	"reflect"
	"testing"
	"time"
	"unsafe"

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

func TestOpenObject(t *testing.T) {
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

		openBucket, err := project.OpenBucket(ctx, bucketName, nil)
		require.NoError(t, err)
		require.NotNil(t, openBucket)

		cBucketRef := CBucketRef(structRefMap.Add(openBucket))
		for _, testObj := range testObjects {
			testObj.goUpload(t, ctx, openBucket)

			require.Empty(t, cCharToGoString(cErr))

			path := stringToCCharPtr(string(testObj.Path))
			OpenObject(cBucketRef, path, &cErr)
			require.Empty(t, cCharToGoString(cErr))
			// TODO: Test only checks for no error right now
		}
	})

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
		bucket, err := project.CreateBucket(ctx, bucketName, bucketCfg)
		require.NoError(t, err)

		// TODO: test with EncryptionAccess
		// TODO: test with different content types
		openBucket, err := project.OpenBucket(ctx, bucketName, nil)
		require.NoError(t, err)
		require.NotNil(t, openBucket)

		cBucketRef := CBucketRef(structRefMap.Add(openBucket))
		for _, testObj := range testObjects {
			testObj.cUpload(t, cBucketRef, &cErr)
			require.Empty(t, cCharToGoString(cErr))

			objectList, err := openBucket.ListObjects(ctx, nil)
			require.NoError(t, err)
			require.NotEmpty(t, objectList)
			require.Len(t, objectList.Items, 1)

			object := objectList.Items[0]

			assert.True(t, reflect.DeepEqual(bucket, object.Bucket))
			assert.Equal(t, object.Path, testObj.Path)
			assert.True(t, object.Created.Sub(time.Now()).Seconds() < 2)
			assert.Equal(t, object.Created, object.Modified)
			assert.Equal(t, object.Expires.Unix(), testObj.UploadOpts.Expires.Unix())
			assert.Equal(t, object.ContentType, testObj.UploadOpts.ContentType)
			assert.Equal(t, object.Metadata, testObj.UploadOpts.Metadata)
			// TODO: test with `IsPrefix` == true
			assert.Equal(t, object.IsPrefix, testObj.IsPrefix)
			// TODO: assert version

			err = openBucket.DeleteObject(ctx, object.Path)
			require.NoError(t, err)
		}
	})
}

func TestListObjects(t *testing.T) {
	ctx := testcontext.New(t)
	defer ctx.Cleanup()

	planet := startTestPlanet(t, ctx)
	defer ctx.Check(planet.Shutdown)

	var cErr Cchar
	bucketName := "TestBucket"
	project, _ := openTestProject(t, ctx, planet)

	testObjects := newTestObjects(15)
	testEachBucketConfig(t, func(bucketCfg *uplink.BucketConfig) {
		bucket, err := project.CreateBucket(ctx, bucketName, bucketCfg)
		require.NoError(t, err)

		// TODO: test with EncryptionAccess
		// TODO: test with different content types
		openBucket, err := project.OpenBucket(ctx, bucketName, nil)
		require.NoError(t, err)
		require.NotNil(t, openBucket)

		cBucketRef := CBucketRef(structRefMap.Add(openBucket))

		for _, testObj := range testObjects {
			testObj.goUpload(t, ctx, openBucket)
			require.Empty(t, cCharToGoString(cErr))

			// TODO: test with different list options
			cObjectList := ListObjects(cBucketRef, nil, &cErr)
			require.Empty(t, cCharToGoString(cErr))

			assert.Equal(t, 1, int(cObjectList.length))
			assert.Equal(t, bucket.Name,  cCharToGoString(cObjectList.bucket))

			object := newGoObject(t, (*CObject)(unsafe.Pointer(cObjectList.items)))

			// NB (workaround): should we use nano precision in c bucket?
			bucket.Created = time.Unix(bucket.Created.Unix(), 0).UTC()
			assert.True(t, reflect.DeepEqual(bucket, object.Bucket))

			assert.Equal(t, object.Path, testObj.Path)
			assert.True(t, object.Created.Sub(time.Now()).Seconds() < 2)
			assert.Equal(t, object.Created, object.Modified)
			assert.Equal(t, object.Expires.Unix(), testObj.UploadOpts.Expires.Unix())
			assert.Equal(t, object.ContentType, testObj.UploadOpts.ContentType)
			assert.Equal(t, object.Metadata, testObj.UploadOpts.Metadata)
			// TODO: test with `IsPrefix` == true
			assert.Equal(t, object.IsPrefix, testObj.IsPrefix)
			// TODO: assert version

			err = openBucket.DeleteObject(ctx, object.Path)
			require.NoError(t, err)
		}
	})
}

func TestCloseBucket(t *testing.T) {
	ctx := testcontext.New(t)
	defer ctx.Cleanup()

	planet := startTestPlanet(t, ctx)
	defer ctx.Check(planet.Shutdown)

	var cErr Cchar
	bucketName := "TestBucket"
	project, _ := openTestProject(t, ctx, planet)

	testEachBucketConfig(t, func(bucketCfg *uplink.BucketConfig) {
		_, err := project.CreateBucket(ctx, bucketName, bucketCfg)
		require.NoError(t, err)

		bucket, err := project.OpenBucket(ctx, bucketName, nil)
		require.NoError(t, err)
		require.NotNil(t, bucket)

		cBucketRef := CBucketRef(structRefMap.Add(bucket))
		CloseBucket(cBucketRef, &cErr)
		require.Empty(t, cCharToGoString(cErr))
	})
}

func (obj *TestObject) cUpload(t *testing.T, cBucketRef CBucketRef, cErr *Cchar) {
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

func (obj *TestObject) goUpload(t *testing.T, ctx *testcontext.Context, bucket *uplink.Bucket) {
	data := bytes.NewBuffer([]byte("test data for path " + obj.Path))
	err := bucket.UploadObject(ctx, obj.Path, data, &obj.UploadOpts)
	require.NoError(t, err)
}

func newTestObjects(count int) (objects []TestObject) {
	rand.Seed(time.Now().UnixNano())
	randPath := make([]byte, 15)
	copy(randPath[:], randSeq(15))

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

var letters = []rune("abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ")

func randSeq(n int) string {
	b := make([]rune, n)
	for i := range b {
		b[i] = letters[rand.Intn(len(letters))]
	}
	return string(b)
}
