// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package testidentity

import (
	"context"

	"storj.io/storj/pkg/identity"
)

// NewTestIdentity is a helper function to generate new node identities with
// correct difficulty and concurrency
func NewTestIdentity(ctx context.Context) (*identity.FullIdentity, error) {
	ca, err := identity.NewCA(ctx, identity.NewCAOptions{
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
func NewTestCA(ctx context.Context) (*identity.FullCertificateAuthority, error) {
	return identity.NewCA(ctx, identity.NewCAOptions{
		Difficulty:  4,
		Concurrency: 1,
	})
}

// NewTestManageablePeerIdentity returns a new manageable peer identity for use in tests.
func NewTestManageablePeerIdentity(ctx context.Context) (*identity.ManageablePeerIdentity, error) {
	ca, err := NewTestCA(ctx)
	if err != nil {
		return nil, err
	}

	ident, err := ca.NewIdentity()
	if err != nil {
		return nil, err
	}
	return identity.NewManageablePeerIdentity(ident.PeerIdentity(), ca), nil
}
