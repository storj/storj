// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

// +build drpc

package rpcstatus

import (
	"context"
	"errors"
	"fmt"

	"storj.io/drpc/drpcerr"
)

// StatusCode is the type of status codes for drpc.
type StatusCode uint64

// These constants are all the rpc error codes.
const (
	Unknown StatusCode = iota
	OK
	Canceled
	InvalidArgument
	DeadlineExceeded
	NotFound
	AlreadyExists
	PermissionDenied
	ResourceExhausted
	FailedPrecondition
	Aborted
	OutOfRange
	Unimplemented
	Internal
	Unavailable
	DataLoss
	Unauthenticated
)

// Code returns the status code associated with the error.
func Code(err error) StatusCode {
	// special case: if the error is context canceled or deadline exceeded, the code
	// must be those.
	switch err {
	case context.Canceled:
		return Canceled
	case context.DeadlineExceeded:
		return DeadlineExceeded
	default:
		return drpcerr.Code(err)
	}

}

// Error wraps the message with a status code into an error.
func Error(code StatusCode, msg string) error {
	return drpcerr.WithCode(errors.New(msg), uint64(code))
}

// Errorf : Error :: fmt.Sprintf : fmt.Sprint
func Errorf(code StatusCode, format string, a ...interface{}) error {
	return drpcerr.WithCode(fmt.Errorf(format, a...), uint64(code))
}
