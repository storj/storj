// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package errs2

import (
	"context"
	"net/http"

	"github.com/zeebo/errs"
	"google.golang.org/grpc"
)

// IgnoreCanceled returns nil, when the operation was about cancelling
func IgnoreCanceled(originalError error) error {
	err := originalError
	for err != nil {
		if err == context.Canceled ||
			err == grpc.ErrServerStopped ||
			err == http.ErrServerClosed {
			return nil
		}
		unwrapped := errs.Unwrap(err)
		if unwrapped == err {
			return originalError
		}
		err = unwrapped
	}

	return originalError
}
