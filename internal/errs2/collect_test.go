// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package errs2_test

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/zeebo/errs"

	"storj.io/storj/internal/errs2"
)

func TestCollectSingleError(t *testing.T) {
	errchan := make(chan error)
	defer close(errchan)

	go func() {
		errchan <- errs.New("error")
	}()

	err := errs2.Collect(errchan, 1*time.Second)
	assert.Error(t, err)
	assert.Equal(t, err.Error(), "error")
}

func TestCollectMultipleError(t *testing.T) {
	errchan := make(chan error)
	defer close(errchan)

	go func() {
		errchan <- errs.New("error1")
		errchan <- errs.New("error2")
		errchan <- errs.New("error3")
	}()

	err := errs2.Collect(errchan, 1*time.Second)
	assert.Error(t, err)
	assert.Equal(t, err.Error(), "error1; error2; error3")
}
