// Copyright (C) 2018 Storj Labs, Inc.
// See LICENSE for copying information.

package signal

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"storj.io/storj/pkg/utils"
)

func TestDispatcher(t *testing.T) {
	d := NewDispatcher("test")
	assert.Equal(t, d.source, "test")
	assert.NotNil(t, d)
}

func TestRegistrationAndDispatch(t *testing.T) {
	d := NewDispatcher("test")
	d.Register("test:error", func() error {
		return errors.New("error")
	})

	err := d.Dispatch("test:error")
	assert.Equal(t, err, errors.New("error"))

	err = d.Register("test:normal", func() error {
		return nil
	})
	err = d.Dispatch("test:normal")
	assert.Equal(t, err, nil)

	err = d.Register("test:multiple", func() error {
		return nil
	})

	err = d.Register("test:multiple", func() error {
		return errors.New("multiple")
	})

	err = d.Register("test:multiple", func() error {
		return nil
	})

	err = d.Dispatch("test:multiple")
	assert.Equal(t, err, utils.CombineErrors(nil, errors.New("multiple"), nil))

	err = d.Register("test:nil", nil)
	assert.Error(t, err)
}
