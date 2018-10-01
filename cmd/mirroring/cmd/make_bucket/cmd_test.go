// Copyright (C) 2018 Storj Labs, Inc.
// See LICENSE for copying information.

package make_bucket

import (
	"context"
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
			testName: "Nil slice arguments",

			testFunc: func() {
				err := validateArgs(nil, nil)

				assert.Error(t, err)
				assert.Equal(t, "at least one argument required", err.Error())
			},
		},
		{
			testName: "Empty slice arguments",

			testFunc: func() {
				err := validateArgs(nil, []string{})

				assert.Error(t, err)
				assert.Equal(t, "at least one argument required", err.Error())
			},
		},
		{
			testName: "Invalid bucket name",

			testFunc: func() {
				err := validateArgs(nil, []string{"."})

				assert.Error(t, err)
			},
		},
		{
			testName: "Too many arguments",

			testFunc: func() {
				err := validateArgs(nil, []string{".", "."})

				assert.Error(t, err)
				assert.Equal(t, "too many arguments", err.Error())
			},
		},
		{
			testName: "Valid",

			testFunc: func() {
				err := validateArgs(nil, []string{"a.y.e.bucket"})

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

func TestExec(t *testing.T) {

	prime := test.NewProxyObjectLayer()
	mirroring = prime

	cases := []struct {
		testName string
		testFunc func()
	}{
		{
			testName: "MakeBucketWithLocation error",

			testFunc: func() {

				makeBucketErrorString := "MakeBucketWithLocation failed"

				prime.MakeBucketWithLocationFunc = func (ctx context.Context, bucket string, location string) (err error) {
					return errors.New(makeBucketErrorString)
				}

				err := exec(nil, []string{"-flag"})

				assert.Error(t, err)
				assert.Equal(t, err.Error(), makeBucketErrorString)
			},
		},
		{
			testName: "MakeBucketWithLocation success",

			testFunc: func() {

				prime.MakeBucketWithLocationFunc = func (ctx context.Context, bucket string, location string) (err error) {
					return nil
				}

				err := exec(nil, []string{"-flag"})

				assert.Nil(t, err)
				assert.NoError(t, err)
			},
		},
	}

	for _, c := range cases {
		t.Run(c.testName, func(t *testing.T) {
			c.testFunc()
		})
	}

}
