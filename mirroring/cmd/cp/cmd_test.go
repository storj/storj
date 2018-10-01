// Copyright (C) 2018 Storj Labs, Inc.
// See LICENSE for copying information.

package cp

import (
	"context"
	"github.com/minio/minio/cmd"
	"github.com/pkg/errors"
	"github.com/stretchr/testify/assert"
	"testing"

	test "storj.io/mirroring/utils/testing_utils"
)

func TestValidateArgs(t *testing.T) {

	cases := []struct {
		testName string
		testFunc func()
	}{
		{
			testName: "Nil slice arguments - error thrown",

			testFunc: func() {
				err := validateArgs(nil, nil)

				assert.Error(t, err)
				assert.NotNil(t, err)
				assert.Equal(t, missingArgsErrorMessage, err.Error())
			},
		},
		{
			testName: "empty slice arguments - error thrown",

			testFunc: func() {
				err := validateArgs(nil, []string{})

				assert.Error(t, err)
				assert.NotNil(t, err)
				assert.Equal(t, missingArgsErrorMessage, err.Error())
			},
		},
		{
			testName: "one argument - error thrown",

			testFunc: func() {
				err := validateArgs(nil, []string{"arg1"})

				assert.Error(t, err)
				assert.NotNil(t, err)
				assert.Equal(t, missingArgsErrorMessage, err.Error())
			},
		},
		{
			testName: "two argument - error thrown",

			testFunc: func() {
				err := validateArgs(nil, []string{"arg1", "arg2"})

				assert.Error(t, err)
				assert.NotNil(t, err)
				assert.Equal(t, missingArgsErrorMessage, err.Error())
			},
		},
		{
			testName: "three arguments - valid",

			testFunc: func() {
				err := validateArgs(nil, []string{"arg1", "arg2", "arg3"})

				assert.NoError(t, err)
				assert.Nil(t, err)
			},
		},
		{
			testName: "three arguments, srcBucket unvalid, err thrown",

			testFunc: func() {
				err := validateArgs(nil, []string{".", "arg2", "arg3"})

				assert.Error(t, err)
				assert.NotNil(t, err)
				assert.Equal(t, "srcBucket - Bucket name cannot be smaller than 3 characters", err.Error())
			},
		},
		{
			testName: "three arguments, srcBucket and dstBucket unvalid, err thrown",

			testFunc: func() {
				expectedErrorMessage := "srcBucket - Bucket name cannot be smaller than 3 characters\ndstBucket - Bucket name cannot be smaller than 3 characters"
				err := validateArgs(nil, []string{".", "arg2", "."})

				assert.Error(t, err)
				assert.NotNil(t, err)
				assert.Equal(t, expectedErrorMessage , err.Error())
			},
		},
		{
			testName: "three arguments, srcBucket, dstBucket and srcObject unvalid, err thrown",

			testFunc: func() {
				expectedErrorMessage := "srcBucket - Bucket name cannot be smaller than 3 characters\n" +
					"dstBucket - Bucket name cannot be smaller than 3 characters\n" +
					"srcObject - Object name cannot be empty"
				err := validateArgs(nil, []string{".", "", "."})

				assert.Error(t, err)
				assert.NotNil(t, err)
				assert.Equal(t, expectedErrorMessage , err.Error())
			},
		},
		{
			testName: "4 args, dstObj unvalid, err thrown",

			testFunc: func() {
				expectedErrorMessage := "dstObject - Object name cannot be empty"
				err := validateArgs(nil, []string{"arg1", "arg2", "arg3", ""})

				assert.Error(t, err)
				assert.NotNil(t, err)
				assert.Equal(t, expectedErrorMessage , err.Error())
			},
		},

	}

	for _, c := range cases {
		t.Run(c.testName, func(t *testing.T) {
			c.testFunc()
		})
	}
}

func TestExec(t *testing.T) {

	proxy := test.NewProxyObjectLayer()
	mirroring = proxy

	cases := []struct {
		testName string
		testFunc func()
	}{
		{
			testName: "CopyObject error",

			testFunc: func() {

				copyObjectErrorString := "CopyObject failed"

				proxy.CopyObjectFunc = func(ctx context.Context, srcBucket, srcObject, destBucket, destObject string, srcInfo cmd.ObjectInfo, srcOpts, dstOpts cmd.ObjectOptions) (objInfo cmd.ObjectInfo, err error) {
					return objInfo, errors.New(copyObjectErrorString)
				}

				err := exec(nil, []string{"srcBucket", "srcObj", "dstBucket"})

				assert.Error(t, err)
				assert.NotNil(t, err)
				assert.Equal(t, err.Error(), copyObjectErrorString)
			},
		},
		{
			testName: "CopyObject success",

			testFunc: func() {
				proxy.CopyObjectFunc = func(ctx context.Context, srcBucket, srcObject, destBucket, destObject string, srcInfo cmd.ObjectInfo, srcOpts, dstOpts cmd.ObjectOptions) (objInfo cmd.ObjectInfo, err error) {
					return objInfo, nil
				}

				err := exec(nil, []string{"srcBucket", "srcObj", "dstBucket"})

				assert.NoError(t, err)
				assert.Nil(t, err)
			},
		},
	}

	for _, c := range cases {
		t.Run(c.testName, func(t *testing.T) {
			c.testFunc()
		})
	}

}
