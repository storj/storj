// Copyright (C) 2024 Storj Labs, Inc.
// See LICENSE for copying information.

package kms

import (
	"context"

	secretmanager "cloud.google.com/go/secretmanager/apiv1"
	"cloud.google.com/go/secretmanager/apiv1/secretmanagerpb"
)

// client gets the secret data from the secret manager.
type client interface {
	AccessSecretVersion(ctx context.Context, req *secretmanagerpb.AccessSecretVersionRequest) (*secretmanagerpb.AccessSecretVersionResponse, error)
	Close() error
}

type gsmClient struct {
	client *secretmanager.Client
}

func (c *gsmClient) AccessSecretVersion(ctx context.Context, req *secretmanagerpb.AccessSecretVersionRequest) (_ *secretmanagerpb.AccessSecretVersionResponse, err error) {
	defer mon.Task()(&ctx)(&err)

	return c.client.AccessSecretVersion(ctx, req)
}

func (c *gsmClient) Close() error {
	return c.client.Close()
}
