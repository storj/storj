// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package server

import (
	"context"
	"io"

	grpc_middleware "github.com/grpc-ecosystem/go-grpc-middleware"
	grpc_zap "github.com/grpc-ecosystem/go-grpc-middleware/logging/zap"
	grpc_ctxtags "github.com/grpc-ecosystem/go-grpc-middleware/tags"
	"go.uber.org/zap"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"storj.io/storj/storage"
)

func logOnErrorStreamInterceptor(srv interface{}, ss grpc.ServerStream, info *grpc.StreamServerInfo, handler grpc.StreamHandler) (err error) {
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

func logOnErrorUnaryInterceptor(ctx context.Context, req interface{}, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (resp interface{},
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

// the always-yes logging decider function (because grpc_zap requires a decider function)
func yesLogIt(ctx context.Context, fullMethodName string, servingObject interface{}) bool {
	return true
}

// WithUnaryLoggingInterceptor combines interceptors for logging all GRPC unary calls with the
// given other unary interceptors.
func WithUnaryLoggingInterceptor(log *zap.Logger, otherInterceptors ...grpc.UnaryServerInterceptor) grpc.UnaryServerInterceptor {
	interceptors := append([]grpc.UnaryServerInterceptor{
		grpc_ctxtags.UnaryServerInterceptor(),
		grpc_zap.PayloadUnaryServerInterceptor(log, yesLogIt),
	}, otherInterceptors...)
	return grpc_middleware.ChainUnaryServer(interceptors...)
}

// WithStreamLoggingInterceptor combines interceptors for logging all GRPC streaming calls with the
// given other stream interceptors.
func WithStreamLoggingInterceptor(log *zap.Logger, otherInterceptors ...grpc.StreamServerInterceptor) grpc.StreamServerInterceptor {
	interceptors := append([]grpc.StreamServerInterceptor{
		grpc_ctxtags.StreamServerInterceptor(),
		grpc_zap.PayloadStreamServerInterceptor(log, yesLogIt),
	}, otherInterceptors...)
	return grpc_middleware.ChainStreamServer(interceptors...)
}
