// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package testpeertls

import (
	"context"
	"crypto"
	"crypto/x509"

	"storj.io/storj/internal/testidentity"
	"storj.io/storj/pkg/identity"
	"storj.io/storj/pkg/storj"
)

// NewCertChain creates a valid peertls certificate chain (and respective keys) of the desired length.
func NewCertChain(length int, versionNumber storj.IDVersionNumber) (keys []crypto.PrivateKey, certs []*x509.Certificate, err error) {
	var ca, parent *identity.FullCertificateAuthority
	ctx := context.Background()

	// NB: `identity.NewCA` does some things that some tests expect to be done
	// i.e.: adding extensions like version and proof-of-work counter.
	// TODO: check every usage and replace those with `testidentity.NewTestIdentity`
	//  where length == 2 and a `FullIdentity` struct is created from it.
	// (or use `testpeertls.IdentityVersions[version.Number].NewIdentity()`)
	//ca, err := testidentity.NewTestCA(ctx, versionNumber)
	//if err != nil {
	//	return nil, nil, err
	//}
	//certs = append([]*x509.Certificate{ca.Cert}, certs...)
	//keys = append([]crypto.PrivateKey{ca.Key}, keys...)
	//parent = ca

	for i := length; i > 0; i-- {
		if parent == nil {
			ca, err = testidentity.NewTestCA(ctx, versionNumber)
		} else {
			ca, err = testidentity.NewTestCAWithParent(ctx, versionNumber, parent)
		}

		if err != nil {
			return nil, nil, err
		}
		certs = append([]*x509.Certificate{ca.Cert}, certs...)
		keys = append([]crypto.PrivateKey{ca.Key}, keys...)
		parent = ca
	}

	if length > 1 {
		ident, err := ca.NewIdentity()
		if err != nil {
			return nil, nil, err
		}

		certs = append([]*x509.Certificate{ident.Leaf}, certs...)
		keys = append([]crypto.PrivateKey{ident.Key}, keys...)
	}
	return keys, certs, nil
}
