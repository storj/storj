// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package errs2

import (
	"context"
	"net/http"

	"github.com/zeebo/errs"
	"google.golang.org/grpc"
)

// IgnoreCanceled returns nil, when the operation was about canceling.
func IgnoreCanceled(err error) error {
	if errs.IsFunc(err, func(err error) bool {
		return err == context.Canceled ||
			err == grpc.ErrServerStopped ||
			err == http.ErrServerClosed
	}) {
		return nil
	}

	return err
}
