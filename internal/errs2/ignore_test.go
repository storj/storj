// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package errs2_test

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"github.com/zeebo/errs"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"storj.io/storj/internal/errs2"
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

	// grpc errors
	grpcErr := status.Error(codes.Canceled, context.Canceled.Error())

	require.NotEqual(t, grpcErr, context.Canceled)
	require.True(t, errs2.IsCanceled(grpcErr))

	// nested errors
	nestedParentErr := nestedErr.Wrap(parentErr)
	nestedChildErr := nestedErr.Wrap(childErr)
	nestedGRPCErr := nestedErr.Wrap(grpcErr)

	require.NotEqual(t, nestedParentErr, context.Canceled)
	require.NotEqual(t, nestedChildErr, context.Canceled)
	require.NotEqual(t, nestedGRPCErr, context.Canceled)

	require.True(t, errs2.IsCanceled(nestedParentErr))
	require.True(t, errs2.IsCanceled(nestedChildErr))
	require.True(t, errs2.IsCanceled(nestedChildErr))

	// combined errors
	combinedParentErr := errs.Combine(combinedErr, parentErr)
	combinedChildErr := errs.Combine(combinedErr, childErr)
	combinedGRPCErr := errs.Combine(combinedErr, childErr)

	require.NotEqual(t, combinedParentErr, context.Canceled)
	require.NotEqual(t, combinedChildErr, context.Canceled)
	require.NotEqual(t, combinedGRPCErr, context.Canceled)

	require.True(t, errs2.IsCanceled(combinedParentErr))
	require.True(t, errs2.IsCanceled(combinedChildErr))
	require.True(t, errs2.IsCanceled(combinedGRPCErr))
}
