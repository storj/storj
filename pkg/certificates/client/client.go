// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package client

import (
	"context"
	"time"

	"google.golang.org/grpc"
	"gopkg.in/spacemonkeygo/monkit.v2"

	"storj.io/storj/pkg/identity"
	"storj.io/storj/pkg/pb"
	"storj.io/storj/pkg/peertls/extensions"
	"storj.io/storj/pkg/peertls/tlsopts"
	"storj.io/storj/pkg/transport"
)

var mon = monkit.Package()

// ClientConfig is a config struct for use with a certificate signing service client
type ClientConfig struct {
	Address string `help:"address of the certificate signing rpc service"`
	TLS     tlsopts.Config
}

// Client implements pb.CertificateClient
type Client struct {
	conn   *grpc.ClientConn
	client pb.CertificatesClient
}

// NewClient creates a new certificate signing grpc client
func NewClient(ctx context.Context, tc transport.Client, address string) (_ *Client, err error) {
	defer mon.Task()(&ctx)(&err)

	conn, err := tc.DialAddress(ctx, address)
	if err != nil {
		return nil, err
	}

	return &Client{
		conn:   conn,
		client: pb.NewCertificatesClient(conn),
	}, nil
}

// NewClientFrom creates a new certificate signing grpc client from an existing
// grpc cert signing client
func NewClientFrom(client pb.CertificatesClient) (*Client, error) {
	return &Client{
		client: client,
	}, nil
}

// Sign submits a certificate signing request given the config
func (c ClientConfig) Sign(ctx context.Context, ident *identity.FullIdentity, authToken string, revDB extensions.RevocationDB) (_ [][]byte, err error) {
	defer mon.Task()(&ctx)(&err)

	tlsOpts, err := tlsopts.NewOptions(ident, c.TLS, revDB)
	if err != nil {
		return nil, err
	}
	client, err := NewClient(ctx, transport.NewClient(tlsOpts), c.Address)
	if err != nil {
		return nil, err
	}

	return client.Sign(ctx, authToken)
}

// Sign claims an authorization using the token string and returns a signed
// copy of the client's CA certificate
func (c *Client) Sign(ctx context.Context, tokenStr string) (_ [][]byte, err error) {
	defer mon.Task()(&ctx)(&err)

	res, err := c.client.Sign(ctx, &pb.SigningRequest{
		AuthToken: tokenStr,
		Timestamp: time.Now().Unix(),
	})
	if err != nil {
		return nil, err
	}

	return res.Chain, nil
}

// Close closes the client
func (c *Client) Close() error {
	if c.conn != nil {
		return c.conn.Close()
	}
	return nil
}
