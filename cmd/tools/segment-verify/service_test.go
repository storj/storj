// Copyright (C) 2022 Storj Labs, Inc.
// See LICENSE for copying information.

package main_test

import (
	"testing"

	"github.com/stretchr/testify/require"

	segmentverify "storj.io/storj/cmd/tools/segment-verify"
)

func TestService(t *testing.T) {
	service := segmentverify.NewService()
	require.NotNil(t, service)
}
