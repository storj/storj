// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package errs2_test

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"github.com/zeebo/errs"

	"storj.io/storj/internal/errs2"
	"storj.io/storj/pkg/rpc/rpcstatus"
)

func TestIsCanceled(t *testing.T) {
	nestedErr := errs.Class("nested error")
	combinedErr := errs.New("combined error")
	parentCtx, cancel := context.WithCancel(context.Background())
	childCtx, childCancel := context.WithTimeout(parentCtx, 30*time.Second)
	defer childCancel()

	cancel()

	// context errors
	parentErr := parentCtx.Err()
	childErr := childCtx.Err()

	require.Equal(t, parentErr, context.Canceled)
	require.Equal(t, childErr, context.Canceled)

	require.True(t, errs2.IsCanceled(parentErr))
	require.True(t, errs2.IsCanceled(childErr))

	// rpc errors
	rpcErr := rpcstatus.Error(rpcstatus.Canceled, context.Canceled.Error())

	require.NotEqual(t, rpcErr, context.Canceled)
	require.True(t, errs2.IsCanceled(rpcErr))

	// nested errors
	nestedParentErr := nestedErr.Wrap(parentErr)
	nestedChildErr := nestedErr.Wrap(childErr)
	nestedRPCErr := nestedErr.Wrap(rpcErr)

	require.NotEqual(t, nestedParentErr, context.Canceled)
	require.NotEqual(t, nestedChildErr, context.Canceled)
	require.NotEqual(t, nestedRPCErr, context.Canceled)

	require.True(t, errs2.IsCanceled(nestedParentErr))
	require.True(t, errs2.IsCanceled(nestedChildErr))
	require.True(t, errs2.IsCanceled(nestedChildErr))

	// combined errors
	combinedParentErr := errs.Combine(combinedErr, parentErr)
	combinedChildErr := errs.Combine(combinedErr, childErr)
	combinedRPCErr := errs.Combine(combinedErr, childErr)

	require.NotEqual(t, combinedParentErr, context.Canceled)
	require.NotEqual(t, combinedChildErr, context.Canceled)
	require.NotEqual(t, combinedRPCErr, context.Canceled)

	require.True(t, errs2.IsCanceled(combinedParentErr))
	require.True(t, errs2.IsCanceled(combinedChildErr))
	require.True(t, errs2.IsCanceled(combinedRPCErr))
}
