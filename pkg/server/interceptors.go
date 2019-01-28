// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package server

import (
	"context"
	"io"

	"go.uber.org/zap"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"storj.io/storj/storage"
)

func streamInterceptor(srv interface{}, ss grpc.ServerStream, info *grpc.StreamServerInfo, handler grpc.StreamHandler) (err error) {
	err = handler(srv, ss)
	if err != nil {
		// no zap errors for canceled or wrong file downloads
		if storage.ErrKeyNotFound.Has(err) ||
			status.Code(err) == codes.Canceled ||
			status.Code(err) == codes.Unavailable ||
			err == io.EOF {
			return err
		}
		zap.S().Errorf("%+v", err)
	}
	return err
}

func unaryInterceptor(ctx context.Context, req interface{}, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (resp interface{},
	err error) {
	resp, err = handler(ctx, req)
	if err != nil {
		// no zap errors for wrong file downloads
		if status.Code(err) == codes.NotFound {
			return resp, err
		}
		zap.S().Errorf("%+v", err)
	}
	return resp, err
}

func combineInterceptors(a, b grpc.UnaryServerInterceptor) grpc.UnaryServerInterceptor {
	return func(ctx context.Context, req interface{}, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (interface{}, error) {
		return a(ctx, req, info, func(actx context.Context, areq interface{}) (interface{}, error) {
			return b(actx, areq, info, func(bctx context.Context, breq interface{}) (interface{}, error) {
				return handler(bctx, breq)
			})
		})
	}
}
