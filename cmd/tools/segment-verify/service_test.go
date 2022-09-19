// Copyright (C) 2022 Storj Labs, Inc.
// See LICENSE for copying information.

package main_test

import (
	"testing"

	"github.com/stretchr/testify/require"

	"storj.io/common/testcontext"
	segmentverify "storj.io/storj/cmd/tools/segment-verify"
	"storj.io/storj/private/testplanet"
)

func TestService(t *testing.T) {
	ctx := testcontext.New(t)
	log := testplanet.NewLogger(t)

	config := segmentverify.ServiceConfig{
		NotFoundPath: ctx.File("not-found.csv"),
		RetryPath:    ctx.File("retry.csv"),
	}

	service, err := segmentverify.NewService(log.Named("segment-verify"), nil, nil, nil, config)
	require.NoError(t, err)
	require.NotNil(t, service)

	defer ctx.Check(service.Close)
}
