// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package console

import (
	"context"

	"github.com/zeebo/errs"

	"storj.io/storj/satellite/console/consoleauth"
)

// TODO: change to JWT or Macaroon based auth

// key is a context value key type.
type key int

// authKey is context key for Authorization.
const authKey key = 0

// requestKey is context key for Requests.
const requestKey key = 1

// ErrUnauthorized is error class for authorization related errors.
var ErrUnauthorized = errs.Class("unauthorized")

// Authorization contains auth info of authorized User.
type Authorization struct {
	User   User
	Claims consoleauth.Claims
}

// WithAuth creates new context with Authorization.
func WithAuth(ctx context.Context, auth Authorization) context.Context {
	return context.WithValue(ctx, authKey, auth)
}

// WithAuthFailure creates new context with authorization failure.
func WithAuthFailure(ctx context.Context, err error) context.Context {
	return context.WithValue(ctx, authKey, err)
}

// GetAuth gets Authorization from context.
func GetAuth(ctx context.Context) (Authorization, error) {
	value := ctx.Value(authKey)

	if auth, ok := value.(Authorization); ok {
		return auth, nil
	}

	if err, ok := value.(error); ok {
		return Authorization{}, ErrUnauthorized.Wrap(err)
	}

	return Authorization{}, ErrUnauthorized.New(unauthorizedErrMsg)
}
