// Copyright (C) 2022 Storj Labs, Inc.
// See LICENSE for copying information.

package main_test

import (
	"testing"

	"github.com/stretchr/testify/require"

	segmentverify "storj.io/storj/cmd/tools/segment-verify"
	"storj.io/storj/private/testplanet"
)

func TestService(t *testing.T) {
	log := testplanet.NewLogger(t)
	config := segmentverify.ServiceConfig{}
	service := segmentverify.NewService(log.Named("segment-verify"), nil, nil, nil, config)
	require.NotNil(t, service)
}
