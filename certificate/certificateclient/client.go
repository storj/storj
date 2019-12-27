// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package certificateclient

import (
	"context"
	"time"

	"github.com/zeebo/errs"
	"gopkg.in/spacemonkeygo/monkit.v2"

	"storj.io/common/identity"
	"storj.io/common/pb"
	"storj.io/common/peertls/tlsopts"
	"storj.io/common/rpc"
)

var mon = monkit.Package()

// Config is a config struct for use with a certificate signing service client.
type Config struct {
	Address string `help:"address of the certificate signing rpc service"`
	TLS     tlsopts.Config
}

// Client implements pb.DRPCCertificatesClient
type Client struct {
	conn   *rpc.Conn
	client pb.DRPCCertificatesClient
}

// New creates a new certificate signing rpc client.
func New(ctx context.Context, dialer rpc.Dialer, address string) (_ *Client, err error) {
	defer mon.Task()(&ctx, address)(&err)

	conn, err := dialer.DialAddressInsecure(ctx, address)
	if err != nil {
		return nil, err
	}

	return &Client{
		conn:   conn,
		client: pb.NewDRPCCertificatesClient(conn.Raw()),
	}, nil
}

// NewClientFrom creates a new certificate signing gRPC client from an existing
// grpc cert signing client.
func NewClientFrom(client pb.DRPCCertificatesClient) *Client {
	return &Client{
		client: client,
	}
}

// Sign submits a certificate signing request given the config.
func (config Config) Sign(ctx context.Context, ident *identity.FullIdentity, authToken string) (_ [][]byte, err error) {
	defer mon.Task()(&ctx)(&err)

	tlsOptions, err := tlsopts.NewOptions(ident, config.TLS, nil)
	if err != nil {
		return nil, err
	}
	client, err := New(ctx, rpc.NewDefaultDialer(tlsOptions), config.Address)
	if err != nil {
		return nil, err
	}
	defer func() { err = errs.Combine(err, client.Close()) }()

	return client.Sign(ctx, authToken)
}

// Sign claims an authorization using the token string and returns a signed
// copy of the client's CA certificate.
func (client *Client) Sign(ctx context.Context, tokenStr string) (_ [][]byte, err error) {
	defer mon.Task()(&ctx)(&err)

	res, err := client.client.Sign(ctx, &pb.SigningRequest{
		AuthToken: tokenStr,
		Timestamp: time.Now().Unix(),
	})
	if err != nil {
		return nil, err
	}

	return res.Chain, nil
}

// Close closes the client.
func (client *Client) Close() error {
	if client.conn != nil {
		return client.conn.Close()
	}
	return nil
}
