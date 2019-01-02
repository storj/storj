// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package grpcutils

import (
	grpcmiddleware "github.com/grpc-ecosystem/go-grpc-middleware"
	"google.golang.org/grpc"
)

// ServerInterceptor is a pair of gRPC interceptors, for stream and unary
// methods. It's important to remember both, hence this type.
type ServerInterceptor struct {
	Stream grpc.StreamServerInterceptor
	Unary  grpc.UnaryServerInterceptor
}

// ServerInterceptors turns a list of interceptors into a slice of
// grpc.ServerOptions. You can only add interceptors to a slice of
// ServerOptions once. ServerInterceptors are executed left-to right, so
// ServerInterceptors(one, two, three) will execute one, then two, then three.
func ServerInterceptors(interceptors ...ServerInterceptor) []grpc.ServerOption {
	streamInterceptors := make(
		[]grpc.StreamServerInterceptor, 0, len(interceptors))
	unaryInterceptors := make(
		[]grpc.UnaryServerInterceptor, 0, len(interceptors))
	for _, i := range interceptors {
		streamInterceptors = append(streamInterceptors, i.Stream)
		unaryInterceptors = append(unaryInterceptors, i.Unary)
	}
	return []grpc.ServerOption{
		grpcmiddleware.WithStreamServerChain(streamInterceptors...),
		grpcmiddleware.WithUnaryServerChain(unaryInterceptors...)}
}

// ClientInterceptor is a pair of gRPC interceptors, for stream and unary
// methods. It's important to remember both, hence this type.
type ClientInterceptor struct {
	Stream grpc.StreamClientInterceptor
	Unary  grpc.UnaryClientInterceptor
}

// ClientInterceptors turns a list of interceptors into a slice of
// grpc.DialOptions. You can only add interceptors to a slice of
// DialOptions once. ClientInterceptors are executed left-to right, so
// ClientInterceptors(one, two, three) will execute one, then two, then three.
func ClientInterceptors(interceptors ...ClientInterceptor) []grpc.DialOption {
	streamInterceptors := make(
		[]grpc.StreamClientInterceptor, 0, len(interceptors))
	unaryInterceptors := make(
		[]grpc.UnaryClientInterceptor, 0, len(interceptors))
	for _, i := range interceptors {
		streamInterceptors = append(streamInterceptors, i.Stream)
		unaryInterceptors = append(unaryInterceptors, i.Unary)
	}
	return []grpc.DialOption{
		grpc.WithStreamInterceptor(
			grpcmiddleware.ChainStreamClient(streamInterceptors...)),
		grpc.WithUnaryInterceptor(
			grpcmiddleware.ChainUnaryClient(unaryInterceptors...)),
	}
}
