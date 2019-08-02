// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package errs2

import (
	"context"
	"net/http"

	"github.com/zeebo/errs"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// IsCanceled returns true, when the error is a cancellation.
func IsCanceled(err error) bool {
	return errs.IsFunc(err, func(err error) bool {
		status, ok := status.FromError(err)

		return err == context.Canceled ||
			err == grpc.ErrServerStopped ||
			err == http.ErrServerClosed ||
			(ok && status.Code() == codes.Canceled)
	})
}

// IgnoreCanceled returns nil, when the operation was about canceling.
func IgnoreCanceled(err error) error {
	if IsCanceled(err) {
		return nil
	}
	return err
}
