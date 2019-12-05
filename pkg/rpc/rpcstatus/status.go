// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package rpcstatus

import (
	"context"
	"errors"
	"fmt"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"storj.io/drpc/drpcerr"
)

// StatusCode is an enumeration of rpc status codes.
type StatusCode uint64

// These constants are all the rpc error codes. It is important that
// their numerical values do not change.
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
	// must be those. additionally, grpc returns OK for a nil error, so we will, too.
	switch err {
	case nil:
		return OK
	case context.Canceled:
		return Canceled
	case context.DeadlineExceeded:
		return DeadlineExceeded
	default:
		if code := StatusCode(drpcerr.Code(err)); code != Unknown {
			return code
		}
		if grpccode := status.Code(err); grpccode != codes.Unknown {
			return statusCodeFromGRPC(grpccode)
		}
		return Unknown
	}
}

// Error wraps the message with a status code into an error.
func Error(code StatusCode, msg string) error {
	return &codeErr{
		err:  errors.New(msg),
		code: code,
		grpc: status.New(code.toGRPC(), msg),
	}
}

// Errorf : Error :: fmt.Sprintf : fmt.Sprint
func Errorf(code StatusCode, format string, a ...interface{}) error {
	return &codeErr{
		err:  fmt.Errorf(format, a...),
		code: code,
		grpc: status.Newf(code.toGRPC(), format, a...),
	}
}

// codeErr implements error that can work both in grpc and drpc.
type codeErr struct {
	err  error
	code StatusCode
	grpc *status.Status
}

func (c *codeErr) Error() string              { return c.err.Error() }
func (c *codeErr) Unwrap() error              { return c.err }
func (c *codeErr) Cause() error               { return c.err }
func (c *codeErr) Code() uint64               { return uint64(c.code) }
func (c *codeErr) GRPCStatus() *status.Status { return c.grpc }

// toGRPC returns the grpc version of the status code.
func (s StatusCode) toGRPC() codes.Code {
	switch s {
	case Unknown:
		return codes.Unknown
	case OK:
		return codes.OK
	case Canceled:
		return codes.Canceled
	case InvalidArgument:
		return codes.InvalidArgument
	case DeadlineExceeded:
		return codes.DeadlineExceeded
	case NotFound:
		return codes.NotFound
	case AlreadyExists:
		return codes.AlreadyExists
	case PermissionDenied:
		return codes.PermissionDenied
	case ResourceExhausted:
		return codes.ResourceExhausted
	case FailedPrecondition:
		return codes.FailedPrecondition
	case Aborted:
		return codes.Aborted
	case OutOfRange:
		return codes.OutOfRange
	case Unimplemented:
		return codes.Unimplemented
	case Internal:
		return codes.Internal
	case Unavailable:
		return codes.Unavailable
	case DataLoss:
		return codes.DataLoss
	case Unauthenticated:
		return codes.Unauthenticated
	default:
		return codes.Unknown
	}
}

// statusCodeFromGRPC turns the grpc status code into a StatusCode.
func statusCodeFromGRPC(code codes.Code) StatusCode {
	switch code {
	case codes.Unknown:
		return Unknown
	case codes.OK:
		return OK
	case codes.Canceled:
		return Canceled
	case codes.InvalidArgument:
		return InvalidArgument
	case codes.DeadlineExceeded:
		return DeadlineExceeded
	case codes.NotFound:
		return NotFound
	case codes.AlreadyExists:
		return AlreadyExists
	case codes.PermissionDenied:
		return PermissionDenied
	case codes.ResourceExhausted:
		return ResourceExhausted
	case codes.FailedPrecondition:
		return FailedPrecondition
	case codes.Aborted:
		return Aborted
	case codes.OutOfRange:
		return OutOfRange
	case codes.Unimplemented:
		return Unimplemented
	case codes.Internal:
		return Internal
	case codes.Unavailable:
		return Unavailable
	case codes.DataLoss:
		return DataLoss
	case codes.Unauthenticated:
		return Unauthenticated
	default:
		return Unknown
	}
}
