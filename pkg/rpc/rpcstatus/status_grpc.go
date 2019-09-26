// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

// +build !drpc

package rpcstatus

import (
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// StatusCode is the type of status codes for grpc.
type StatusCode = codes.Code

// These constants are all the rpc error codes.
const (
	OK                 = codes.OK
	Canceled           = codes.Canceled
	Unknown            = codes.Unknown
	InvalidArgument    = codes.InvalidArgument
	DeadlineExceeded   = codes.DeadlineExceeded
	NotFound           = codes.NotFound
	AlreadyExists      = codes.AlreadyExists
	PermissionDenied   = codes.PermissionDenied
	ResourceExhausted  = codes.ResourceExhausted
	FailedPrecondition = codes.FailedPrecondition
	Aborted            = codes.Aborted
	OutOfRange         = codes.OutOfRange
	Unimplemented      = codes.Unimplemented
	Internal           = codes.Internal
	Unavailable        = codes.Unavailable
	DataLoss           = codes.DataLoss
	Unauthenticated    = codes.Unauthenticated
)

// Code returns the status code associated with the error.
func Code(err error) StatusCode {
	return status.Code(err)
}

// Error wraps the message with a status code into an error.
func Error(code StatusCode, msg string) error {
	return status.Error(code, msg)
}

// Errorf : Error :: fmt.Sprintf : fmt.Sprint
func Errorf(code StatusCode, format string, a ...interface{}) error {
	return status.Errorf(code, format, a...)
}
