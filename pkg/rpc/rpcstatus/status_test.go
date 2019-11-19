// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package rpcstatus

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"google.golang.org/grpc/status"

	"storj.io/drpc/drpcerr"
)

var allCodes = []StatusCode{
	Unknown,
	OK,
	Canceled,
	InvalidArgument,
	DeadlineExceeded,
	NotFound,
	AlreadyExists,
	PermissionDenied,
	ResourceExhausted,
	FailedPrecondition,
	Aborted,
	OutOfRange,
	Unimplemented,
	Internal,
	Unavailable,
	DataLoss,
	Unauthenticated,
}

func TestStatus(t *testing.T) {
	for _, code := range allCodes {
		err := Error(code, "")
		assert.Equal(t, Code(err), code)
		assert.Equal(t, status.Code(err), code.toGRPC())
		assert.Equal(t, drpcerr.Code(err), uint64(code))
	}

	assert.Equal(t, Code(nil), OK)
	assert.Equal(t, Code(context.Canceled), Canceled)
	assert.Equal(t, Code(context.DeadlineExceeded), DeadlineExceeded)
}
