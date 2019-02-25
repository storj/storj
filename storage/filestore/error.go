// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package filestore

import "os"

// underlyingError returns the underlying error for known os error types.
func underlyingError(err error) error {
	switch err := err.(type) {
	case *os.PathError:
		return err.Err
	case *os.LinkError:
		return err.Err
	case *os.SyscallError:
		return err.Err
	}
	return err
}
