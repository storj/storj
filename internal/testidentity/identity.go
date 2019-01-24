// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package testidentity

import (
	"context"

	"storj.io/storj/pkg/provider"
)

// NewTestIdentity is a helper function to generate new node identities with
// correct difficulty and concurrency
func NewTestIdentity(ctx context.Context) (*provider.FullIdentity, error) {
	ca, err := provider.NewCA(ctx, provider.NewCAOptions{
		Difficulty:  4,
		Concurrency: 1,
	})
	if err != nil {
		return nil, err
	}
	identity, err := ca.NewIdentity()
	if err != nil {
		return nil, err
	}
	return identity, err
}

// NewTestCA returns a ca with a default difficulty and concurrency for use in tests
func NewTestCA(ctx context.Context) (*provider.FullCertificateAuthority, error) {
	return provider.NewCA(ctx, provider.NewCAOptions{
		Difficulty:  4,
		Concurrency: 1,
	})
}
