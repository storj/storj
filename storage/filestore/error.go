// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package filestore

import (
	"errors"
	"os"
)

// underlyingError returns the underlying error for known os error types.
func underlyingError(err error) error {
	var perr *os.PathError
	var lerr *os.LinkError
	var serr *os.SyscallError
	switch {
	case errors.As(err, &perr):
		return perr.Err
	case errors.As(err, &lerr):
		return lerr.Err
	case errors.As(err, &serr):
		return serr.Err
	}
	return err
}
