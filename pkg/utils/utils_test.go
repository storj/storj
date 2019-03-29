// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package utils_test

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/zeebo/errs"

	"storj.io/storj/pkg/utils"
)

func TestCollectSingleError(t *testing.T) {
	errchan := make(chan error)
	defer close(errchan)

	go func() {
		errchan <- errs.New("error")
	}()

	err := utils.CollectErrors(errchan, 1*time.Second)
	assert.Error(t, err)
	assert.Equal(t, err.Error(), "error")
}

func TestCollecMultipleError(t *testing.T) {
	errchan := make(chan error)
	defer close(errchan)

	go func() {
		errchan <- errs.New("error1")
		errchan <- errs.New("error2")
		errchan <- errs.New("error3")
	}()

	err := utils.CollectErrors(errchan, 1*time.Second)
	assert.Error(t, err)
	assert.Equal(t, err.Error(), "error1; error2; error3")
}
