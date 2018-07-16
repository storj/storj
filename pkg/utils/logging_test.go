// Copyright (C) 2018 Storj Labs, Inc.
// See LICENSE for copying information.

package utils

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"go.uber.org/zap"
)

var errExpected = errors.New("error with initializing logger")

func TestNewLoggerDev(t *testing.T) {
	oldZapNewDevelopment := zapNewDevelopment

	defer func() { zapNewDevelopment = oldZapNewDevelopment }()

	zapNewDevelopment = func(options ...zap.Option) (*zap.Logger, error) {
		return nil, errExpected
	}

	_, err := NewLogger("dev")

	assert.NotNil(t, err)
	assert.Equal(t, err, errExpected)

	_, err = NewLogger("development")

	assert.NotNil(t, err)
	assert.Equal(t, err, errExpected)
}

func TestNewLoggerProd(t *testing.T) {
	oldZapNewProduction := zapNewProduction

	defer func() { zapNewProduction = oldZapNewProduction }()

	zapNewProduction = func(options ...zap.Option) (*zap.Logger, error) {
		return nil, errExpected
	}

	_, err := NewLogger("prod")

	assert.NotNil(t, err)
	assert.Equal(t, err, errExpected)

	_, err = NewLogger("production")

	assert.NotNil(t, err)
	assert.Equal(t, err, errExpected)
}

func TestNewLoggerDefault(t *testing.T) {
	oldZapNewNop := zapNewNop

	defer func() { zapNewNop = oldZapNewNop }()

	zapNewNop = func() *zap.Logger {
		return nil
	}

	client, err := NewLogger("default")

	assert.Nil(t, client)
	assert.Nil(t, err)
}
