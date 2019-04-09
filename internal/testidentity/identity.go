// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package testidentity

import (
	"context"
	"fmt"
	"testing"

	"github.com/stretchr/testify/require"

	"storj.io/storj/pkg/identity"
	"storj.io/storj/pkg/storj"
)

// IdentityTest is a function that takes a testing struct, an identity version
// and a full identity.
type IdentityTest func(*testing.T, storj.IDVersion, *identity.FullIdentity)

// SignerTest is a function that takes a testing struct, an identity version
// and a full certificate authority.
type SignerTest func(*testing.T, storj.IDVersion, *identity.FullCertificateAuthority)

// NewTestIdentity is a helper function to generate new node identities with
// correct difficulty and concurrency
func NewTestIdentity(ctx context.Context) (*identity.FullIdentity, error) {
	ca, err := NewTestCA(ctx)
	if err != nil {
		return nil, err
	}
	return ca.NewIdentity()
}

// NewTestCA returns a ca with a default difficulty and concurrency for use in tests
func NewTestCA(ctx context.Context) (*identity.FullCertificateAuthority, error) {
	return identity.NewCA(ctx, identity.NewCAOptions{
		Difficulty:  8,
		Concurrency: 4,
	})
}

// IdentityVersionsTest runs the `IdentityTest` for each identity
// version, with an unsigned identity.
func IdentityVersionsTest(t *testing.T, test IdentityTest) {
	for versionNumber, version := range storj.IDVersions {
		t.Run(fmt.Sprintf("identity version %d", versionNumber), func(t *testing.T) {
			ident, err := IdentityVersions[versionNumber].NewIdentity()
			require.NoError(t, err)

			test(t, version, ident)
		})
	}
}

// SignedIdentityVersionsTest runs the `IdentityTest` for each identity
// version, with an signed identity.
func SignedIdentityVersionsTest(t *testing.T, test IdentityTest) {
	for versionNumber, version := range storj.IDVersions {
		t.Run(fmt.Sprintf("identity version %d", versionNumber), func(t *testing.T) {
			ident, err := SignedIdentityVersions[versionNumber].NewIdentity()
			require.NoError(t, err)

			test(t, version, ident)
		})
	}
}

// CompleteIdentityVersionsTest runs the `IdentityTest` for each identity
// version, with both signed dn unsigned identities.
func CompleteIdentityVersionsTest(t *testing.T, test IdentityTest) {
	t.Run("unsigned identity", func(t *testing.T) {
		IdentityVersionsTest(t, test)
	})

	t.Run("signed identity", func(t *testing.T) {
		SignedIdentityVersionsTest(t, test)
	})
}

// SignerVersionsTest runs the `SignerTest` for each identity version, with the
// respective signer used to sign pregenerated, signed  identities.
func SignerVersionsTest(t *testing.T, test SignerTest) {
	for versionNumber, version := range storj.IDVersions {
		t.Run(fmt.Sprintf("identity version %d", versionNumber), func(t *testing.T) {
			ca := SignerVersions[versionNumber]

			test(t, version, ca)
		})
	}
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

// NewTestManageableFullIdentity returns a new manageable full identity for use in tests.
func NewTestManageableFullIdentity(ctx context.Context) (*identity.ManageableFullIdentity, error) {
	ca, err := NewTestCA(ctx)
	if err != nil {
		return nil, err
	}

	ident, err := ca.NewIdentity()
	if err != nil {
		return nil, err
	}
	return identity.NewManageableFullIdentity(ident, ca), nil
}
