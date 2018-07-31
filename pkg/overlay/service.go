// Copyright (C) 2018 Storj Labs, Inc.
// See LICENSE for copying information.

package overlay

import (
	"go.uber.org/zap"
	"google.golang.org/grpc"
	"gopkg.in/spacemonkeygo/monkit.v2"

	"storj.io/storj/pkg/kademlia"
	"storj.io/storj/pkg/peertls"
	proto "storj.io/storj/protos/overlay"
)

// NewServer creates a new Overlay Service Server
func NewServer(k *kademlia.Kademlia, cache *Cache, l *zap.Logger, m *monkit.Registry) *grpc.Server {
	grpcServer := grpc.NewServer()
	proto.RegisterOverlayServer(grpcServer, &Server{
		dht:     k,
		cache:   cache,
		logger:  l,
		metrics: m,
	})

	return grpcServer
}

// NewClient connects to grpc server at the provided address with the provided options
// returns a new instance of an overlay Client
func NewClient(serverAddr string, opts ...grpc.DialOption) (proto.OverlayClient, error) {
	conn, err := grpc.Dial(serverAddr, opts...)
	if err != nil {
		return nil, err
	}

	return proto.NewOverlayClient(conn), nil
}

// NewTLSServer returns a newly initialized gRPC overlay server, configured with TLS
func NewTLSServer(k *kademlia.Kademlia, cache *Cache, l *zap.Logger, m *monkit.Registry, fopts peertls.TLSFileOptions) (_ *grpc.Server, _ error) {
	t, err := peertls.NewTLSFileOptions(
		fopts.RootCertRelPath,
		fopts.RootKeyRelPath,
		fopts.Create,
		fopts.Overwrite,
	)
	if err != nil {
		return nil, err
	}

	grpcServer := grpc.NewServer(t.ServerOption())
	proto.RegisterOverlayServer(grpcServer, &Server{
		dht:     k,
		cache:   cache,
		logger:  l,
		metrics: m,
	})

	return grpcServer, nil
}

// NewTLSClient connects to grpc server at the provided address with the provided options plus TLS option(s)
// returns a new instance of an overlay Client
func NewTLSClient(serverAddr *string, fopts peertls.TLSFileOptions, opts ...grpc.DialOption) (proto.OverlayClient, error) {
	t, err := peertls.NewTLSFileOptions(
		fopts.RootCertRelPath,
		fopts.RootCertRelPath,
		fopts.Create,
		fopts.Overwrite,
	)
	if err != nil {
		return nil, err
	}

	opts = append(opts, t.DialOption())
	conn, err := grpc.Dial(*serverAddr, opts...)
	if err != nil {
		return nil, err
	}

	return proto.NewOverlayClient(conn), nil
}
