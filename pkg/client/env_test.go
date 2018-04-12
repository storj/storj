// Copyright (C) 2018 Storj Labs, Inc.
// See LICENSE for copying information.

package client

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

var envTests = []struct {
	env         Env
	expectedURL string
}{
	{Env{}, ""},
	{NewEnv(), DefaultURL},
	{Env{URL: mockBridgeURL}, mockBridgeURL},
}

func TestNewEnv(t *testing.T) {
	for _, tt := range envTests {
		assert.Equal(t, tt.expectedURL, tt.env.URL)
	}
}
