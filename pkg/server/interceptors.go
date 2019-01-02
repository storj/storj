// Copyright (C) 2018 Storj Labs, Inc.
// See LICENSE for copying information.

package server

import (
	"context"
	"io"

	"go.uber.org/zap"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"storj.io/storj/pkg/grpcutils"
	"storj.io/storj/storage"
)

func defaultLogger() grpcutils.ServerInterceptor {
	return grpcutils.ServerInterceptor{
		Stream: func(srv interface{}, ss grpc.ServerStream,
			info *grpc.StreamServerInfo, handler grpc.StreamHandler) (err error) {
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
		},
		Unary: func(ctx context.Context, req interface{},
			info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (
			resp interface{}, err error) {
			resp, err = handler(ctx, req)
			if err != nil {
				// no zap errors for wrong file downloads
				if status.Code(err) == codes.NotFound {
					return resp, err
				}
				zap.S().Errorf("%+v", err)
			}
			return resp, err
		},
	}
}
