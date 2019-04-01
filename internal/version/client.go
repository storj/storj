// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package version

import (
	"context"

	"github.com/zeebo/errs"
	"google.golang.org/grpc"

	"storj.io/storj/pkg/identity"
	"storj.io/storj/pkg/pb"
	"storj.io/storj/pkg/transport"
)

// VersionClient creates a transport client, that internally checks if the software version is still up to date
// and refuses outgoing connections in case it is outdated
type VersionedClient struct {
	transport transport.Client
	version   *Service
}

const (
	errOldVersion = "Outdated Software Version, please update!"
)

// NewVersionedClient returns a transport client which ensures, that the software is up to date
func NewVersionedClient(transport transport.Client, service *Service) *VersionedClient {
	return &VersionedClient{
		transport: transport,
		version:   service,
	}
}

// DialNode returns a grpc connection with tls to a node.
//
// Use this method for communicating with nodes as it is more secure than
// DialAddress. The connection will be established successfully only if the
// target node has the private key for the requested node ID.
func (client *VersionedClient) DialNode(ctx context.Context, node *pb.Node, opts ...grpc.DialOption) (*grpc.ClientConn, error) {
	if !client.version.IsUpToDate() {
		return nil, errs.New(errOldVersion)
	}
	return client.transport.DialNode(ctx, node, opts...)
}

// DialAddress returns a grpc connection with tls to an IP address.
//
// Do not use this method unless having a good reason. In most cases DialNode
// should be used for communicating with nodes as it is more secure than
// DialAddress.
func (client *VersionedClient) DialAddress(ctx context.Context, address string, opts ...grpc.DialOption) (*grpc.ClientConn, error) {
	if !client.version.IsUpToDate() {
		return nil, errs.New(errOldVersion)
	}
	return client.transport.DialAddress(ctx, address, opts...)
}

// Identity is a getter for the transport's identity
func (client *VersionedClient) Identity() *identity.FullIdentity {
	return client.transport.Identity()
}

// WithObservers returns a new transport including the listed observers.
func (client *VersionedClient) WithObservers(obs ...transport.Observer) transport.Client {
	return &VersionedClient{client.transport.WithObservers(obs...), client.version}
}
