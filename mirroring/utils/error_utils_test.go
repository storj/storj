// Copyright (C) 2018 Storj Labs, Inc.
// See LICENSE for copying information.

package utils

import (
	"github.com/pkg/errors"
	"github.com/stretchr/testify/assert"
	"testing"
)

func TestCombineErrors(t *testing.T) {
	cases := []struct {
		testName string
		testFunc func()
	}{
		{
			testName: "Nil slice will return nil",

			testFunc: func() {
				err := CombineErrors(nil)

				assert.Nil(t, err)
				assert.NoError(t, err)
			},
		},
		{
			testName: "Empty slice will return nil",

			testFunc: func() {
				err := CombineErrors([]error{})

				assert.Nil(t, err)
				assert.NoError(t, err)
			},
		},
		{
			testName: "Single error - single message",

			testFunc: func() {
				errMsg := "first error"
				err := CombineErrors([]error{errors.New(errMsg)})

				assert.NotNil(t, err)
				assert.Error(t, err)
				assert.Equal(t, errMsg, err.Error())
			},
		},
		{
			testName: "Two errors - two messages",

			testFunc: func() {
				firstErrMsg := "first error"
				secondErrMsg := "second error"

				err := CombineErrors([]error{errors.New(firstErrMsg), errors.New(secondErrMsg)})

				assert.NotNil(t, err)
				assert.Error(t, err)
				assert.Equal(t, firstErrMsg + "\n" + secondErrMsg, err.Error())
			},
		},
		{
			testName: "Nine errors - nine messages",

			testFunc: func() {
				errSlice := []error {
					errors.New("1"),
					errors.New("2"),
					errors.New("3"),
					errors.New("4"),
					errors.New("5"),
					errors.New("6"),
					errors.New("7"),
					errors.New("8"),
					errors.New("9"),

				}

				err := CombineErrors(errSlice)

				assert.NotNil(t, err)
				assert.Error(t, err)
				assert.Equal(t, "1\n2\n3\n4\n5\n6\n7\n8\n9", err.Error())
			},
		},
		{
			testName: "Nil errors will be skipped",

			testFunc: func() {
				errSlice := []error {
					errors.New("1"),
					nil,
					errors.New("3"),
					errors.New("4"),
					nil,
					errors.New("6"),
					nil,
					errors.New("8"),
					nil,
				}

				err := CombineErrors(errSlice)

				assert.NotNil(t, err)
				assert.Error(t, err)
				assert.Equal(t, "1\n3\n4\n6\n8", err.Error())
			},
		},
	}

	for _, c := range cases {
		t.Run(c.testName, func(t *testing.T) {
			c.testFunc()
		})
	}
}

func TestNewError(t *testing.T) {

}