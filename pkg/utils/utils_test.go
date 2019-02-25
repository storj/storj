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

func TestErrorGroup(t *testing.T) {
	var errlist utils.ErrorGroup
	errlist.Add(nil, nil, nil)
	assert.NoError(t, errlist.Finish())
	assert.Equal(t, len(errlist), 0)
	e1 := errs.New("err1")
	errlist.Add(nil, nil, e1, nil)
	assert.Equal(t, errlist.Finish(), e1)
	assert.Equal(t, len(errlist), 1)
	e2, e3 := errs.New("err2"), errs.New("err3")
	errlist.Add(e2, e3)
	assert.Error(t, errlist.Finish())
	assert.Equal(t, len(errlist), 3)
	assert.Equal(t, errlist[0], e1)
	assert.Equal(t, errlist[1], e2)
	assert.Equal(t, errlist[2], e3)
}
