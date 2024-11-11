// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package metasearch

// ErrorResponse is a struct for error responses that also implements the error interface.
type ErrorResponse struct {
	StatusCode int    `json:"-"`
	Message    string `json:"error"`
}

func (e *ErrorResponse) Error() string {
	return e.Message
}

var (
	// ErrBadRequest is returned when the request is malformed.
	ErrBadRequest = &ErrorResponse{StatusCode: 400, Message: "bad request"}

	// ErrNotFound is returned when the requested resource is not found.
	ErrNotFound = &ErrorResponse{StatusCode: 404, Message: "not found"}

	// ErrAuthorizationFailed is returned when the request is not authorized.
	ErrAuthorizationFailed = &ErrorResponse{StatusCode: 401, Message: "authorization failed"}

	// ErrInternalError is returned when an internal error occurs.
	ErrInternalError = &ErrorResponse{StatusCode: 500, Message: "internal error"}
)
